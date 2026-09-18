// Package quota implements tenant-level AI intent usage accounting and
// admission control. Daily call and token counters live in Redis with keys
// scoped by UTC+8 date and tenant; the in-flight concurrency count is derived
// from running analysis history rows so it survives process restarts.
package quota

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

// Config mirrors config.QuotaConfig; 0 means "no limit".
type Config struct {
	DailyCalls  int64
	DailyTokens int64
	Concurrency int64
}

// Limits holds the effective limits for one tenant (override or platform default).
type Limits struct {
	DailyCalls  int64
	DailyTokens int64
	Concurrency int64
}

// ExceededError reports which quota gate rejected an operation.
type ExceededError struct {
	Kind  string // calls | tokens | concurrency
	Limit int64
}

func (e *ExceededError) Error() string {
	switch e.Kind {
	case "calls":
		return fmt.Sprintf("今日AI分析次数已达上限（%d 次），请明天再试或联系管理员调整额度", e.Limit)
	case "tokens":
		return fmt.Sprintf("今日AI分析Token用量已达上限（%d），请明天再试或联系管理员调整额度", e.Limit)
	case "concurrency":
		return fmt.Sprintf("当前AI分析并发已达上限（%d），请稍后再试或联系管理员调整额度", e.Limit)
	default:
		return "AI分析额度已达上限"
	}
}

const (
	quotaCallsKeyPrefix  = "nanyicrm:crm:customer-intent:quota:calls:"
	quotaTokensKeyPrefix = "nanyicrm:crm:customer-intent:quota:tokens:"

	// runningWindow bounds the DB query for in-flight analyses. A task that
	// crashes leaves its history row in "running"; rows older than the window
	// are stale and ignored. The analysis HTTP timeout is 45s.
	runningWindow = 45 * time.Minute
)

// Service enforces AI intent quotas with Redis counters and a database-backed
// concurrency view.
type Service struct {
	db  *gorm.DB
	rdb *redis.Client
	cfg Config
}

func New(db *gorm.DB, rdb *redis.Client, cfg Config) *Service {
	return &Service{db: db, rdb: rdb, cfg: cfg}
}

// EffectiveLimits resolves a tenant's overrides against the platform default.
func (s *Service) EffectiveLimits(tenant *model.SysTenant) Limits {
	limits := Limits{DailyCalls: s.cfg.DailyCalls, DailyTokens: s.cfg.DailyTokens, Concurrency: s.cfg.Concurrency}
	if tenant == nil {
		return limits
	}
	if tenant.AIDailyCalls != nil {
		limits.DailyCalls = *tenant.AIDailyCalls
	}
	if tenant.AIDailyTokens != nil {
		limits.DailyTokens = *tenant.AIDailyTokens
	}
	if tenant.AIConcurrency != nil {
		limits.Concurrency = *tenant.AIConcurrency
	}
	return limits
}

// LimitsOf loads a tenant's effective limits. A missing tenant falls back to
// the platform default so platform-level callers keep working.
func (s *Service) LimitsOf(ctx context.Context, tenantID uint64) (Limits, error) {
	var tenant model.SysTenant
	err := s.db.WithContext(ctx).First(&tenant, tenantID).Error
	if err == gorm.ErrRecordNotFound {
		return s.EffectiveLimits(nil), nil
	}
	if err != nil {
		return Limits{}, err
	}
	return s.EffectiveLimits(&tenant), nil
}

// CheckSubmission rejects creating new analysis tasks when today's usage
// already meets a limit. extraCalls estimates how many analyses the incoming
// submission would add (1 for a single customer, N for a batch) and is only
// applied to the daily call limit; concurrency checks the current in-flight
// count only.
func (s *Service) CheckSubmission(ctx context.Context, tenantID uint64, extraCalls int64) error {
	limits, err := s.LimitsOf(ctx, tenantID)
	if err != nil {
		return err
	}
	calls, tokens, err := s.Usage(ctx, tenantID)
	if err != nil {
		return err
	}
	if limits.DailyCalls > 0 && extraCalls > 0 && calls+extraCalls > limits.DailyCalls {
		return &ExceededError{Kind: "calls", Limit: limits.DailyCalls}
	}
	if limits.DailyTokens > 0 && tokens >= limits.DailyTokens {
		return &ExceededError{Kind: "tokens", Limit: limits.DailyTokens}
	}
	if limits.Concurrency > 0 {
		running, err := s.RunningCount(ctx, tenantID)
		if err != nil {
			return err
		}
		if running >= limits.Concurrency {
			return &ExceededError{Kind: "concurrency", Limit: limits.Concurrency}
		}
	}
	return nil
}

