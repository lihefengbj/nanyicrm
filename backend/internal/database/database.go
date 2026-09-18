package database

import (
	"context"
	"fmt"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
)

func NewMySQL(cfg config.MySQLConfig, debug bool) (*gorm.DB, error) {
	logLevel := gormlogger.Warn
	if debug {
		logLevel = gormlogger.Info
	}
	dsn, err := tuneMySQLDSN(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("prepare mysql dsn: %w", err)
	}
	// The remote MySQL occasionally accepts TCP but stalls the handshake;
	// retry with backoff so a transient blip does not kill the process.
	var db *gorm.DB
	err = nil
	for attempt := 1; attempt <= 5; attempt++ {
		db, err = gorm.Open(gormmysql.Open(dsn), &gorm.Config{
			Logger: gormlogger.Default.LogMode(logLevel),
			// tenant_id = 0 means "platform / no tenant", so a real FK to
			// sys_tenant would reject it. Manage referential integrity in
			// code instead of database constraints.
			DisableForeignKeyConstraintWhenMigrating: true,
		})
		if err == nil {
			break
		}
		if attempt == 5 {
			return nil, fmt.Errorf("connect mysql: %w", err)
		}
		time.Sleep(time.Duration(attempt) * 2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// Keep the pool deliberately small for the remote development database.
	// A large burst of new TCP handshakes can exceed the NAT/database limit
	// during auto-migration or a batch analysis. One connection also keeps the
	// request path from opening a second handshake while the first one is
	// still being recycled by the remote endpoint.
	sqlDB.SetMaxOpenConns(1)
	// This database is remote and sits behind a NAT. Keeping idle sockets
	// around makes it possible for the NAT or MySQL to drop them before the
	// next request gets a chance to use them.
	sqlDB.SetMaxIdleConns(1)
	// The remote MySQL is behind a NAT that silently drops idle TCP
	// connections. AI analysis can leave a pooled connection idle while it
	// waits for the provider, so keep the idle lifetime shorter than the
	// observed network idle window and avoid reusing a dead connection.
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(30 * time.Second)
	return db, nil
}

func tuneMySQLDSN(raw string) (string, error) {
	parsed, err := mysql.ParseDSN(raw)
	if err != nil {
		return "", err
	}
	// Keep explicit user-provided values, while ensuring a dead remote
	// connection cannot block a request indefinitely.
	if parsed.Timeout == 0 {
		parsed.Timeout = 5 * time.Second
	}
	if parsed.ReadTimeout == 0 {
		parsed.ReadTimeout = 10 * time.Second
	}
	if parsed.WriteTimeout == 0 {
		parsed.WriteTimeout = 10 * time.Second
	}
	parsed.CheckConnLiveness = true
	return parsed.FormatDSN(), nil
}

// NewMySQLAll opens every configured MySQL connection and returns them keyed
// by name, so business code can hold several databases at the same time.
func NewMySQLAll(cfgs []config.MySQLConfig, debug bool) (map[string]*gorm.DB, error) {
	dbs := make(map[string]*gorm.DB, len(cfgs))
	for _, c := range cfgs {
		name := c.Name
		if name == "" {
			name = "default"
		}
		if _, dup := dbs[name]; dup {
			return nil, fmt.Errorf("duplicate mysql connection name %q", name)
		}
		db, err := NewMySQL(c, debug)
		if err != nil {
			return nil, fmt.Errorf("mysql[%s]: %w", name, err)
		}
		dbs[name] = db
	}
	return dbs, nil
}

func NewRedis(cfg config.RedisConfig) (*redis.Client, error) {
	var lastErr error
	for attempt := 1; attempt <= 5; attempt++ {
		rdb := redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Pwd,
			DB:       cfg.DB,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := rdb.Ping(ctx).Err()
		cancel()
		if err == nil {
			return rdb, nil
		}
		lastErr = err
		_ = rdb.Close()
		if attempt < 5 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}
	return nil, fmt.Errorf("connect redis after retries: %w", lastErr)
}
