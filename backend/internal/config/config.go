package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Server ServerConfig
	MySQL  MySQLConfig
	Redis  RedisConfig
	JWT    JWTConfig
}

type ServerConfig struct {
	Port string
	Mode string // debug / release
}

type MySQLConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr string
	Pwd  string
	DB   int
}

type JWTConfig struct {
	SigningKey      string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

// Load reads configuration from environment variables, falling back to
// local-development defaults. Secrets must be provided via env in production.
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
			Mode: getEnv("GIN_MODE", "debug"),
		},
		MySQL: MySQLConfig{
			DSN: withTimeoutDefaults(getEnv("MYSQL_DSN", "root:root@tcp(127.0.0.1:3306)/nanyicrm?charset=utf8mb4&parseTime=True&loc=Local")),
		},
		Redis: RedisConfig{
			Addr: getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			Pwd:  getEnv("REDIS_PWD", ""),
			DB:   getEnvInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			SigningKey:      getEnv("JWT_SIGNING_KEY", "nanyicrm-dev-signing-key-change-me"),
			AccessTokenTTL:  getEnvDuration("JWT_ACCESS_TTL", 2*time.Hour),
			RefreshTokenTTL: getEnvDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		},
	}
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

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
