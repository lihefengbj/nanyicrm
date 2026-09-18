package quota

import (
	"strings"
	"testing"
	"time"

	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

func int64ptr(v int64) *int64 { return &v }

func TestEffectiveLimits(t *testing.T) {
	service := &Service{cfg: Config{DailyCalls: 100, DailyTokens: 2000, Concurrency: 3}}

	t.Run("nil tenant uses platform defaults", func(t *testing.T) {
		limits := service.EffectiveLimits(nil)
		if limits.DailyCalls != 100 || limits.DailyTokens != 2000 || limits.Concurrency != 3 {
			t.Fatalf("unexpected limits: %+v", limits)
		}
	})

	t.Run("full override", func(t *testing.T) {
		tenant := &model.SysTenant{
			AIDailyCalls:  int64ptr(50),
			AIDailyTokens: int64ptr(500),
			AIConcurrency: int64ptr(1),
		}
		limits := service.EffectiveLimits(tenant)
		if limits.DailyCalls != 50 || limits.DailyTokens != 500 || limits.Concurrency != 1 {
			t.Fatalf("unexpected limits: %+v", limits)
		}
	})

	t.Run("partial override keeps defaults for nil fields", func(t *testing.T) {
		tenant := &model.SysTenant{AIDailyCalls: int64ptr(0)}
		limits := service.EffectiveLimits(tenant)
		if limits.DailyCalls != 0 || limits.DailyTokens != 2000 || limits.Concurrency != 3 {
			t.Fatalf("unexpected limits: %+v", limits)
		}
	})
}

func TestExceededErrorMessage(t *testing.T) {
	cases := []struct {
		kind  string
		limit int64
		want  string
	}{
		{"calls", 10, "今日AI分析次数已达上限（10 次）"},
		{"tokens", 5000, "今日AI分析Token用量已达上限（5000）"},
		{"concurrency", 2, "当前AI分析并发已达上限（2）"},
		{"unknown", 1, "AI分析额度已达上限"},
	}
	for _, tc := range cases {
		message := (&ExceededError{Kind: tc.kind, Limit: tc.limit}).Error()
		if !strings.Contains(message, tc.want) {
			t.Fatalf("kind=%s message %q does not contain %q", tc.kind, message, tc.want)
		}
	}
}

func TestDailyKeyAndTTL(t *testing.T) {
	now := time.Date(2026, 9, 18, 15, 30, 0, 0, time.Local)
	key := dailyKey(quotaCallsKeyPrefix, 7, now)
	if key != "nanyicrm:crm:customer-intent:quota:calls:20260918:7" {
		t.Fatalf("unexpected key %q", key)
	}
	ttl := ttlUntilTomorrow(now)
	want := 9*time.Hour + 30*time.Minute
	if ttl < want || ttl > want+time.Hour {
		t.Fatalf("ttl %v, want %v", ttl, want)
	}
}
