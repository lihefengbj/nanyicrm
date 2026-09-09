package main

import (
	"log"

	"github.com/gin-gonic/gin"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/database"
	"github.com/lihefengbj/nanyicrm/backend/internal/router"
	"github.com/lihefengbj/nanyicrm/backend/internal/seed"
)

func main() {
	cfg := config.Load()
	gin.SetMode(cfg.Server.Mode)

	db, err := database.NewMySQL(cfg.MySQL, cfg.Server.Mode == "debug")
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	rdb, err := database.NewRedis(cfg.Redis)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if err := seed.Run(db); err != nil {
		log.Fatalf("seed: %v", err)
	}

	r := router.New(db, rdb, cfg)
	log.Printf("nanyicrm backend listening on :%s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
