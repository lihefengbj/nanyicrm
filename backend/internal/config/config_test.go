package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadExpandsEnvPlaceholders(t *testing.T) {
	t.Setenv("TEST_MYSQL_PWD", "s3cret")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "server:\n  port: \"19090\"\nmysql:\n  - name: default\n    dsn: \"root:${TEST_MYSQL_PWD}@tcp(localhost:3306)/nanyicrm\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)

	cfg := Load()
	if cfg.Server.Port != "19090" {
		t.Fatalf("port = %q, want 19090", cfg.Server.Port)
	}
	dsn := cfg.DefaultMySQL().DSN
	if want := "root:s3cret@tcp(localhost:3306)/nanyicrm"; dsn[:len(want)] != want {
		t.Fatalf("dsn = %q, want prefix %q", dsn, want)
	}
}

func TestLoadFallsBackToDefaults(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	cfg := Load()
	if cfg.Server.Port != "8080" {
		t.Fatalf("port = %q, want default 8080", cfg.Server.Port)
	}
	if len(cfg.MySQL) != 1 {
		t.Fatalf("mysql entries = %d, want 1 default", len(cfg.MySQL))
	}
}

func TestLoadKeepsZeroLogRetention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("log:\n  retain_days: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	if got := Load().Log.RetainDays; got != 0 {
		t.Fatalf("retain days = %d, want 0", got)
	}
}

func TestLoadIntentRetentionDays(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("llm:\n  retention_days: 45\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	if got := Load().LLM.RetentionDays; got != 45 {
		t.Fatalf("intent retention days = %d, want 45", got)
	}
}

func TestLoadIntentArchiveDays(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("llm:\n  archive_days: 365\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	if got := Load().LLM.ArchiveDays; got != 365 {
		t.Fatalf("intent archive days = %d, want 365", got)
	}
}

func TestLoadIntentPricing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("llm:\n  pricing:\n    currency: CNY\n    input_per_1m_tokens: 2.5\n    output_per_1m_tokens: 8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	cfg := Load()
	if cfg.LLM.Pricing.Currency != "CNY" ||
		cfg.LLM.Pricing.InputPerMillionTokens != 2.5 ||
		cfg.LLM.Pricing.OutputPerMillionTokens != 8 {
		t.Fatalf("unexpected pricing config: %+v", cfg.LLM.Pricing)
	}
}

func TestLoadDeepSeekTieredPricing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`llm:
  pricing:
    currency: CNY
    period: peak
    input_cache_hit_idle_per_1m_tokens: 0.02
    input_cache_hit_peak_per_1m_tokens: 0.04
    input_cache_miss_idle_per_1m_tokens: 1
    input_cache_miss_peak_per_1m_tokens: 2
    output_idle_per_1m_tokens: 4
    output_peak_per_1m_tokens: 8
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	cfg := Load()
	pricing := cfg.LLM.Pricing
	if pricing.Currency != "CNY" || pricing.Period != PricingPeriodPeak ||
		pricing.InputCacheHitPeakPerMillion != 0.04 ||
		pricing.InputCacheMissPeakPerMillion != 2 ||
		pricing.OutputPeakPerMillion != 8 {
		t.Fatalf("unexpected tiered pricing config: %+v", pricing)
	}
	rates := pricing.Rates()
	if rates.InputCacheHitPerMillion != 0.04 ||
		rates.InputCacheMissPerMillion != 2 ||
		rates.OutputPerMillion != 8 {
		t.Fatalf("unexpected peak pricing rates: %+v", rates)
	}
}

func TestLoadPricingModeAndFixedPeriod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`llm:
  pricing:
    mode: auto
    fixed_period: peak
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	pricing := Load().LLM.Pricing
	if pricing.ModeValue() != PricingModeAuto || pricing.FixedPeriodValue() != PricingPeriodPeak {
		t.Fatalf("unexpected pricing mode: mode=%q fixed=%q", pricing.ModeValue(), pricing.FixedPeriodValue())
	}
	if got := pricing.PeriodAt(time.Date(2026, 9, 20, 1, 0, 0, 0, time.UTC)); got != PricingPeriodPeak {
		t.Fatalf("fixed period = %q, want peak", got)
	}
}

