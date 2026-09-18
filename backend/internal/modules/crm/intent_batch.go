package crm

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

var ErrNoBatchCustomers = errors.New("no eligible customers for batch analysis")
var ErrBatchTenantRequired = errors.New("tenantId is required for privileged batch analysis")

type IntentBatchRequest struct {
	TenantID       uint64   `json:"tenantId"`
	CustomerIDs    []uint64 `json:"customerIds"`
	IntentLevel    string   `json:"intentLevel"`
	MinScore       *int     `json:"minScore"`
	MaxScore       *int     `json:"maxScore"`
	FollowUpStatus string   `json:"followUpStatus"`
	Limit          int      `json:"limit"`
}

type IntentTaskResponse struct {
	ID            uint64                   `json:"id"`
	TenantID      uint64                   `json:"tenantId"`
	CreatedBy     uint64                   `json:"createdBy"`
	Status        string                   `json:"status"`
	TotalCount    int                      `json:"totalCount"`
	PendingCount  int                      `json:"pendingCount"`
	RunningCount  int                      `json:"runningCount"`
	SuccessCount  int                      `json:"successCount"`
	FailedCount   int                      `json:"failedCount"`
	CanceledCount int                      `json:"canceledCount"`
	MaxAttempts   int                      `json:"maxAttempts"`
	ErrorMessage  string                   `json:"errorMessage"`
	StartedAt     *time.Time               `json:"startedAt"`
	FinishedAt    *time.Time               `json:"finishedAt"`
	CreatedAt     time.Time                `json:"createdAt"`
	Items         []IntentTaskItemResponse `json:"items,omitempty"`
}

