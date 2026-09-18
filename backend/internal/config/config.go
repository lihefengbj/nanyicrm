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
	Bootstrap BootstrapConfig `yaml:"bootstrap"`
}

type AppConfig struct {
	Env string `yaml:"env"` // dev / test / prod; swagger UI is disabled in prod
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
	Enabled        bool          `yaml:"enabled"`
	Provider       string        `yaml:"provider"`
	BaseURL        string        `yaml:"base_url"`
	APIKey         string        `yaml:"api_key"`
	Model          string        `yaml:"model"`
	Timeout        time.Duration `yaml:"-"`
	TimeoutText    string        `yaml:"timeout"`
	MaxTokens      int           `yaml:"max_tokens"`
	Temperature    float32       `yaml:"temperature"`
	ConfigVersion  string        `yaml:"config_version"`
	ResponseFormat string        `yaml:"response_format"`
	ThinkingMode   string        `yaml:"thinking_mode"`
	Quota          QuotaConfig   `yaml:"quota"`
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
			ResponseFormat: "json_object",
			Quota: QuotaConfig{
				DailyCalls:  1000,
				DailyTokens: 2000000,
				Concurrency: 5,
			},
		},
		Log: LogConfig{Dir: "log", File: "server.log", RetainDays: 30},
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
	if c.LLM.ConfigVersion == "" {
		c.LLM.ConfigVersion = "v1"
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
		if strings.TrimSpace(c.LLM.APIKey) == "" {
			return fmt.Errorf("llm.api_key is required when llm.enabled=true; check LLM_API_KEY")
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