func TestLoadPricingPeakPeriods(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`llm:
  pricing:
    mode: auto
    timezone: Asia/Shanghai
    peak_periods:
      - weekdays: [1, 2, 3, 4, 5]
        start: "08:30"
        end: "11:30"
      - weekdays: [1, 3, 5]
        start: "20:00"
        end: "21:00"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	pricing := Load().LLM.Pricing
	if len(pricing.PeakPeriods) != 2 ||
		pricing.PeakPeriods[0].Start != "08:30" ||
		pricing.PeakPeriods[1].Weekdays[2] != 5 {
		t.Fatalf("unexpected peak periods: %+v", pricing.PeakPeriods)
	}
	if got := pricing.PeriodAt(time.Date(2026, 9, 21, 0, 30, 0, 0, time.UTC)); got != PricingPeriodPeak {
		t.Fatalf("08:30 weekday period = %q, want peak", got)
	}
	if got := pricing.PeriodAt(time.Date(2026, 9, 21, 3, 30, 0, 0, time.UTC)); got != PricingPeriodIdle {
		t.Fatalf("11:30 weekday period = %q, want idle", got)
	}
	if got := pricing.PeriodAt(time.Date(2026, 9, 22, 12, 30, 0, 0, time.UTC)); got != PricingPeriodIdle {
		t.Fatalf("20:30 Tuesday period = %q, want idle", got)
	}
	if got := pricing.PeriodAt(time.Date(2026, 9, 21, 12, 30, 0, 0, time.UTC)); got != PricingPeriodPeak {
		t.Fatalf("20:30 Monday period = %q, want peak", got)
	}
}

func TestLegacyUnifiedPeriodMapsToUnifiedMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`llm:
  pricing:
    period: unified
    input_per_1m_tokens: 1
    output_per_1m_tokens: 4
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	pricing := Load().LLM.Pricing
	if pricing.ModeValue() != PricingModeUnified || pricing.PeriodAt(time.Now()) != PricingPeriodUnified {
		t.Fatalf("legacy unified period not preserved: %+v", pricing)
	}
}

