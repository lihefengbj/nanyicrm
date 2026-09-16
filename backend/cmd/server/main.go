package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	_ "github.com/lihefengbj/nanyicrm/backend/internal/apidocs"
	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/database"
	"github.com/lihefengbj/nanyicrm/backend/internal/logger"
	"github.com/lihefengbj/nanyicrm/backend/internal/modules/system"
	"github.com/lihefengbj/nanyicrm/backend/internal/router"
	"github.com/lihefengbj/nanyicrm/backend/internal/seed"
)

// @title        Nanyi CRM API
// @version      1.0
// @description  Nanyi CRM 管理后台接口文档。统一前缀 /api/v1，响应结构 { code, message, data }，分页结构 { records, total, pageNum, pageSize }。异常码段：1xxx 参数 / 2xxx 认证授权 / 3xxx 业务 / 5xxx 系统。
// @host         localhost:8080
// @BasePath     /api/v1
// @securityDefinitions.apikey  BearerAuth
// @in                          header
// @name                        Authorization
// @description                 输入 "Bearer {access_token}"
func main() {
	cfg := config.Load()
	gin.SetMode(cfg.Server.Mode)

	closer, err := logger.Setup(cfg.Log)
	if err != nil {
		log.Fatalf("logger: %v", err)
	}
	defer closer.Close()

	// Connect all configured MySQL instances at once; the one named
	// "default" (or the first entry) drives migrate/seed/business logic.
	dbs, err := database.NewMySQLAll(cfg.MySQL, cfg.Server.Mode == "debug")
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defaultName := cfg.DefaultMySQL().Name
	if defaultName == "" {
		defaultName = "default"
	}
	db := dbs[defaultName]
	defer func() {
		for name, gormDB := range dbs {
			sqlDB, err := gormDB.DB()
			if err == nil {
				if err := sqlDB.Close(); err != nil {
					log.Printf("close mysql[%s]: %v", name, err)
				}
			}
		}
	}()

	rdb, err := database.NewRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer rdb.Close()
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := seed.Run(db, cfg.Bootstrap); err != nil {
		log.Fatalf("seed: %v", err)
	}

	r, apiRegistry := router.New(db, rdb, cfg)
	system.SyncApis(db, apiRegistry)
	log.Printf("nanyicrm backend listening on :%s (%d mysql connection(s))", cfg.Server.Port, len(dbs))

	server := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
		log.Printf("shutdown signal received")
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}
}