// ReserveCall atomically counts one started analysis against the tenant's
// daily call limit. It fails with an ExceededError when the limit is already
// reached, so racing submissions cannot exceed the configured ceiling.
func (s *Service) ReserveCall(ctx context.Context, tenantID uint64) error {
	limits, err := s.LimitsOf(ctx, tenantID)
	if err != nil {
		return err
	}
	now := time.Now()
	key := dailyKey(quotaCallsKeyPrefix, tenantID, now)
	result, err := reserveCallScript.Run(ctx, s.rdb, []string{key},
		limits.DailyCalls, int64(ttlUntilTomorrow(now)/time.Second)).Int64()
	if err != nil {
		return err
	}
	if result < 0 {
		return &ExceededError{Kind: "calls", Limit: limits.DailyCalls}
	}
	return nil
}

// AddTokens accumulates provider token usage for today. Accounting is
// best-effort: a Redis failure is logged and must not fail the analysis that
// already succeeded.
func (s *Service) AddTokens(ctx context.Context, tenantID uint64, tokens int64) {
	if tokens <= 0 {
		return
	}
	now := time.Now()
	key := dailyKey(quotaTokensKeyPrefix, tenantID, now)
	if err := s.rdb.IncrBy(ctx, key, tokens).Err(); err != nil {
		log.Printf("quota token accounting tenant=%d: %v", tenantID, err)
		return
	}
	_ = s.rdb.Expire(ctx, key, ttlUntilTomorrow(now)).Err()
}

// Usage returns today's used call and token counts.
func (s *Service) Usage(ctx context.Context, tenantID uint64) (int64, int64, error) {
	now := time.Now()
	pipe := s.rdb.Pipeline()
	callsCmd := pipe.Get(ctx, dailyKey(quotaCallsKeyPrefix, tenantID, now))
	tokensCmd := pipe.Get(ctx, dailyKey(quotaTokensKeyPrefix, tenantID, now))
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return 0, 0, err
	}
	calls, _ := callsCmd.Int64()
	tokens, _ := tokensCmd.Int64()
	return calls, tokens, nil
}

// RunningCount returns the number of analyses currently in flight for the
// tenant, ignoring rows left running by crashed workers.
func (s *Service) RunningCount(ctx context.Context, tenantID uint64) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.CrmCustomerIntentAnalysis{}).
		Where("tenant_id = ? AND status = ? AND created_at > ?", tenantID, "running", time.Now().Add(-runningWindow)).
		Count(&count).Error
	return count, err
}

// ResetToday clears a tenant's daily call and token counters so it can keep
// submitting analyses after a misconfiguration or an agreed quota raise.
func (s *Service) ResetToday(ctx context.Context, tenantID uint64) error {
	now := time.Now()
	return s.rdb.Del(ctx,
		dailyKey(quotaCallsKeyPrefix, tenantID, now),
		dailyKey(quotaTokensKeyPrefix, tenantID, now)).Err()
}

func dailyKey(prefix string, tenantID uint64, now time.Time) string {
	return fmt.Sprintf("%s%s:%d", prefix, now.Format("20060102"), tenantID)
}

// ttlUntilTomorrow keeps a daily counter alive until the end of the local day
// plus a one-hour safety buffer. Keys also embed the date, so a stale key is
// never read by accident.
func ttlUntilTomorrow(now time.Time) time.Duration {
	tomorrow := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	return tomorrow.Sub(now) + time.Hour
}

// reserveCallScript atomically enforces the daily call ceiling: limit <= 0
// means unlimited (the counter still records usage). Returns -1 when the
// limit is already reached and -2 when the increment crossed it, in which
// case the increment is rolled back.
var reserveCallScript = redis.NewScript(`
local current = tonumber(redis.call('GET', KEYS[1]) or '0')
local limit = tonumber(ARGV[1])
if limit > 0 and current >= limit then
  return -1
end
local value = redis.call('INCR', KEYS[1])
if limit > 0 and value > limit then
  redis.call('DECR', KEYS[1])
  return -2
end
redis.call('EXPIRE', KEYS[1], tonumber(ARGV[2]))
return value
`)
