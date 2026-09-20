// purge-intent-data removes expired AI intent snapshots and historical
// records. The server also runs the same cleanup daily when retention_days is
// configured, while this command is useful for one-off maintenance.
package main

import (
	"context"
	"log"
	"time"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/database"
	"github.com/lihefengbj/nanyicrm/backend/internal/retention"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.LLM.RetentionDays <= 0 {
		log.Println("intent retention cleanup disabled: llm.retention_days must be greater than zero")
		return
	}

	db, err := database.NewMySQL(cfg.DefaultMySQL(), false)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	cutoff := time.Now().AddDate(0, 0, -cfg.LLM.RetentionDays)
	report, err := retention.PurgeIntentData(context.Background(), db, cutoff)
	if err != nil {
		log.Fatalf("purge intent data: %v", err)
	}
	log.Printf("purged intent data cutoff=%s redacted_analyses=%d deleted_analyses=%d deleted_feedback=%d deleted_task_items=%d deleted_tasks=%d",
		cutoff.Format(time.RFC3339), report.RedactedAnalyses, report.DeletedAnalyses,
		report.DeletedFeedback, report.DeletedTaskItems, report.DeletedTasks)
}
