package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App       AppConfig       `yaml:"app"`
	Server    ServerConfig    `yaml:"server"`
	MySQL     []MySQLConfig   `yaml:"mysql"`
	Redis     RedisConfig     `yaml:"redis"`
	JWT       JWTConfig       `yaml:"jwt"`
	LLM       LLMConfig       `yaml:"llm"`
	Log       LogConfig       `yaml:"log"`
	Alerts    AlertConfig     `yaml:"alerts"`
	Metrics   MetricsConfig   `yaml:"metrics"`
	Bootstrap BootstrapConfig `yaml:"bootstrap"`
}

type AppConfig struct {
	Env         string `yaml:"env"` // dev / test / prod; swagger UI is disabled in prod
	AutoMigrate bool   `yaml:"auto_migrate"`
}

type ServerConfig struct {
	Port           string   `yaml:"port"`
	Mode           string   `yaml:"mode"` // debug / release
	AllowedOrigins []string `yaml:"allowed_origins"`
	TrustedProxies []string `yaml:"trusted_proxies"`
}

// MySQLConfig describes one MySQL connection. Multiple entries are supported;
// Name identifies the connection, "default" is the primary business database.
type MySQLConfig struct {
	Name string `yaml:"name"`
	DSN  string `yaml:"dsn"`
}

type RedisConfig struct {
	Addr string `yaml:"addr"`
	Pwd  string `yaml:"pwd"`
	DB   int    `yaml:"db"`
}

type JWTConfig struct {
	SigningKey      string        `yaml:"signing_key"`
	AccessTokenTTL  time.Duration `yaml:"-"`
	RefreshTokenTTL time.Duration `yaml:"-"`
	// raw string forms as written in the yaml file, e.g. "2h"
	AccessTTL  string `yaml:"access_ttl"`
	RefreshTTL string `yaml:"refresh_ttl"`
}

type LLMConfig struct {
	Enabled                 bool          `yaml:"enabled"`
	Provider                string        `yaml:"provider"`
	BaseURL                 string        `yaml:"base_url"`
	APIKey                  string        `yaml:"api_key"`
	CredentialEncryptionKey string        `yaml:"credential_encryption_key"`
	Model                   string        `yaml:"model"`
	Timeout                 time.Duration `yaml:"-"`
	TimeoutText             string        `yaml:"timeout"`
	MaxTokens               int           `yaml:"max_tokens"`
	Temperature             float32       `yaml:"temperature"`
	ConfigVersion           string        `yaml:"config_version"`
	PromptVersion           string        `yaml:"prompt_version"`
	ResponseFormat          string        `yaml:"response_format"`
	ThinkingMode            string        `yaml:"thinking_mode"`
	RetentionDays           int           `yaml:"retention_days"` // raw AI snapshots to keep; 0 disables cleanup
	ArchiveDays             int           `yaml:"archive_days"`   // archived AI records to keep after live cleanup
	Pricing                 PricingConfig `yaml:"pricing"`
	Quota                   QuotaConfig   `yaml:"quota"`
}

const (
	PricingModeAuto      = "auto"
	PricingModeUnified   = "unified"
	PricingPeriodAuto    = "auto"
	PricingPeriodIdle    = "idle"
	PricingPeriodPeak    = "peak"
	PricingPeriodUnified = "unified"
)

// PricingConfig describes an optional estimate in the configured currency.
// Tiered prices are per one million provider tokens. The legacy input/output
// fields remain supported for existing deployments and treat all input tokens
// as uncached tokens.
type PricingConfig struct {
	Currency                        string                `yaml:"currency"`
	Mode                            string                `yaml:"mode"`
	FixedPeriod                     string                `yaml:"fixed_period"`
	Period                          string                `yaml:"period"` // Deprecated: use mode/fixed_period.
	Timezone                        string                `yaml:"timezone"`
	Holidays                        []string              `yaml:"holidays"`
	PeakPeriods                     []PricingPeriodWindow `yaml:"peak_periods"`
	InputCacheHitIdlePerMillion     float64               `yaml:"input_cache_hit_idle_per_1m_tokens"`
	InputCacheHitPeakPerMillion     float64               `yaml:"input_cache_hit_peak_per_1m_tokens"`
	InputCacheMissIdlePerMillion    float64               `yaml:"input_cache_miss_idle_per_1m_tokens"`
	InputCacheMissPeakPerMillion    float64               `yaml:"input_cache_miss_peak_per_1m_tokens"`
	OutputIdlePerMillion            float64               `yaml:"output_idle_per_1m_tokens"`
	OutputPeakPerMillion            float64               `yaml:"output_peak_per_1m_tokens"`
	InputCacheHitUnifiedPerMillion  float64               `yaml:"input_cache_hit_per_1m_tokens"`
	InputCacheMissUnifiedPerMillion float64               `yaml:"input_cache_miss_per_1m_tokens"`

	// Deprecated: use the tiered fields above.
	InputPerMillionTokens  float64 `yaml:"input_per_1m_tokens"`
	OutputPerMillionTokens float64 `yaml:"output_per_1m_tokens"`
}

