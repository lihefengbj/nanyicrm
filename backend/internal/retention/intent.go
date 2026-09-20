// Package retention removes or redacts AI intent data after its configured
// retention window. Raw provider input and output are more sensitive than the
// derived current intent, so the current record is kept but its snapshots are
// cleared when it is still referenced.
package retention

import (
	"context"
	"errors"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type Report struct {
	RedactedAnalyses int64
	DeletedAnalyses  int64
	DeletedFeedback  int64
	DeletedTaskItems int64
	DeletedTasks     int64
}

// PurgeIntentData redacts snapshots on retained current analyses and
// physically removes expired historical analyses, feedback, and completed
// batch tasks. A zero or negative cutoff is rejected to prevent accidental
// full-table deletion.
func PurgeIntentData(ctx context.Context, db *gorm.DB, cutoff time.Time) (Report, error) {
	if db == nil {
		return Report{}, errors.New("retention: database is nil")
	}
	if cutoff.IsZero() {
		return Report{}, errors.New("retention: cutoff is required")
	}

	var report Report
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		currentIDs := tx.Model(&model.CrmCustomerIntent{}).
			Select("analysis_id").
			Where("analysis_id > 0 AND deleted_at IS NULL")

		redacted := tx.Unscoped().
			Model(&model.CrmCustomerIntentAnalysis{}).
			Where("created_at < ?", cutoff).
			Where("id IN (?)", currentIDs).
			Updates(map[string]interface{}{
				"input_snapshot":  "",
				"result_snapshot": "",
			})
		if redacted.Error != nil {
			return redacted.Error
		}
		report.RedactedAnalyses = redacted.RowsAffected

		var analysisIDs []uint64
		if err := tx.Unscoped().
			Model(&model.CrmCustomerIntentAnalysis{}).
			Where("created_at < ?", cutoff).
			Where(`NOT EXISTS (
				SELECT 1
				FROM crm_customer_intent AS ci
				WHERE ci.analysis_id = crm_customer_intent_analysis.id
				  AND ci.deleted_at IS NULL
			)`).
			Pluck("id", &analysisIDs).Error; err != nil {
			return err
		}
		if len(analysisIDs) > 0 {
			deletedFeedback := tx.Unscoped().
				Where("analysis_id IN ?", analysisIDs).
				Delete(&model.CrmCustomerIntentFeedback{})
			if deletedFeedback.Error != nil {
				return deletedFeedback.Error
			}
			report.DeletedFeedback += deletedFeedback.RowsAffected

			deletedAnalyses := tx.Unscoped().
				Where("id IN ?", analysisIDs).
				Delete(&model.CrmCustomerIntentAnalysis{})
			if deletedAnalyses.Error != nil {
				return deletedAnalyses.Error
			}
			report.DeletedAnalyses += deletedAnalyses.RowsAffected
		}

		deletedFeedback := tx.Unscoped().
			Where("created_at < ?", cutoff).
			Delete(&model.CrmCustomerIntentFeedback{})
		if deletedFeedback.Error != nil {
			return deletedFeedback.Error
		}
		report.DeletedFeedback += deletedFeedback.RowsAffected

		var taskIDs []uint64
		if err := tx.Unscoped().
			Model(&model.CrmCustomerIntentTask{}).
			Where("created_at < ? AND status IN ?", cutoff, []string{"success", "failed", "canceled"}).
			Pluck("id", &taskIDs).Error; err != nil {
			return err
		}
		if len(taskIDs) > 0 {
			deletedItems := tx.Unscoped().
				Where("task_id IN ?", taskIDs).
				Delete(&model.CrmCustomerIntentTaskItem{})
			if deletedItems.Error != nil {
				return deletedItems.Error
			}
			report.DeletedTaskItems = deletedItems.RowsAffected

			deletedTasks := tx.Unscoped().
				Where("id IN ?", taskIDs).
				Delete(&model.CrmCustomerIntentTask{})
			if deletedTasks.Error != nil {
				return deletedTasks.Error
			}
			report.DeletedTasks = deletedTasks.RowsAffected
		}
		return nil
	})
	return report, err
}

