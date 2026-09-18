package crm

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

const (
	intentTaskStatusPending  = "pending"
	intentTaskStatusRunning  = "running"
	intentTaskStatusSuccess  = "success"
	intentTaskStatusFailed   = "failed"
	intentTaskStatusCanceled = "canceled"

	intentTaskItemStatusPending  = "pending"
	intentTaskItemStatusRunning  = "running"
	intentTaskItemStatusSuccess  = "success"
	intentTaskItemStatusFailed   = "failed"
	intentTaskItemStatusCanceled = "canceled"
)

func (h *IntentHandler) processQueuedTask(ctx context.Context, task IntentTask) error {
	if task.ItemID > 0 {
		var item model.CrmCustomerIntentTaskItem
		if err := h.db.First(&item, task.ItemID).Error; err != nil {
			return err
		}
		if item.Status == intentTaskItemStatusCanceled ||
			item.Status == intentTaskItemStatusSuccess {
			return nil
		}
		if err := h.markTaskItemRunning(task.ItemID, task.Attempts+1); err != nil {
			return err
		}
	}
	err := h.AnalyzeCustomerByID(ctx, task.TenantID, task.CustomerID, task.TriggerUserID)
	if task.ItemID == 0 {
		return err
	}
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			_ = h.markTaskItemPending(task.ItemID)
			return err
		}
		_ = h.markTaskItemFailed(task.ItemID, err.Error())
		return err
	}
	_ = h.markTaskItemSuccess(task.ItemID)
	return nil
}

func (h *IntentHandler) markTaskItemRunning(itemID uint64, attempts int) error {
	now := time.Now()
	return h.db.Transaction(func(tx *gorm.DB) error {
		var item model.CrmCustomerIntentTaskItem
		if err := tx.First(&item, itemID).Error; err != nil {
			return err
		}
		item.Status = intentTaskItemStatusRunning
		item.Attempts = attempts
		item.StartedAt = &now
		item.ErrorMessage = ""
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		return updateIntentTaskCounters(tx, item.TaskID)
	})
}

func (h *IntentHandler) markTaskItemPending(itemID uint64) error {
	return h.db.Transaction(func(tx *gorm.DB) error {
		var item model.CrmCustomerIntentTaskItem
		if err := tx.First(&item, itemID).Error; err != nil {
			return err
		}
		item.Status = intentTaskItemStatusPending
		item.FinishedAt = nil
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		return updateIntentTaskCounters(tx, item.TaskID)
	})
}

func (h *IntentHandler) markTaskItemSuccess(itemID uint64) error {
	now := time.Now()
	return h.db.Transaction(func(tx *gorm.DB) error {
		var item model.CrmCustomerIntentTaskItem
		if err := tx.First(&item, itemID).Error; err != nil {
			return err
		}
		item.Status = intentTaskItemStatusSuccess
		item.FinishedAt = &now
		item.ErrorMessage = ""
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		return updateIntentTaskCounters(tx, item.TaskID)
	})
}

func (h *IntentHandler) markTaskItemFailed(itemID uint64, message string) error {
	now := time.Now()
	return h.db.Transaction(func(tx *gorm.DB) error {
		var item model.CrmCustomerIntentTaskItem
		if err := tx.First(&item, itemID).Error; err != nil {
			return err
		}
		item.Status = intentTaskItemStatusFailed
		item.FinishedAt = &now
		item.ErrorMessage = truncateIntentError(message, 1024)
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		return updateIntentTaskCounters(tx, item.TaskID)
	})
}

func updateIntentTaskCounters(tx *gorm.DB, taskID uint64) error {
	type counts struct {
		Total    int64
		Pending  int64
		Running  int64
		Success  int64
		Failed   int64
		Canceled int64
	}
	var count counts
	if err := tx.Model(&model.CrmCustomerIntentTaskItem{}).
		Select(`COUNT(*) AS total,
			SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) AS pending,
			SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END) AS running,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) AS success,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) AS failed,
			SUM(CASE WHEN status = 'canceled' THEN 1 ELSE 0 END) AS canceled`).
		Where("task_id = ?", taskID).Scan(&count).Error; err != nil {
		return err
	}
	status := intentTaskStatusRunning
	switch {
	case count.Total == 0:
		status = intentTaskStatusFailed
	case count.Pending+count.Running > 0:
		status = intentTaskStatusRunning
	case count.Success == count.Total:
		status = intentTaskStatusSuccess
	case count.Canceled == count.Total:
		status = intentTaskStatusCanceled
	default:
		status = intentTaskStatusFailed
	}
	updates := map[string]interface{}{
		"status":         status,
		"total_count":    count.Total,
		"pending_count":  count.Pending,
		"running_count":  count.Running,
		"success_count":  count.Success,
		"failed_count":   count.Failed,
		"canceled_count": count.Canceled,
	}
	if status == intentTaskStatusSuccess || status == intentTaskStatusFailed || status == intentTaskStatusCanceled {
		now := time.Now()
		updates["finished_at"] = &now
	}
	return tx.Model(&model.CrmCustomerIntentTask{}).Where("id = ?", taskID).Updates(updates).Error
}