// PricingPeriodWindow defines one recurring peak-price window. Weekdays use
// Go's time.Weekday values: Sunday=0, Monday=1, ..., Saturday=6.
// The start is inclusive and the end is exclusive; idle time is the
// complement of all matching peak windows.
type PricingPeriodWindow struct {
	Weekdays []int  `yaml:"weekdays"`
	Start    string `yaml:"start"`
	End      string `yaml:"end"`
}

type PricingRates struct {
	InputCacheHitPerMillion  float64
	InputCacheMissPerMillion float64
	OutputPerMillion         float64
}

func defaultPeakPeriods() []PricingPeriodWindow {
	return []PricingPeriodWindow{
		{Weekdays: []int{1, 2, 3, 4, 5}, Start: "09:00", End: "12:00"},
		{Weekdays: []int{1, 2, 3, 4, 5}, Start: "14:00", End: "18:00"},
	}
}

func (w PricingPeriodWindow) contains(at time.Time) bool {
	weekdayMatches := false
	for _, weekday := range w.Weekdays {
		if weekday == int(at.Weekday()) {
			weekdayMatches = true
			break
		}
	}
	if !weekdayMatches {
		return false
	}

	start, err := time.Parse("15:04", strings.TrimSpace(w.Start))
	if err != nil {
		return false
	}
	end, err := time.Parse("15:04", strings.TrimSpace(w.End))
	if err != nil {
		return false
	}
	startMinute := start.Hour()*60 + start.Minute()
	endMinute := end.Hour()*60 + end.Minute()
	if endMinute <= startMinute {
		return false
	}
	currentMinute := at.Hour()*60 + at.Minute()
	return currentMinute >= startMinute && currentMinute < endMinute
}

func (p PricingConfig) peakPeriods() []PricingPeriodWindow {
	if len(p.PeakPeriods) == 0 {
		return defaultPeakPeriods()
	}
	return p.PeakPeriods
}

func (p PricingConfig) HasTieredRates() bool {
	return p.InputCacheHitIdlePerMillion > 0 ||
		p.InputCacheHitPeakPerMillion > 0 ||
		p.InputCacheMissIdlePerMillion > 0 ||
		p.InputCacheMissPeakPerMillion > 0 ||
		p.OutputIdlePerMillion > 0 ||
		p.OutputPeakPerMillion > 0
}

func (p PricingConfig) IsConfigured() bool {
	return p.HasTieredRates() || p.HasUnifiedRates()
}

func (p PricingConfig) Rates() PricingRates {
	return p.RatesAt(time.Now())
}

func (p PricingConfig) RatesAt(at time.Time) PricingRates {
	return p.RatesForPeriod(p.PeriodAt(at))
}

func (p PricingConfig) RatesForPeriod(period string) PricingRates {
	if strings.EqualFold(period, PricingPeriodUnified) {
		return p.UnifiedRates()
	}
	if !p.HasTieredRates() {
		return p.UnifiedRates()
	}
	if strings.EqualFold(period, PricingPeriodPeak) {
		return PricingRates{
			InputCacheHitPerMillion:  p.InputCacheHitPeakPerMillion,
			InputCacheMissPerMillion: p.InputCacheMissPeakPerMillion,
			OutputPerMillion:         p.OutputPeakPerMillion,
		}
	}
	return PricingRates{
		InputCacheHitPerMillion:  p.InputCacheHitIdlePerMillion,
		InputCacheMissPerMillion: p.InputCacheMissIdlePerMillion,
		OutputPerMillion:         p.OutputIdlePerMillion,
	}
}

func (p PricingConfig) HasUnifiedRates() bool {
	return p.InputCacheHitUnifiedPerMillion > 0 ||
		p.InputCacheMissUnifiedPerMillion > 0 ||
		p.InputPerMillionTokens > 0 ||
		p.OutputPerMillionTokens > 0
}

