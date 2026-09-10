package database

import (
	"context"
	"fmt"
	"time"

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
	// The remote MySQL occasionally accepts TCP but stalls the handshake;
	// retry with backoff so a transient blip does not kill the process.
	var db *gorm.DB
	var err error
	for attempt := 1; attempt <= 5; attempt++ {
		db, err = gorm.Open(gormmysql.Open(cfg.DSN), &gorm.Config{
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
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	// The remote MySQL is behind a NAT that silently drops idle TCP
	// connections; recycle pooled connections aggressively so we never
	// reuse a dead one (surfaced as "invalid connection" after ~20s hangs).
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(30 * time.Second)
	return db, nil
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
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Pwd,
		DB:       cfg.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	return rdb, nil
}