// DeleteCustomerIntentData physically removes AI records when a customer is
// deleted. This is intentionally stronger than the normal soft-delete path:
// the customer-owned snapshots must not remain recoverable after deletion.
func DeleteCustomerIntentData(ctx context.Context, db *gorm.DB, tenantID, customerID uint64) error {
	if db == nil {
		return errors.New("retention: database is nil")
	}
	if customerID == 0 {
		return errors.New("retention: customer is required")
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var taskIDs []uint64
		if err := tx.Unscoped().
			Model(&model.CrmCustomerIntentTaskItem{}).
			Where("tenant_id = ? AND customer_id = ?", tenantID, customerID).
			Pluck("task_id", &taskIDs).Error; err != nil {
			return err
		}

		for _, item := range []interface{}{
			&model.CrmCustomerIntentFeedback{},
			&model.CrmCustomerIntent{},
			&model.CrmCustomerIntentAnalysis{},
			&model.CrmCustomerIntentTaskItem{},
		} {
			if err := tx.Unscoped().
				Where("tenant_id = ? AND customer_id = ?", tenantID, customerID).
				Delete(item).Error; err != nil {
				return err
			}
		}

		for _, taskID := range taskIDs {
			if err := recountTask(tx, taskID); err != nil {
				return err
			}
		}
		return nil
	})
}

func recountTask(tx *gorm.DB, taskID uint64) error {
	type counts struct {
		Total    int64
		Pending  int64
		Running  int64
		Success  int64
		Failed   int64
		Canceled int64
	}
	var count counts
	if err := tx.Unscoped().
		Model(&model.CrmCustomerIntentTaskItem{}).
		Select(`COUNT(*) AS total,
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) AS pending,
			COALESCE(SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END), 0) AS running,
			COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) AS success,
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) AS failed,
			COALESCE(SUM(CASE WHEN status = 'canceled' THEN 1 ELSE 0 END), 0) AS canceled`).
		Where("task_id = ?", taskID).
		Scan(&count).Error; err != nil {
		return err
	}
	if count.Total == 0 {
		return tx.Unscoped().Delete(&model.CrmCustomerIntentTask{}, taskID).Error
	}

	status := "pending"
	switch {
	case count.Running > 0:
		status = "running"
	case count.Pending > 0:
		status = "pending"
	case count.Failed > 0:
		status = "failed"
	case count.Canceled == count.Total:
		status = "canceled"
	default:
		status = "success"
	}
	return tx.Unscoped().Model(&model.CrmCustomerIntentTask{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":         status,
			"total_count":    count.Total,
			"pending_count":  count.Pending,
			"running_count":  count.Running,
			"success_count":  count.Success,
			"failed_count":   count.Failed,
			"canceled_count": count.Canceled,
		}).Error
}

// StartIntentCleanup runs once at startup and then daily. RetentionDays <= 0
// leaves the feature disabled, preserving the development default.
func StartIntentCleanup(ctx context.Context, db *gorm.DB, retentionDays int) {
	if retentionDays <= 0 || db == nil {
		return
	}
	cleanup := func() {
		cutoff := time.Now().AddDate(0, 0, -retentionDays)
		report, err := PurgeIntentData(ctx, db, cutoff)
		if err != nil {
			log.Printf("intent retention cleanup failed: %v", err)
			return
		}
		if report.RedactedAnalyses > 0 || report.DeletedAnalyses > 0 ||
			report.DeletedFeedback > 0 || report.DeletedTaskItems > 0 ||
			report.DeletedTasks > 0 {
			log.Printf("intent retention cleanup completed cutoff=%s redacted_analyses=%d deleted_analyses=%d deleted_feedback=%d deleted_task_items=%d deleted_tasks=%d",
				cutoff.Format(time.RFC3339), report.RedactedAnalyses, report.DeletedAnalyses,
				report.DeletedFeedback, report.DeletedTaskItems, report.DeletedTasks)
		}
	}

	go func() {
		cleanup()
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanup()
			}
		}
	}()
}