func (p PricingConfig) UnifiedRates() PricingRates {
	inputHit := p.InputCacheHitUnifiedPerMillion
	inputMiss := p.InputCacheMissUnifiedPerMillion
	if inputHit == 0 && inputMiss == 0 {
		inputHit = p.InputPerMillionTokens
		inputMiss = p.InputPerMillionTokens
	}
	return PricingRates{
		InputCacheHitPerMillion:  inputHit,
		InputCacheMissPerMillion: inputMiss,
		OutputPerMillion:         p.OutputPerMillionTokens,
	}
}

func (p PricingConfig) ModeValue() string {
	mode := strings.ToLower(strings.TrimSpace(p.Mode))
	if mode == "" {
		if strings.EqualFold(strings.TrimSpace(p.Period), PricingPeriodUnified) {
			return PricingModeUnified
		}
		return PricingModeAuto
	}
	if mode != PricingModeUnified {
		return PricingModeAuto
	}
	return mode
}

func (p PricingConfig) FixedPeriodValue() string {
	fixed := strings.ToLower(strings.TrimSpace(p.FixedPeriod))
	if fixed == PricingPeriodIdle || fixed == PricingPeriodPeak {
		return fixed
	}
	// Backward compatibility for the old period: idle/peak meant a forced
	// period, while unified meant a unified pricing mode.
	if p.Mode == "" {
		if fixed := strings.ToLower(strings.TrimSpace(p.Period)); fixed == PricingPeriodIdle || fixed == PricingPeriodPeak {
			return fixed
		}
	}
	return ""
}

func (p PricingConfig) PeriodAt(at time.Time) string {
	if p.ModeValue() == PricingModeUnified {
		return PricingPeriodUnified
	}
	if fixed := p.FixedPeriodValue(); fixed != "" {
		return fixed
	}

	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	if timezone := strings.TrimSpace(p.Timezone); timezone != "" {
		if loaded, err := time.LoadLocation(timezone); err == nil {
			location = loaded
		}
	}
	local := at.In(location)
	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday || p.IsHoliday(local) {
		return PricingPeriodIdle
	}
	for _, window := range p.peakPeriods() {
		if window.contains(local) {
			return PricingPeriodPeak
		}
	}
	return PricingPeriodIdle
}

func (p PricingConfig) IsHoliday(date time.Time) bool {
	dateText := date.Format("2006-01-02")
	for _, holiday := range p.Holidays {
		if strings.TrimSpace(holiday) == dateText {
			return true
		}
	}
	return false
}

// QuotaConfig holds platform-level default AI intent quotas. A value of 0
// means "no limit"; each tenant may override these via sys_tenant columns.
type QuotaConfig struct {
	DailyCalls  int64 `yaml:"daily_calls"`  // per-tenant daily analysis count limit
	DailyTokens int64 `yaml:"daily_tokens"` // per-tenant daily provider token usage limit
	Concurrency int64 `yaml:"concurrency"`  // per-tenant concurrent in-flight analyses
}

type LogConfig struct {
	Dir        string `yaml:"dir"`
	File       string `yaml:"file"`
	RetainDays int    `yaml:"retain_days"` // days to keep daily log files, 0 keeps forever
}

type AlertConfig struct {
	Enabled               bool          `yaml:"enabled"`
	WebhookURL            string        `yaml:"webhook_url"`
	LoginFailureThreshold int           `yaml:"login_failure_threshold"`
	WindowText            string        `yaml:"window"`
	Window                time.Duration `yaml:"-"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"`
}

type BootstrapConfig struct {
	AdminPassword      string `yaml:"admin_password"`
	SuperAdminPassword string `yaml:"super_admin_password"`
}

// Load reads configuration from the yaml file at CONFIG_PATH, falling back to
// config/config.yaml (relative to the working directory) and then to built-in
// local-development defaults.
func Load() *Config {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = filepath.Join("config", "config.yaml")
	}

	cfg := defaults()
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("config: %v, using built-in defaults\n", err)
	} else if err := yaml.Unmarshal([]byte(os.ExpandEnv(string(data))), cfg); err != nil {
		// A malformed config file must not silently fall back to defaults.
		panic(fmt.Sprintf("config: parse %s: %v", path, err))
	}

	cfg.applyDefaults()
	return cfg
}