type IntentTaskItemResponse struct {
	ID           uint64     `json:"id"`
	TaskID       uint64     `json:"taskId"`
	CustomerID   uint64     `json:"customerId"`
	Status       string     `json:"status"`
	Attempts     int        `json:"attempts"`
	ErrorMessage string     `json:"errorMessage"`
	StartedAt    *time.Time `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
}

// @Summary  批量提交客户AI意向分析
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:batch；单次最多50个客户
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/intent/batch [post]
func (h *IntentHandler) Batch(c *gin.Context) {
	if !h.enabled || h.provider == nil || h.taskEnqueuer == nil {
		common.Fail(c, common.CodeAIUnavailable)
		return
	}
	var req IntentBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	tenantID, ids, err := h.selectBatchCustomers(c, req)
	if err != nil {
		if errors.Is(err, ErrNoBatchCustomers) {
			common.FailMsg(c, common.CodeParamInvalid, "没有符合条件且未在分析中的客户")
		} else if errors.Is(err, ErrBatchTenantRequired) {
			common.FailMsg(c, common.CodeParamInvalid, "平台用户批量分析必须指定租户")
		} else {
			common.Fail(c, common.CodeParamInvalid)
		}
		return
	}
	// Stop creating new batch tasks once the tenant's daily ceiling would be
	// crossed by this submission.
	if !h.checkQuota(c, tenantID, int64(len(ids))) {
		return
	}
	now := time.Now()
	task := model.CrmCustomerIntentTask{
		TenantID:     tenantID,
		CreatedBy:    middleware.CurrentUserID(c),
		Status:       intentTaskStatusPending,
		TotalCount:   len(ids),
		PendingCount: len(ids),
		MaxAttempts:  3,
		StartedAt:    &now,
	}
	items := make([]model.CrmCustomerIntentTaskItem, 0, len(ids))
	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		for _, customerID := range ids {
			item := model.CrmCustomerIntentTaskItem{
				TaskID:        task.ID,
				TenantID:      tenantID,
				CustomerID:    customerID,
				TriggerUserID: middleware.CurrentUserID(c),
				Status:        intentTaskItemStatusPending,
			}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
			items = append(items, item)
		}
		return nil
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	for _, item := range items {
		if err := h.taskEnqueuer(c.Request.Context(), IntentTask{
			TenantID:      tenantID,
			CustomerID:    item.CustomerID,
			TriggerUserID: item.TriggerUserID,
			TaskID:        task.ID,
			ItemID:        item.ID,
		}); err != nil {
			_ = h.markTaskItemFailed(item.ID, err.Error())
		}
	}
	var stored model.CrmCustomerIntentTask
	if err := h.db.First(&stored, task.ID).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, intentTaskResponse(&stored, nil))
}

// @Summary  查询批量AI意向任务进度
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:batch
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/intent/tasks/{id} [get]
func (h *IntentHandler) Task(c *gin.Context) {
	task, ok := h.findTask(c)
	if !ok {
		return
	}
	var items []model.CrmCustomerIntentTaskItem
	if err := h.db.Where("task_id = ?", task.ID).Order("id ASC").Limit(500).Find(&items).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, intentTaskResponse(task, items))
}

// @Summary  取消尚未开始的批量AI意向任务
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:batch
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/intent/tasks/{id}/cancel [post]
func (h *IntentHandler) CancelTask(c *gin.Context) {
	task, ok := h.findTask(c)
	if !ok {
		return
	}
	if task.Status == intentTaskStatusSuccess || task.Status == intentTaskStatusFailed ||
		task.Status == intentTaskStatusCanceled {
		common.FailMsg(c, common.CodeParamInvalid, "任务已经结束")
		return
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.CrmCustomerIntentTaskItem{}).
			Where("task_id = ? AND status = ?", task.ID, intentTaskItemStatusPending).
			Updates(map[string]interface{}{
				"status":      intentTaskItemStatusCanceled,
				"finished_at": time.Now(),
			}).Error; err != nil {
			return err
		}
		return updateIntentTaskCounters(tx, task.ID)
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	h.db.First(&task, task.ID)
	common.OK(c, intentTaskResponse(task, nil))
}

func (h *IntentHandler) findTask(c *gin.Context) (*model.CrmCustomerIntentTask, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return nil, false
	}
	query := h.db
	if !middleware.IsPrivileged(c) {
		query = query.Where("tenant_id = ?", middleware.CurrentTenantID(c))
	}
	var task model.CrmCustomerIntentTask
	if err := query.First(&task, id).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return nil, false
	}
	return &task, true
}

func (h *IntentHandler) selectBatchCustomers(c *gin.Context, req IntentBatchRequest) (uint64, []uint64, error) {
	tenantID := middleware.CurrentTenantID(c)
	if middleware.IsPrivileged(c) {
		if req.TenantID == 0 {
			return 0, nil, ErrBatchTenantRequired
		}
		tenantID = req.TenantID
	}
	limit := req.Limit
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	query := h.db.Model(&model.CrmCustomer{}).
		Select("crm_customer.id").
		Where("crm_customer.tenant_id = ?", tenantID)
	if len(req.CustomerIDs) > 0 {
		query = query.Where("crm_customer.id IN ?", req.CustomerIDs)
	}
	if req.IntentLevel != "" || req.MinScore != nil || req.MaxScore != nil || req.FollowUpStatus != "" {
		query = query.Joins(`LEFT JOIN crm_customer_intent ci
			ON ci.tenant_id = crm_customer.tenant_id
			AND ci.customer_id = crm_customer.id
			AND ci.deleted_at IS NULL`)
	}
	if req.IntentLevel != "" {
		if req.IntentLevel == "none" {
			query = query.Where("ci.id IS NULL")
		} else {
			query = query.Where(effectiveIntentLevelSQL("ci")+" = ?", req.IntentLevel)
		}
	}
	if req.MinScore != nil {
		query = query.Where("ci.intent_score >= ?", *req.MinScore)
	}
	if req.MaxScore != nil {
		query = query.Where("ci.intent_score <= ?", *req.MaxScore)
	}
	if req.FollowUpStatus != "" {
		now := time.Now()
		switch req.FollowUpStatus {
		case "today":
			start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			query = query.Where("ci.suggested_next_at >= ? AND ci.suggested_next_at < ?", start, start.AddDate(0, 0, 1))
		case "overdue":
			query = query.Where("ci.suggested_next_at < ?", now)
		case "future":
			query = query.Where("ci.suggested_next_at >= ?", now)
		case "none":
			query = query.Where("ci.suggested_next_at IS NULL")
		}
	}
	var ids []uint64
	if err := query.Order("crm_customer.id DESC").Limit(limit).Pluck("crm_customer.id", &ids).Error; err != nil {
		return 0, nil, err
	}
	if len(ids) == 0 {
		return 0, nil, ErrNoBatchCustomers
	}
	var active []uint64
	if err := h.db.Model(&model.CrmCustomerIntentTaskItem{}).
		Where("tenant_id = ? AND customer_id IN ? AND status IN ?",
			tenantID, ids, []string{intentTaskItemStatusPending, intentTaskItemStatusRunning}).
		Pluck("customer_id", &active).Error; err != nil {
		return 0, nil, err
	}
	activeSet := make(map[uint64]struct{}, len(active))
	for _, id := range active {
		activeSet[id] = struct{}{}
	}
	eligible := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if _, exists := activeSet[id]; !exists {
			eligible = append(eligible, id)
		}
	}
	if len(eligible) == 0 {
		return 0, nil, ErrNoBatchCustomers
	}
	return tenantID, eligible, nil
}

func intentTaskResponse(task *model.CrmCustomerIntentTask, items []model.CrmCustomerIntentTaskItem) IntentTaskResponse {
	response := IntentTaskResponse{
		ID:            task.ID,
		TenantID:      task.TenantID,
		CreatedBy:     task.CreatedBy,
		Status:        task.Status,
		TotalCount:    task.TotalCount,
		PendingCount:  task.PendingCount,
		RunningCount:  task.RunningCount,
		SuccessCount:  task.SuccessCount,
		FailedCount:   task.FailedCount,
		CanceledCount: task.CanceledCount,
		MaxAttempts:   task.MaxAttempts,
		ErrorMessage:  task.ErrorMessage,
		StartedAt:     task.StartedAt,
		FinishedAt:    task.FinishedAt,
		CreatedAt:     task.CreatedAt,
	}
	if len(items) > 0 {
		response.Items = make([]IntentTaskItemResponse, 0, len(items))
		for _, item := range items {
			response.Items = append(response.Items, IntentTaskItemResponse{
				ID:           item.ID,
				TaskID:       item.TaskID,
				CustomerID:   item.CustomerID,
				Status:       item.Status,
				Attempts:     item.Attempts,
				ErrorMessage: item.ErrorMessage,
				StartedAt:    item.StartedAt,
				FinishedAt:   item.FinishedAt,
			})
		}
	}
	return response
}
