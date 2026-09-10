package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server ServerConfig  `yaml:"server"`
	MySQL  []MySQLConfig `yaml:"mysql"`
	Redis  RedisConfig   `yaml:"redis"`
	JWT    JWTConfig     `yaml:"jwt"`
	Log    LogConfig     `yaml:"log"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
	Mode string `yaml:"mode"` // debug / release
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

type LogConfig struct {
	Dir        string `yaml:"dir"`
	File       string `yaml:"file"`
	RetainDays int    `yaml:"retain_days"` // days to keep daily log files, 0 keeps forever
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
	} else if err := yaml.Unmarshal(data, cfg); err != nil {
		// A malformed config file must not silently fall back to defaults.
		panic(fmt.Sprintf("config: parse %s: %v", path, err))
	}

	cfg.applyDefaults()
	return cfg
}

func defaults() *Config {
	return &Config{
		Server: ServerConfig{Port: "8080", Mode: "debug"},
		Redis:  RedisConfig{Addr: "127.0.0.1:6379"},
		Log:    LogConfig{Dir: "log", File: "server.log"},
	}
}

func (c *Config) applyDefaults() {
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
	if c.Redis.Addr == "" {
		c.Redis.Addr = "127.0.0.1:6379"
	}
	if c.JWT.SigningKey == "" {
		c.JWT.SigningKey = "nanyicrm-dev-signing-key-change-me"
	}
	c.JWT.AccessTokenTTL = parseDuration(c.JWT.AccessTTL, 2*time.Hour)
	c.JWT.RefreshTokenTTL = parseDuration(c.JWT.RefreshTTL, 7*24*time.Hour)
	if c.Log.Dir == "" {
		c.Log.Dir = "log"
	}
	if c.Log.File == "" {
		c.Log.File = "server.log"
	}
	if c.Log.RetainDays == 0 {
		c.Log.RetainDays = 30
	}
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
	if strings.Contains(dsn, "timeout=") || strings.Contains(dsn, "readTimeout=") {
		return dsn
	}
	sep := "&"
	if !strings.Contains(dsn, "?") {
		sep = "?"
	}
	return dsn + sep + "timeout=10s&readTimeout=30s&writeTimeout=30s"
}