func defaults() *Config {
	return &Config{
		App: AppConfig{Env: "dev", AutoMigrate: true},
		Server: ServerConfig{
			Port:           "8080",
			Mode:           "debug",
			AllowedOrigins: []string{"http://localhost:5173", "http://127.0.0.1:5173"},
			TrustedProxies: []string{"127.0.0.1", "::1"},
		},
		Redis: RedisConfig{Addr: "127.0.0.1:6379"},
		LLM: LLMConfig{
			Provider:       "openai-compatible",
			TimeoutText:    "30s",
			MaxTokens:      1200,
			Temperature:    0.2,
			ConfigVersion:  "v1",
			PromptVersion:  "v1",
			ResponseFormat: "json_object",
			Pricing: PricingConfig{
				Currency:    "CNY",
				Mode:        PricingModeAuto,
				Timezone:    "Asia/Shanghai",
				PeakPeriods: defaultPeakPeriods(),
			},
			Quota: QuotaConfig{
				DailyCalls:  1000,
				DailyTokens: 2000000,
				Concurrency: 5,
			},
			ArchiveDays: 90,
		},
		Log: LogConfig{Dir: "log", File: "server.log", RetainDays: 30},
		Alerts: AlertConfig{
			LoginFailureThreshold: 5,
			WindowText:            "10m",
		},
		Metrics: MetricsConfig{Enabled: true},
		Bootstrap: BootstrapConfig{
			AdminPassword:      "admin123",
			SuperAdminPassword: "superAdmin123",
		},
	}
}

