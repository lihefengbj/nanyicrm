package observability

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
)

// Metrics is a small dependency-free Prometheus exposition registry. It is
// intentionally process-local; Prometheus remains the source of aggregation.
type Metrics struct {
	httpRequests   atomic.Uint64
	httpErrors     atomic.Uint64
	loginSuccesses atomic.Uint64
	loginFailures  atomic.Uint64
	aiRequests     atomic.Uint64
	aiSuccesses    atomic.Uint64
	aiFailures     atomic.Uint64
	alertsSent     atomic.Uint64
	archiveRows    atomic.Uint64
	cleanupRows    atomic.Uint64
}

func NewMetrics() *Metrics { return &Metrics{} }

func (m *Metrics) IncHTTPRequest()  { m.httpRequests.Add(1) }
func (m *Metrics) IncHTTPError()    { m.httpErrors.Add(1) }
func (m *Metrics) IncLoginSuccess() { m.loginSuccesses.Add(1) }
func (m *Metrics) IncLoginFailure() { m.loginFailures.Add(1) }
func (m *Metrics) IncAIRequest()    { m.aiRequests.Add(1) }
func (m *Metrics) IncAISuccess()    { m.aiSuccesses.Add(1) }
func (m *Metrics) IncAIFailure()    { m.aiFailures.Add(1) }
func (m *Metrics) IncAlertSent()    { m.alertsSent.Add(1) }
func (m *Metrics) AddArchiveRows(n int64) {
	if n > 0 {
		m.archiveRows.Add(uint64(n))
	}
}
func (m *Metrics) AddCleanupRows(n int64) {
	if n > 0 {
		m.cleanupRows.Add(uint64(n))
	}
}

func (m *Metrics) Render() string {
	var b strings.Builder
	write := func(name, help string, value uint64) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", name, help, name, name, value)
	}
	write("nanyicrm_http_requests_total", "Total HTTP requests.", m.httpRequests.Load())
	write("nanyicrm_http_errors_total", "HTTP requests returning an error status.", m.httpErrors.Load())
	write("nanyicrm_login_success_total", "Successful login attempts.", m.loginSuccesses.Load())
	write("nanyicrm_login_failure_total", "Failed login attempts.", m.loginFailures.Load())
	write("nanyicrm_ai_requests_total", "AI analysis requests.", m.aiRequests.Load())
	write("nanyicrm_ai_success_total", "Successful AI analyses.", m.aiSuccesses.Load())
	write("nanyicrm_ai_failure_total", "Failed AI analyses.", m.aiFailures.Load())
	write("nanyicrm_security_alerts_sent_total", "Security alerts sent to the configured webhook.", m.alertsSent.Load())
	write("nanyicrm_ai_archive_rows_total", "AI rows archived by retention cleanup.", m.archiveRows.Load())
	write("nanyicrm_ai_cleanup_rows_total", "AI rows removed by retention cleanup.", m.cleanupRows.Load())
	return b.String()
}

// AlertManager emits de-duplicated security alerts. Redis provides the
// rolling counter and alert cooldown, so multiple backend replicas share one
// threshold.
type AlertManager struct {
	cfg     config.AlertConfig
	rdb     *redis.Client
	metrics *Metrics
	client  *http.Client
}

func NewAlertManager(cfg config.AlertConfig, rdb *redis.Client, metrics *Metrics) *AlertManager {
	return &AlertManager{
		cfg: cfg, rdb: rdb, metrics: metrics,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (a *AlertManager) LoginFailure(ctx context.Context, username, ip, reason string) {
	if a == nil || a.metrics == nil {
		return
	}
	a.metrics.IncLoginFailure()
	if !a.cfg.Enabled || strings.TrimSpace(a.cfg.WebhookURL) == "" || a.rdb == nil {
		return
	}
	sum := sha256.Sum256([]byte(ip + "\x00" + username))
	base := "security:login-failure:" + hex.EncodeToString(sum[:])
	count, err := a.rdb.Incr(ctx, base).Result()
	if err != nil {
		return
	}
	_ = a.rdb.Expire(ctx, base, a.cfg.Window).Err()
	if count < int64(a.cfg.LoginFailureThreshold) {
		return
	}
	alertKey := base + ":alert"
	claimed, err := a.rdb.SetNX(ctx, alertKey, "1", a.cfg.Window).Result()
	if err != nil || !claimed {
		return
	}
	payload := map[string]interface{}{
		"event":    "abnormal_login",
		"username": username,
		"ip":       ip,
		"reason":   reason,
		"count":    count,
		"window":   a.cfg.Window.String(),
		"occurred": time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)
	go a.send(body)
}

func (a *AlertManager) send(body []byte) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.client.Do(req)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			a.metrics.alertsSent.Add(1)
		}
	}
}

func MetricsTokenMatches(expected, provided string) bool {
	if expected == "" {
		return true
	}
	if len(expected) != len(provided) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}