func TestLoadUnifiedCachePricing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`llm:
  pricing:
    mode: unified
    input_cache_hit_per_1m_tokens: 0.02
    input_cache_miss_per_1m_tokens: 1
    output_per_1m_tokens: 4
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	rates := Load().LLM.Pricing.Rates()
	if rates.InputCacheHitPerMillion != 0.02 ||
		rates.InputCacheMissPerMillion != 1 ||
		rates.OutputPerMillion != 4 {
		t.Fatalf("unexpected unified rates: %+v", rates)
	}
}

func TestPricingPeriodAutoSchedule(t *testing.T) {
	pricing := PricingConfig{
		Period:   PricingPeriodAuto,
		Timezone: "Asia/Shanghai",
		Holidays: []string{"2026-10-01"},
	}
	if got := pricing.PeriodAt(time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC)); got != PricingPeriodPeak {
		t.Fatalf("09:00 weekday period = %q, want peak", got)
	}
	if got := pricing.PeriodAt(time.Date(2026, 9, 21, 4, 0, 0, 0, time.UTC)); got != PricingPeriodIdle {
		t.Fatalf("12:00 weekday period = %q, want idle", got)
	}
	if got := pricing.PeriodAt(time.Date(2026, 9, 26, 1, 0, 0, 0, time.UTC)); got != PricingPeriodIdle {
		t.Fatalf("Saturday period = %q, want idle", got)
	}
	if got := pricing.PeriodAt(time.Date(2026, 10, 1, 1, 0, 0, 0, time.UTC)); got != PricingPeriodIdle {
		t.Fatalf("holiday period = %q, want idle", got)
	}
}

func TestPricingPeriodAutoScheduleUsesConfiguredPeakWindows(t *testing.T) {
	pricing := PricingConfig{
		Mode:     PricingModeAuto,
		Timezone: "Asia/Shanghai",
		PeakPeriods: []PricingPeriodWindow{
			{Weekdays: []int{1}, Start: "10:15", End: "10:45"},
		},
	}
	if got := pricing.PeriodAt(time.Date(2026, 9, 21, 2, 14, 0, 0, time.UTC)); got != PricingPeriodIdle {
		t.Fatalf("10:14 Monday period = %q, want idle", got)
	}
	if got := pricing.PeriodAt(time.Date(2026, 9, 21, 2, 15, 0, 0, time.UTC)); got != PricingPeriodPeak {
		t.Fatalf("10:15 Monday period = %q, want peak", got)
	}
	if got := pricing.PeriodAt(time.Date(2026, 9, 21, 2, 45, 0, 0, time.UTC)); got != PricingPeriodIdle {
		t.Fatalf("10:45 Monday period = %q, want idle", got)
	}
}

func TestUnifiedPricingDoesNotUseTieredRates(t *testing.T) {
	pricing := PricingConfig{
		Period:                       PricingPeriodUnified,
		InputPerMillionTokens:        1.5,
		OutputPerMillionTokens:       6,
		InputCacheHitIdlePerMillion:  0.02,
		InputCacheMissIdlePerMillion: 1,
		OutputIdlePerMillion:         4,
	}
	rates := pricing.RatesAt(time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC))
	if rates.InputCacheHitPerMillion != 1.5 ||
		rates.InputCacheMissPerMillion != 1.5 ||
		rates.OutputPerMillion != 6 {
		t.Fatalf("unified rates = %+v", rates)
	}
}

func TestValidateRejectsMissingMySQLDSN(t *testing.T) {
	cfg := defaults()
	cfg.MySQL = []MySQLConfig{{Name: "default", DSN: ""}}
	cfg.Redis.Addr = "127.0.0.1:6379"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted missing MySQL DSN")
	}
}

func TestValidateRejectsEnabledLLMWithoutCredentials(t *testing.T) {
	cfg := defaults()
	cfg.MySQL = []MySQLConfig{{Name: "default", DSN: "root:pwd@tcp(127.0.0.1:3306)/nanyicrm"}}
	cfg.Redis.Addr = "127.0.0.1:6379"
	cfg.LLM.Enabled = true
	cfg.LLM.Provider = "deepseek"
	cfg.LLM.BaseURL = "https://api.deepseek.com"
	cfg.LLM.Model = "deepseek-flash"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted enabled LLM without API key")
	}
}

func TestValidateRejectsInvalidLLMCapabilityConfig(t *testing.T) {
	cfg := defaults()
	cfg.MySQL = []MySQLConfig{{Name: "default", DSN: "root:pwd@tcp(127.0.0.1:3306)/nanyicrm"}}
	cfg.Redis.Addr = "127.0.0.1:6379"
	cfg.LLM.Enabled = true
	cfg.LLM.Provider = "deepseek"
	cfg.LLM.BaseURL = "https://api.example.com"
	cfg.LLM.APIKey = "test-key"
	cfg.LLM.Model = "test-model"
	cfg.LLM.ResponseFormat = "xml"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted unsupported LLM response format")
	}
}

func TestValidateRequiresHTTPSForProductionLLM(t *testing.T) {
	cfg := defaults()
	cfg.App.Env = "prod"
	cfg.MySQL = []MySQLConfig{{Name: "default", DSN: "root:pwd@tcp(127.0.0.1:3306)/nanyicrm"}}
	cfg.Redis.Addr = "127.0.0.1:6379"
	cfg.LLM.Enabled = true
	cfg.LLM.Provider = "deepseek"
	cfg.LLM.BaseURL = "http://api.example.com"
	cfg.LLM.APIKey = "test-key"
	cfg.LLM.Model = "test-model"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate accepted non-HTTPS production LLM URL")
	}
}

func TestProductionRequiresStrongSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "app:\n  env: prod\njwt:\n  signing_key: short\nbootstrap:\n  admin_password: short\n  super_admin_password: short\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	defer func() {
		if recover() == nil {
			t.Fatal("Load did not reject weak production secrets")
		}
	}()
	Load()
}