func (c *Config) applyDefaults() {
	if c.App.Env == "" {
		c.App.Env = "dev"
	}
	if c.App.Env == "prod" && c.App.AutoMigrate {
		panic("config: production app.auto_migrate must be false")
	}
	if c.Server.Port == "" {
		c.Server.Port = "8080"
	}
	if c.Server.Mode == "" {
		c.Server.Mode = "debug"
	}
	for i := range c.MySQL {
		c.MySQL[i].DSN = withTimeoutDefaults(c.MySQL[i].DSN)
	}
	if len(c.MySQL) == 0 {
		c.MySQL = []MySQLConfig{{
			Name: "default",
			DSN:  withTimeoutDefaults("root:root@tcp(127.0.0.1:3306)/nanyicrm?charset=utf8mb4&parseTime=True&loc=Local"),
		}}
	}
	if c.JWT.SigningKey == "" {
		c.JWT.SigningKey = "nanyicrm-dev-signing-key-change-me"
	}
	c.JWT.AccessTokenTTL = parseDuration(c.JWT.AccessTTL, 2*time.Hour)
	c.JWT.RefreshTokenTTL = parseDuration(c.JWT.RefreshTTL, 7*24*time.Hour)
	if c.LLM.Provider == "" {
		c.LLM.Provider = "openai-compatible"
	}
	if c.LLM.CredentialEncryptionKey == "" && c.App.Env != "prod" {
		// Local development keeps working without another secret while
		// production deployments can provide a dedicated key through env.
		c.LLM.CredentialEncryptionKey = c.JWT.SigningKey
	}
	if c.LLM.ConfigVersion == "" {
		c.LLM.ConfigVersion = "v1"
	}
	if c.LLM.PromptVersion == "" {
		c.LLM.PromptVersion = "v1"
	}
	if c.LLM.ResponseFormat == "" {
		c.LLM.ResponseFormat = "json_object"
	}
	if c.LLM.ThinkingMode == "" && strings.EqualFold(c.LLM.Provider, "deepseek") {
		c.LLM.ThinkingMode = "disabled"
	}
	c.LLM.Timeout = parseDuration(c.LLM.TimeoutText, 30*time.Second)
	if c.LLM.MaxTokens <= 0 {
		c.LLM.MaxTokens = 1200
	}
	if c.LLM.Temperature < 0 {
		c.LLM.Temperature = 0
	}
	if c.LLM.Temperature > 2 {
		c.LLM.Temperature = 2
	}
	if c.LLM.RetentionDays < 0 {
		c.LLM.RetentionDays = 0
	}
	if c.LLM.ArchiveDays < 0 {
		c.LLM.ArchiveDays = 0
	}
	if c.LLM.Pricing.Currency == "" {
		c.LLM.Pricing.Currency = "CNY"
	}
	legacyPeriod := strings.ToLower(strings.TrimSpace(c.LLM.Pricing.Period))
	c.LLM.Pricing.Mode = strings.ToLower(strings.TrimSpace(c.LLM.Pricing.Mode))
	if legacyPeriod == PricingPeriodUnified && c.LLM.Pricing.FixedPeriod == "" {
		c.LLM.Pricing.Mode = PricingModeUnified
	}
	if c.LLM.Pricing.Mode != PricingModeUnified {
		c.LLM.Pricing.Mode = PricingModeAuto
	}
	c.LLM.Pricing.FixedPeriod = strings.ToLower(strings.TrimSpace(c.LLM.Pricing.FixedPeriod))
	if c.LLM.Pricing.FixedPeriod == "" &&
		(legacyPeriod == PricingPeriodIdle || legacyPeriod == PricingPeriodPeak) {
		c.LLM.Pricing.FixedPeriod = legacyPeriod
	}
	if c.LLM.Pricing.FixedPeriod != PricingPeriodIdle && c.LLM.Pricing.FixedPeriod != PricingPeriodPeak {
		c.LLM.Pricing.FixedPeriod = ""
	}
	if c.LLM.Pricing.Timezone == "" {
		c.LLM.Pricing.Timezone = "Asia/Shanghai"
	}
	if len(c.LLM.Pricing.PeakPeriods) == 0 {
		c.LLM.Pricing.PeakPeriods = defaultPeakPeriods()
	}
	if c.LLM.Pricing.InputCacheHitIdlePerMillion < 0 {
		c.LLM.Pricing.InputCacheHitIdlePerMillion = 0
	}
	if c.LLM.Pricing.InputCacheHitPeakPerMillion < 0 {
		c.LLM.Pricing.InputCacheHitPeakPerMillion = 0
	}
	if c.LLM.Pricing.InputCacheMissIdlePerMillion < 0 {
		c.LLM.Pricing.InputCacheMissIdlePerMillion = 0
	}
	if c.LLM.Pricing.InputCacheMissPeakPerMillion < 0 {
		c.LLM.Pricing.InputCacheMissPeakPerMillion = 0
	}
	if c.LLM.Pricing.OutputIdlePerMillion < 0 {
		c.LLM.Pricing.OutputIdlePerMillion = 0
	}
	if c.LLM.Pricing.OutputPeakPerMillion < 0 {
		c.LLM.Pricing.OutputPeakPerMillion = 0
	}
	if c.LLM.Pricing.InputCacheHitUnifiedPerMillion < 0 {
		c.LLM.Pricing.InputCacheHitUnifiedPerMillion = 0
	}
	if c.LLM.Pricing.InputCacheMissUnifiedPerMillion < 0 {
		c.LLM.Pricing.InputCacheMissUnifiedPerMillion = 0
	}
	if c.LLM.Pricing.InputPerMillionTokens < 0 {
		c.LLM.Pricing.InputPerMillionTokens = 0
	}
	if c.LLM.Pricing.OutputPerMillionTokens < 0 {
		c.LLM.Pricing.OutputPerMillionTokens = 0
	}
	// Negative quotas are configuration errors; treat them as "no limit"
	// instead of failing startup for a local development config.
	if c.LLM.Quota.DailyCalls < 0 {
		c.LLM.Quota.DailyCalls = 0
	}
	if c.LLM.Quota.DailyTokens < 0 {
		c.LLM.Quota.DailyTokens = 0
	}
	if c.LLM.Quota.Concurrency < 0 {
		c.LLM.Quota.Concurrency = 0
	}
	if c.Log.Dir == "" {
		c.Log.Dir = "log"
	}
	if c.Log.File == "" {
		c.Log.File = "server.log"
	}
	if c.Alerts.LoginFailureThreshold <= 0 {
		c.Alerts.LoginFailureThreshold = 5
	}
	c.Alerts.Window = parseDuration(c.Alerts.WindowText, 10*time.Minute)
	if c.Alerts.Window <= 0 {
		c.Alerts.Window = 10 * time.Minute
	}
	if c.App.Env == "prod" {
		if len(c.JWT.SigningKey) < 32 || c.JWT.SigningKey == "nanyicrm-dev-signing-key-change-me" {
			panic("config: production jwt.signing_key must be at least 32 characters")
		}
		if len(c.MySQL) == 0 || strings.TrimSpace(c.DefaultMySQL().DSN) == "" {
			panic("config: production mysql.dsn is required")
		}
		if len(c.Bootstrap.AdminPassword) < 12 || c.Bootstrap.AdminPassword == "admin123" {
			panic("config: production bootstrap.admin_password must be at least 12 characters")
		}
		if len(c.Bootstrap.SuperAdminPassword) < 12 || c.Bootstrap.SuperAdminPassword == "superAdmin123" {
			panic("config: production bootstrap.super_admin_password must be at least 12 characters")
		}
	}
}

