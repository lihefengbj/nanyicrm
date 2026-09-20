//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/database"
	"github.com/redis/go-redis/v9"
)

func TestRealMySQLAndRedis(t *testing.T) {
	dsn := os.Getenv("MYSQL_DSN")
	redisAddr := os.Getenv("REDIS_ADDR")
	if dsn == "" || redisAddr == "" {
		t.Skip("MYSQL_DSN and REDIS_ADDR are required for integration tests")
	}

	db, err := database.NewMySQL(config.MySQLConfig{DSN: dsn}, false)
	if err != nil {
		t.Fatalf("mysql connection: %v", err)
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}
	if err := database.RunVersionedMigrations(db); err != nil {
		t.Fatalf("versioned migrations: %v", err)
	}
	for _, table := range []string{
		"sys_ai_model_config",
		"sys_ai_model_change",
		"crm_customer_intent_analysis_archive",
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("migration did not create %s", table)
		}
	}
	table := fmt.Sprintf("codex_integration_probe_%d", time.Now().UnixNano())
	if err := db.Exec("CREATE TABLE " + table + " (id BIGINT PRIMARY KEY, value VARCHAR(64) NOT NULL)").Error; err != nil {
		t.Fatalf("mysql create probe: %v", err)
	}
	defer db.Exec("DROP TABLE " + table)
	if err := db.Exec("INSERT INTO "+table+" (id, value) VALUES (?, ?)", 1, "mysql-ok").Error; err != nil {
		t.Fatalf("mysql insert probe: %v", err)
	}
	var row struct {
		Value string
	}
	if err := db.Table(table).Select("value").Where("id = ?", 1).Scan(&row).Error; err != nil {
		t.Fatalf("mysql select probe: %v", err)
	}
	if row.Value != "mysql-ok" {
		t.Fatalf("mysql value = %q", row.Value)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})
	defer rdb.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := "codex:integration:" + fmt.Sprint(time.Now().UnixNano())
	if err := rdb.Set(ctx, key, "redis-ok", time.Minute).Err(); err != nil {
		t.Fatalf("redis set probe: %v", err)
	}
	defer rdb.Del(context.Background(), key)
	value, err := rdb.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("redis get probe: %v", err)
	}
	if value != "redis-ok" {
		t.Fatalf("redis value = %q", value)
	}
}
