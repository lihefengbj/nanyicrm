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
	db, err := gorm.Open(gormmysql.Open(cfg.DSN), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("connect mysql: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)
	return db, nil
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