// Validate checks configuration before any external connection is opened.
// This keeps missing environment variables from surfacing later as opaque
// driver errors such as "missing the slash separating the database name".
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("configuration is nil")
	}
	if len(c.MySQL) == 0 {
		return fmt.Errorf("mysql.default.dsn is required")
	}
	for _, item := range c.MySQL {
		name := item.Name
		if name == "" {
			name = "default"
		}
		dsn := strings.TrimSpace(item.DSN)
		if dsn == "" {
			return fmt.Errorf("mysql.%s.dsn is required; check MYSQL_DSN or CONFIG_PATH", name)
		}
		if _, err := mysql.ParseDSN(dsn); err != nil {
			return fmt.Errorf("mysql.%s.dsn is invalid: %w", name, err)
		}
	}
	if strings.TrimSpace(c.Redis.Addr) == "" {
		return fmt.Errorf("redis.addr is required; check REDIS_ADDR or CONFIG_PATH")
	}
	if c.LLM.Enabled {
		if strings.TrimSpace(c.LLM.Provider) == "" {
			return fmt.Errorf("llm.provider is required when llm.enabled=true")
		}
		if strings.TrimSpace(c.LLM.BaseURL) == "" {
			return fmt.Errorf("llm.base_url is required when llm.enabled=true")
		}
		if c.App.Env == "prod" && strings.TrimSpace(c.LLM.CredentialEncryptionKey) == "" {
			return fmt.Errorf("llm.credential_encryption_key is required in production; check LLM_CREDENTIAL_ENCRYPTION_KEY")
		}
		if strings.TrimSpace(c.LLM.Model) == "" {
			return fmt.Errorf("llm.model is required when llm.enabled=true")
		}
		baseURL, err := url.Parse(strings.TrimSpace(c.LLM.BaseURL))
		if err != nil || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
			return fmt.Errorf("llm.base_url must be a valid http or https URL")
		}
		if c.App.Env == "prod" && baseURL.Scheme != "https" {
			return fmt.Errorf("llm.base_url must use https in production")
		}
		switch c.LLM.ResponseFormat {
		case "", "json_object":
		default:
			return fmt.Errorf("llm.response_format %q is unsupported", c.LLM.ResponseFormat)
		}
		switch c.LLM.ThinkingMode {
		case "", "disabled", "enabled":
		default:
			return fmt.Errorf("llm.thinking_mode must be empty, disabled or enabled")
		}
	}
	if c.App.Env == "prod" && c.Metrics.Enabled {
		metricsToken := strings.TrimSpace(c.Metrics.Token)
		if len(metricsToken) < 16 || metricsToken == "change-me-metrics-token" {
			return fmt.Errorf("metrics.token must be a non-default value of at least 16 characters in production")
		}
	}
	if c.Alerts.Enabled && strings.TrimSpace(c.Alerts.WebhookURL) == "" {
		return fmt.Errorf("alerts.webhook_url is required when alerts.enabled=true")
	}
	return nil
}

// DefaultMySQL returns the connection named "default", or the first one.
func (c *Config) DefaultMySQL() MySQLConfig {
	for _, m := range c.MySQL {
		if m.Name == "default" {
			return m
		}
	}
	return c.MySQL[0]
}

func parseDuration(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(fmt.Sprintf("config: invalid duration %q: %v", s, err))
	}
	return d
}

// withTimeoutDefaults appends connect/read/write timeouts to the DSN when the
// caller did not set any. Guards against silently-dropped remote connections
// hanging queries for tens of seconds.
func withTimeoutDefaults(dsn string) string {
	if strings.TrimSpace(dsn) == "" {
		return ""
	}
	if strings.Contains(dsn, "timeout=") || strings.Contains(dsn, "readTimeout=") {
		return dsn
	}
	sep := "&"
	if !strings.Contains(dsn, "?") {
		sep = "?"
	}
	return dsn + sep + "timeout=5s&readTimeout=10s&writeTimeout=10s"
}
