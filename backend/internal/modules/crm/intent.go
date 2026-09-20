package crm

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	mysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/ai"
	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
	"github.com/lihefengbj/nanyicrm/backend/internal/quota"
)

const (
	intentStatusRunning = "running"
	intentStatusSuccess = "success"
	intentStatusFailed  = "failed"
)

var ErrIntentAnalysisUnavailable = errors.New("intent analysis unavailable")

const intentInputVersion = "v1"

var (
	intentEmailPattern = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	intentPhonePattern = regexp.MustCompile(`\b1[3-9][0-9]{9}\b`)
	intentIDPattern    = regexp.MustCompile(`\b[0-9]{17}[0-9Xx]\b`)
)

type intentAnalysisError struct {
	errorType string
	retryable bool
	cause     error
}

func (e *intentAnalysisError) Error() string {
	if e.cause == nil {
		return ErrIntentAnalysisUnavailable.Error()
	}
	return e.cause.Error()
}

func (e *intentAnalysisError) Unwrap() error {
	return e.cause
}

func (e *intentAnalysisError) Is(target error) bool {
	return target == ErrIntentAnalysisUnavailable
}

func isRetryableIntentError(err error) bool {
	var analysisErr *intentAnalysisError
	if errors.As(err, &analysisErr) {
		return analysisErr.retryable
	}
	// Database and other infrastructure errors are treated as transient unless
	// they have been explicitly classified by the provider layer.
	return true
}

func logIntentAnalysisError(stage string, customer *model.CrmCustomer, err error) {
	if customer == nil {
		log.Printf("intent analysis failed stage=%s: %v", stage, err)
		return
	}
	log.Printf("intent analysis failed stage=%s tenant=%d customer=%d: %v",
		stage, customer.TenantID, customer.ID, err)
}

const intentHistoryWriteAttempts = 3

func isInvalidDatabaseConnection(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, driver.ErrBadConn) || errors.Is(err, mysql.ErrInvalidConn) {
		return true
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "invalid connection") ||
		strings.Contains(message, "connection is already closed") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "connection reset by peer")
}

type IntentHandler struct {
	db           *gorm.DB
	enabled      bool
	provider     ai.Provider
	taskEnqueuer func(context.Context, IntentTask) error
	quota        *quota.Service
	pricing      config.PricingConfig
}

func NewIntentHandler(db *gorm.DB, enabled bool, provider ai.Provider, quotaSvc *quota.Service) *IntentHandler {
	return &IntentHandler{db: db, enabled: enabled, provider: provider, quota: quotaSvc}
}

func (h *IntentHandler) SetTaskEnqueuer(enqueuer func(context.Context, IntentTask) error) {
	h.taskEnqueuer = enqueuer
}

func (h *IntentHandler) SetPricing(pricing config.PricingConfig) {
	h.pricing = pricing
}

// checkQuota rejects creating new analysis tasks when the tenant already met
// a daily or concurrency ceiling. It writes the response and reports whether
// the submission may proceed.
func (h *IntentHandler) checkQuota(c *gin.Context, tenantID uint64, extraCalls int64) bool {
	if h.quota == nil {
		return true
	}
	if err := h.quota.CheckSubmission(c.Request.Context(), tenantID, extraCalls); err != nil {
		var exceeded *quota.ExceededError
		if errors.As(err, &exceeded) {
			common.FailMsg(c, common.CodeQuotaExceeded, err.Error())
		} else {
			common.Fail(c, common.CodeInternalError)
		}
		return false
	}
	return true
}

// failIntentError maps an analysis error to an HTTP response.
func failIntentError(c *gin.Context, err error) {
	var exceeded *quota.ExceededError
	if errors.As(err, &exceeded) {
		common.FailMsg(c, common.CodeQuotaExceeded, err.Error())
		return
	}
	if errors.Is(err, ErrIntentAnalysisUnavailable) {
		common.Fail(c, common.CodeAIUnavailable)
		return
	}
	common.Fail(c, common.CodeDBError)
}

type IntentResponse struct {
	ID                   uint64     `json:"id"`
	TenantID             uint64     `json:"tenantId"`
	CustomerID           uint64     `json:"customerId"`
	AnalysisID           uint64     `json:"analysisId"`
	IntentLevel          string     `json:"intentLevel"`
	EffectiveIntentLevel string     `json:"effectiveIntentLevel"`
	IntentScore          *int       `json:"intentScore"`
	Confidence           *float64   `json:"confidence"`
	Summary              string     `json:"summary"`
	Needs                []string   `json:"needs"`
	PainPoints           []string   `json:"painPoints"`
	Budget               string     `json:"budget"`
	PurchaseTimeline     string     `json:"purchaseTimeline"`
	DecisionRole         string     `json:"decisionRole"`
	Risks                []string   `json:"risks"`
	NextAction           string     `json:"nextAction"`
	SuggestedNextAt      *time.Time `json:"suggestedNextAt"`
	AnalyzedAt           time.Time  `json:"analyzedAt"`
	Provider             string     `json:"provider"`
	Model                string     `json:"model"`
	PromptVersion        string     `json:"promptVersion"`
	Status               string     `json:"status"`
	ManualOverride       bool       `json:"manualOverride"`
	ManualIntentLevel    string     `json:"manualIntentLevel"`
	FollowUpStatus       string     `json:"followUpStatus"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

type IntentListResponse struct {
	ID                   uint64     `json:"id"`
	CustomerID           uint64     `json:"customerId"`
	AnalysisID           uint64     `json:"analysisId"`
	IntentLevel          string     `json:"intentLevel"`
	EffectiveIntentLevel string     `json:"effectiveIntentLevel"`
	IntentScore          *int       `json:"intentScore"`
	Confidence           *float64   `json:"confidence"`
	Summary              string     `json:"summary"`
	NextAction           string     `json:"nextAction"`
	SuggestedNextAt      *time.Time `json:"suggestedNextAt"`
	AnalyzedAt           time.Time  `json:"analyzedAt"`
	Provider             string     `json:"provider"`
	Model                string     `json:"model"`
	Status               string     `json:"status"`
	ManualOverride       bool       `json:"manualOverride"`
	ManualIntentLevel    string     `json:"manualIntentLevel"`
	FollowUpStatus       string     `json:"followUpStatus"`
}

type IntentHistoryResponse struct {
	ID                 uint64          `json:"id"`
	CustomerID         uint64          `json:"customerId"`
	TriggerUserID      uint64          `json:"triggerUserId"`
	Result             *IntentResponse `json:"result"`
	Status             string          `json:"status"`
	ErrorMessage       string          `json:"errorMessage"`
	Provider           string          `json:"provider"`
	Model              string          `json:"model"`
	ActualModel        string          `json:"actualModel"`
	ModelConfigVersion string          `json:"modelConfigVersion"`
	AdapterVersion     string          `json:"adapterVersion"`
	PromptVersion      string          `json:"promptVersion"`
	InputTokens        int             `json:"inputTokens"`
	OutputTokens       int             `json:"outputTokens"`
	TotalTokens        int             `json:"totalTokens"`
	ProviderRequestID  string          `json:"providerRequestId"`
	ErrorType          string          `json:"errorType"`
	InputHash          string          `json:"inputHash"`
	CostMillis         int64           `json:"costMillis"`
	AnalyzedAt         *time.Time      `json:"analyzedAt"`
	InputSummary       string          `json:"inputSummary"`
	CreatedAt          time.Time       `json:"createdAt"`
}

type IntentFeedbackRequest struct {
	AnalysisID        uint64 `json:"analysisId"`
	FeedbackType      string `json:"feedbackType" binding:"required,oneof=accurate partial inaccurate"`
	Accepted          *bool  `json:"accepted"`
	ManualIntentLevel string `json:"manualIntentLevel" binding:"omitempty,oneof=high medium low unknown"`
	Note              string `json:"note" binding:"max=1024"`
}

type IntentFeedbackResponse struct {
	ID                uint64     `json:"id"`
	CustomerID        uint64     `json:"customerId"`
	AnalysisID        uint64     `json:"analysisId"`
	UserID            uint64     `json:"userId"`
	UserName          string     `json:"userName"`
	FeedbackType      string     `json:"feedbackType"`
	Accepted          *bool      `json:"accepted"`
	ManualIntentLevel string     `json:"manualIntentLevel"`
	Note              string     `json:"note"`
	AnalysisAt        *time.Time `json:"analysisAt"`
	CreatedAt         time.Time  `json:"createdAt"`
}

type IntentCompareResponse struct {
	Current      *IntentHistoryResponse `json:"current"`
	Previous     *IntentHistoryResponse `json:"previous"`
	ScoreDiff    *int                   `json:"scoreDiff"`
	LevelChanged bool                   `json:"levelChanged"`
}

// @Summary  查询客户当前AI意向
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/{id}/intent [get]
func (h *IntentHandler) Current(c *gin.Context) {
	customer, ok := h.customer(c)
	if !ok {
		return
	}

	var intent model.CrmCustomerIntent
	err := h.db.Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).First(&intent).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		common.OK(c, nil)
		return
	}
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, h.toResponse(&intent))
}

// @Summary  查询客户AI意向分析历史
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:history
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/{id}/intent/history [get]
func (h *IntentHandler) History(c *gin.Context) {
	customer, ok := h.customer(c)
	if !ok {
		return
	}
	page := common.ParsePageQuery(c)
	query := h.db.Model(&model.CrmCustomerIntentAnalysis{}).
		Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID)
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		switch status {
		case intentStatusRunning, intentStatusSuccess, intentStatusFailed:
			query = query.Where("status = ?", status)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var rows []model.CrmCustomerIntentAnalysis
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.PageSize).Find(&rows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	result := make([]IntentHistoryResponse, 0, len(rows))
	for i := range rows {
		result = append(result, h.historyResponse(&rows[i]))
	}
	common.OKPage(c, result, total, page.PageNum, page.PageSize)
}

// @Summary  手动分析客户意向
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:analyze
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/{id}/intent/analyze [post]
func (h *IntentHandler) Analyze(c *gin.Context) {
	customer, ok := h.customer(c)
	if !ok {
		return
	}
	if !h.checkQuota(c, customer.TenantID, 1) {
		return
	}
	intent, err := h.analyzeCustomer(c.Request.Context(), customer, middleware.CurrentUserID(c))
	if err != nil {
		logIntentAnalysisError("manual", customer, err)
		failIntentError(c, err)
		return
	}
	common.OK(c, h.toResponse(intent))
}

// @Summary  重试客户最近一次AI意向分析
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:analyze
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/{id}/intent/retry [post]
func (h *IntentHandler) Retry(c *gin.Context) {
	customer, ok := h.customer(c)
	if !ok {
		return
	}
	if !h.checkQuota(c, customer.TenantID, 1) {
		return
	}
	intent, err := h.analyzeCustomer(c.Request.Context(), customer, middleware.CurrentUserID(c))
	if err != nil {
		logIntentAnalysisError("retry", customer, err)
		failIntentError(c, err)
		return
	}
	common.OK(c, h.toResponse(intent))
}

// @Summary  提交客户AI意向反馈
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:feedback
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/{id}/intent/feedback [post]
func (h *IntentHandler) Feedback(c *gin.Context) {
	customer, ok := h.customer(c)
	if !ok {
		return
	}
	var req IntentFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	var analysis model.CrmCustomerIntentAnalysis
	query := h.db.Where("tenant_id = ? AND customer_id = ? AND status = ?",
		customer.TenantID, customer.ID, intentStatusSuccess)
	if req.AnalysisID > 0 {
		query = query.Where("id = ?", req.AnalysisID)
	} else {
		query = query.Order("id DESC")
	}
	if err := query.First(&analysis).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return
	}
	feedback := model.CrmCustomerIntentFeedback{
		TenantID:          customer.TenantID,
		CustomerID:        customer.ID,
		AnalysisID:        analysis.ID,
		UserID:            middleware.CurrentUserID(c),
		FeedbackType:      req.FeedbackType,
		Accepted:          req.Accepted,
		ManualIntentLevel: req.ManualIntentLevel,
		Note:              strings.TrimSpace(req.Note),
	}
	appliedToCurrent := false
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&feedback).Error; err != nil {
			return err
		}
		if req.ManualIntentLevel == "" {
			return nil
		}
		var current model.CrmCustomerIntent
		if err := tx.Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).
			First(&current).Error; err != nil {
			return err
		}
		if !isCurrentIntentAnalysis(&current, &analysis) {
			return nil
		}
		update := tx.Model(&model.CrmCustomerIntent{}).
			Where("id = ? AND tenant_id = ? AND customer_id = ?", current.ID, customer.TenantID, customer.ID)
		if current.AnalysisID == 0 {
			update = update.Where("analysis_id = 0 AND analyzed_at = ?", current.AnalyzedAt)
		} else {
			update = update.Where("analysis_id = ?", analysis.ID)
		}
		values := map[string]interface{}{
			"manual_override":     true,
			"manual_intent_level": req.ManualIntentLevel,
		}
		if current.AnalysisID == 0 {
			values["analysis_id"] = analysis.ID
		}
		result := update.Updates(values)
		if result.Error != nil {
			return result.Error
		}
		appliedToCurrent = result.RowsAffected > 0
		return nil
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": feedback.ID, "appliedToCurrent": appliedToCurrent})
}

// @Summary  查询客户AI意向反馈历史
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:feedback 或 crm:intent:feedback:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/{id}/intent/feedback [get]
func (h *IntentHandler) FeedbackHistory(c *gin.Context) {
	customer, ok := h.customer(c)
	if !ok {
		return
	}
	page := common.ParsePageQuery(c)
	type feedbackRow struct {
		model.CrmCustomerIntentFeedback
		UserName   string     `gorm:"column:user_name"`
		AnalysisAt *time.Time `gorm:"column:analysis_at"`
	}
	query := h.db.Table("crm_customer_intent_feedback AS f").
		Where("f.tenant_id = ? AND f.customer_id = ? AND f.deleted_at IS NULL", customer.TenantID, customer.ID)
	if value := strings.TrimSpace(c.Query("analysisId")); value != "" {
		analysisID, err := strconv.ParseUint(value, 10, 64)
		if err != nil || analysisID == 0 {
			common.Fail(c, common.CodeParamInvalid)
			return
		}
		query = query.Where("f.analysis_id = ?", analysisID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var rows []feedbackRow
	if err := query.
		Select(`f.*, COALESCE(NULLIF(u.nickname, ''), u.username, '') AS user_name, a.analyzed_at AS analysis_at`).
		Joins("LEFT JOIN sys_user AS u ON u.id = f.user_id").
		Joins(`LEFT JOIN crm_customer_intent_analysis AS a
			ON a.id = f.analysis_id
			AND a.tenant_id = f.tenant_id
			AND a.customer_id = f.customer_id
			AND a.deleted_at IS NULL`).
		Order("f.id DESC").Offset(page.Offset()).Limit(page.PageSize).
		Scan(&rows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	result := make([]IntentFeedbackResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, IntentFeedbackResponse{
			ID:                row.ID,
			CustomerID:        row.CustomerID,
			AnalysisID:        row.AnalysisID,
			UserID:            row.UserID,
			UserName:          row.UserName,
			FeedbackType:      row.FeedbackType,
			Accepted:          row.Accepted,
			ManualIntentLevel: row.ManualIntentLevel,
			Note:              row.Note,
			AnalysisAt:        row.AnalysisAt,
			CreatedAt:         row.CreatedAt,
		})
	}
	common.OKPage(c, result, total, page.PageNum, page.PageSize)
}

// @Summary  查询客户AI意向结果对比
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:compare
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/{id}/intent/compare [get]
func (h *IntentHandler) Compare(c *gin.Context) {
	customer, ok := h.customer(c)
	if !ok {
		return
	}
	var rows []model.CrmCustomerIntentAnalysis
	if err := h.db.Where("tenant_id = ? AND customer_id = ? AND status = ?",
		customer.TenantID, customer.ID, intentStatusSuccess).
		Order("id DESC").Limit(2).Find(&rows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	response := IntentCompareResponse{}
	if len(rows) > 0 {
		current := h.historyResponse(&rows[0])
		response.Current = &current
	}
	if len(rows) > 1 {
		previous := h.historyResponse(&rows[1])
		response.Previous = &previous
		if response.Current != nil && response.Previous != nil &&
			response.Current.Result != nil && response.Previous.Result != nil {
			if response.Current.Result.IntentScore != nil && response.Previous.Result.IntentScore != nil {
				diff := *response.Current.Result.IntentScore - *response.Previous.Result.IntentScore
				response.ScoreDiff = &diff
			}
			response.LevelChanged = response.Current.Result.IntentLevel != response.Previous.Result.IntentLevel
		}
	}
	common.OK(c, response)
}

// AnalyzeCustomerByID is used by background workers after a follow-up is
// saved. It intentionally receives tenantID so a queued task cannot cross
// tenant boundaries if a customer ID is reused by an upstream caller.
func (h *IntentHandler) AnalyzeCustomerByID(ctx context.Context, tenantID, customerID, triggerUserID uint64) error {
	if !h.enabled || h.provider == nil {
		return ErrIntentAnalysisUnavailable
	}
	var customer model.CrmCustomer
	if err := h.db.Where("tenant_id = ? AND id = ?", tenantID, customerID).First(&customer).Error; err != nil {
		return err
	}
	_, err := h.analyzeCustomer(ctx, &customer, triggerUserID)
	return err
}

func (h *IntentHandler) analyzeCustomer(ctx context.Context, customer *model.CrmCustomer, triggerUserID uint64) (*model.CrmCustomerIntent, error) {
	if !h.enabled || h.provider == nil {
		return nil, ErrIntentAnalysisUnavailable
	}
	if h.quota != nil {
		// Hard ceiling enforced at execution time so tasks enqueued before the
		// limit was hit cannot overshoot it. The error is non-retryable: quota
		// exhaustion is not transient.
		if err := h.quota.ReserveCall(ctx, customer.TenantID); err != nil {
			var exceeded *quota.ExceededError
			if errors.As(err, &exceeded) {
				return nil, &intentAnalysisError{errorType: ai.ErrorTypeQuotaExceeded, retryable: false, cause: err}
			}
			return nil, err
		}
	}
	input, err := h.buildInput(customer)
	if err != nil {
		return nil, fmt.Errorf("build intent input: %w", err)
	}
	inputSnapshot, _ := json.Marshal(input)
	inputHash := sha256.Sum256(inputSnapshot)
	history := model.CrmCustomerIntentAnalysis{
		TenantID:      customer.TenantID,
		CustomerID:    customer.ID,
		TriggerUserID: triggerUserID,
		InputSnapshot: string(inputSnapshot),
		Status:        intentStatusRunning,
		Provider:      h.provider.Name(),
		Model:         h.provider.Model(),
		PromptVersion: ai.PromptVersion,
		InputHash:     fmt.Sprintf("%x", inputHash),
	}
	if err := h.createIntentAnalysisHistory(ctx, &history); err != nil {
		return nil, fmt.Errorf("create intent analysis history: %w", err)
	}

	start := time.Now()
	history.BillingPeriod = h.pricing.PeriodAt(start)
	analysis, err := h.provider.AnalyzeCustomerIntent(ctx, input)
	history.CostMillis = time.Since(start).Milliseconds()
	if err != nil {
		errorType, retryable := ai.ErrorInfo(err)
		history.Status = intentStatusFailed
		history.ErrorType = errorType
		history.ErrorMessage = truncateIntentError(err.Error(), 1024)
		if saveErr := h.db.Save(&history).Error; saveErr != nil {
			logIntentAnalysisError("save failed provider analysis", customer,
				fmt.Errorf("provider=%w; save history=%v", err, saveErr))
		}
		return nil, &intentAnalysisError{errorType: errorType, retryable: retryable, cause: err}
	}
	if analysis == nil || analysis.Result == nil {
		err := errors.New("provider returned empty analysis result")
		history.Status = intentStatusFailed
		history.ErrorType = ai.ErrorTypeEmptyResponse
		history.ErrorMessage = truncateIntentError(err.Error(), 1024)
		if saveErr := h.db.Save(&history).Error; saveErr != nil {
			logIntentAnalysisError("save empty provider result", customer, saveErr)
		}
		return nil, &intentAnalysisError{errorType: ai.ErrorTypeEmptyResponse, cause: err}
	}
	result := analysis.Result
	history.ActualModel = analysis.Metadata.ActualModel
	if history.ActualModel == "" {
		history.ActualModel = h.provider.Model()
	}
	history.ModelConfigVersion = analysis.Metadata.ConfigVersion
	history.AdapterVersion = analysis.Metadata.AdapterVersion
	history.InputTokens = analysis.Metadata.InputTokens
	history.InputCacheHitTokens = analysis.Metadata.InputCacheHitTokens
	history.InputCacheMissTokens = analysis.Metadata.InputCacheMissTokens
	history.OutputTokens = analysis.Metadata.OutputTokens
	history.TotalTokens = analysis.Metadata.TotalTokens
	history.ProviderRequestID = analysis.Metadata.RequestID
	// Token accounting happens even when result validation fails below: the
	// provider billed the call regardless.
	if h.quota != nil {
		h.quota.AddTokens(ctx, customer.TenantID, int64(analysis.Metadata.TotalTokens))
	}
	if err := validateIntentResult(result); err != nil {
		history.Status = intentStatusFailed
		history.ErrorType = ai.ErrorTypeResultValidation
		history.ErrorMessage = truncateIntentError(err.Error(), 1024)
		if saveErr := h.db.Save(&history).Error; saveErr != nil {
			logIntentAnalysisError("save invalid provider result", customer, saveErr)
		}
		return nil, &intentAnalysisError{errorType: ai.ErrorTypeResultValidation, cause: err}
	}

	resultSnapshot, _ := json.Marshal(result)
	analyzedAt := time.Now()
	history.Status = intentStatusSuccess
	history.ResultSnapshot = string(resultSnapshot)
	history.AnalyzedAt = &analyzedAt
	intent := h.toModel(customer, result, analyzedAt, history.ID)
	promoted := false
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&history).Error; err != nil {
			return err
		}
		// Analysis IDs are created before the provider call, so the largest ID
		// represents the latest triggered analysis rather than the one that
		// happened to finish last. An older slow request must remain in history
		// without overwriting the current result of a newer request.
		var latestAnalysisID uint64
		if err := tx.Model(&model.CrmCustomerIntentAnalysis{}).
			Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).
			Select("COALESCE(MAX(id), 0)").Scan(&latestAnalysisID).Error; err != nil {
			return err
		}
		if latestAnalysisID != history.ID {
			return nil
		}
		var current model.CrmCustomerIntent
		findErr := tx.Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).First(&current).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			if err := tx.Create(&intent).Error; err != nil {
				return err
			}
			promoted = true
			return nil
		}
		if findErr != nil {
			return findErr
		}
		intent.ID = current.ID
		intent.CreatedAt = current.CreatedAt
		if err := tx.Save(&intent).Error; err != nil {
			return err
		}
		promoted = true
		return nil
	}); err != nil {
		return nil, fmt.Errorf("persist intent result: %w", err)
	}
	if !promoted {
		var current model.CrmCustomerIntent
		if err := h.db.Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).
			First(&current).Error; err == nil {
			return &current, nil
		}
	}
	return &intent, nil
}

func (h *IntentHandler) createIntentAnalysisHistory(ctx context.Context, history *model.CrmCustomerIntentAnalysis) error {
	if history == nil {
		return errors.New("history is nil")
	}
	var lastErr error
	for attempt := 1; attempt <= intentHistoryWriteAttempts; attempt++ {
		if err := h.pingDatabase(ctx); err != nil {
			lastErr = err
		} else {
			result := h.db.WithContext(ctx).Create(history)
			if result.Error == nil {
				return nil
			}
			lastErr = result.Error
			if !isInvalidDatabaseConnection(result.Error) {
				return result.Error
			}
		}

		// An INSERT can be accepted by MySQL and still report a broken
		// connection before the client receives the result. Confirm by the
		// stable input hash before retrying, so a retry cannot duplicate the
		// analysis history.
		if existing, err := h.findIntentAnalysisByInput(ctx, history); err == nil && existing != nil {
			*history = *existing
			return nil
		}
		if attempt == intentHistoryWriteAttempts {
			break
		}
		if err := waitIntentRetry(ctx, attempt); err != nil {
			return err
		}
	}
	return lastErr
}

func (h *IntentHandler) pingDatabase(ctx context.Context) error {
	sqlDB, err := h.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (h *IntentHandler) findIntentAnalysisByInput(ctx context.Context, history *model.CrmCustomerIntentAnalysis) (*model.CrmCustomerIntentAnalysis, error) {
	var existing model.CrmCustomerIntentAnalysis
	err := h.db.WithContext(ctx).
		Where("tenant_id = ? AND customer_id = ? AND input_hash = ? AND status = ?",
			history.TenantID, history.CustomerID, history.InputHash, intentStatusRunning).
		Order("id DESC").
		First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &existing, nil
}

func waitIntentRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(attempt) * 150 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (h *IntentHandler) customer(c *gin.Context) (*model.CrmCustomer, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return nil, false
	}
	return findCustomerInTenant(c, h.db, id)
}

func (h *IntentHandler) buildInput(customer *model.CrmCustomer) (ai.IntentInput, error) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	input := ai.IntentInput{
		Customer: ai.CustomerContext{
			Name:     customer.Name,
			Industry: customer.Industry,
			Source:   customer.Source,
			Level:    customer.Level,
			Status:   customer.Status,
			Remark:   sanitizeIntentText(customer.Remark, 1000),
		},
		ReferenceTime: time.Now().In(location),
		Timezone:      "Asia/Shanghai",
		InputVersion:  intentInputVersion,
	}
	var contacts []model.CrmContact
	if err := h.db.Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).
		Order("is_primary DESC, id DESC").Limit(20).Find(&contacts).Error; err != nil {
		return input, fmt.Errorf("query contacts: %w", err)
	}
	for _, contact := range contacts {
		input.Contacts = append(input.Contacts, ai.ContactContext{Name: contact.Name, Position: contact.Position})
	}

	var follows []model.CrmFollowUp
	if err := h.db.Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).
		Order("id DESC").Limit(10).Find(&follows).Error; err != nil {
		return input, fmt.Errorf("query follow-ups: %w", err)
	}
	for _, follow := range follows {
		input.FollowUps = append(input.FollowUps, ai.FollowUpContext{
			Type:      follow.Type,
			Content:   sanitizeIntentText(follow.Content, 2000),
			NextAt:    follow.NextAt,
			CreatedAt: follow.CreatedAt,
		})
	}

	var opportunities []model.CrmOpportunity
	if err := h.db.Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).
		Order("id DESC").Limit(10).Find(&opportunities).Error; err != nil {
		return input, fmt.Errorf("query opportunities: %w", err)
	}
	for _, opportunity := range opportunities {
		input.Opportunities = append(input.Opportunities, ai.OpportunityContext{
			Name:       opportunity.Name,
			Stage:      opportunity.Stage,
			Amount:     opportunity.Amount,
			ExpectDate: opportunity.ExpectDate,
			Remark:     sanitizeIntentText(opportunity.Remark, 1000),
		})
	}
	return input, nil
}

func sanitizeIntentText(value string, maxRunes int) string {
	value = intentEmailPattern.ReplaceAllString(value, "[邮箱已脱敏]")
	value = intentPhonePattern.ReplaceAllString(value, "[手机号已脱敏]")
	value = intentIDPattern.ReplaceAllString(value, "[证件号已脱敏]")
	runes := []rune(value)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return value
}

func (h *IntentHandler) toModel(customer *model.CrmCustomer, result *ai.IntentResult, analyzedAt time.Time, analysisID uint64) model.CrmCustomerIntent {
	return model.CrmCustomerIntent{
		TenantID:         customer.TenantID,
		CustomerID:       customer.ID,
		AnalysisID:       analysisID,
		IntentLevel:      result.IntentLevel,
		IntentScore:      result.IntentScore,
		Confidence:       result.Confidence,
		Summary:          result.Summary,
		Needs:            marshalStrings(result.Needs),
		PainPoints:       marshalStrings(result.PainPoints),
		Budget:           result.Budget,
		PurchaseTimeline: result.PurchaseTimeline,
		DecisionRole:     result.DecisionRole,
		Risks:            marshalStrings(result.Risks),
		NextAction:       result.NextAction,
		SuggestedNextAt:  result.SuggestedNextAt,
		AnalyzedAt:       analyzedAt,
		Provider:         h.provider.Name(),
		Model:            h.provider.Model(),
		PromptVersion:    ai.PromptVersion,
		Status:           intentStatusSuccess,
	}
}

func (h *IntentHandler) toResponse(intent *model.CrmCustomerIntent) IntentResponse {
	return IntentResponse{
		ID:                   intent.ID,
		TenantID:             intent.TenantID,
		CustomerID:           intent.CustomerID,
		AnalysisID:           intent.AnalysisID,
		IntentLevel:          intent.IntentLevel,
		EffectiveIntentLevel: effectiveIntentLevel(intent),
		IntentScore:          intent.IntentScore,
		Confidence:           intent.Confidence,
		Summary:              intent.Summary,
		Needs:                unmarshalStrings(intent.Needs),
		PainPoints:           unmarshalStrings(intent.PainPoints),
		Budget:               intent.Budget,
		PurchaseTimeline:     intent.PurchaseTimeline,
		DecisionRole:         intent.DecisionRole,
		Risks:                unmarshalStrings(intent.Risks),
		NextAction:           intent.NextAction,
		SuggestedNextAt:      intent.SuggestedNextAt,
		AnalyzedAt:           intent.AnalyzedAt,
		Provider:             intent.Provider,
		Model:                intent.Model,
		PromptVersion:        intent.PromptVersion,
		Status:               intent.Status,
		ManualOverride:       intent.ManualOverride,
		ManualIntentLevel:    intent.ManualIntentLevel,
		FollowUpStatus:       followUpStatus(intent.SuggestedNextAt),
		CreatedAt:            intent.CreatedAt,
		UpdatedAt:            intent.UpdatedAt,
	}
}

func intentListResponse(intent *model.CrmCustomerIntent) *IntentListResponse {
	if intent == nil {
		return nil
	}
	return &IntentListResponse{
		ID:                   intent.ID,
		CustomerID:           intent.CustomerID,
		AnalysisID:           intent.AnalysisID,
		IntentLevel:          intent.IntentLevel,
		EffectiveIntentLevel: effectiveIntentLevel(intent),
		IntentScore:          intent.IntentScore,
		Confidence:           intent.Confidence,
		Summary:              intent.Summary,
		NextAction:           intent.NextAction,
		SuggestedNextAt:      intent.SuggestedNextAt,
		AnalyzedAt:           intent.AnalyzedAt,
		Provider:             intent.Provider,
		Model:                intent.Model,
		Status:               intent.Status,
		ManualOverride:       intent.ManualOverride,
		ManualIntentLevel:    intent.ManualIntentLevel,
		FollowUpStatus:       followUpStatus(intent.SuggestedNextAt),
	}
}

func (h *IntentHandler) historyResponse(history *model.CrmCustomerIntentAnalysis) IntentHistoryResponse {
	var result *IntentResponse
	if history.ResultSnapshot != "" {
		var parsed ai.IntentResult
		if json.Unmarshal([]byte(history.ResultSnapshot), &parsed) == nil {
			current := h.toModel(&model.CrmCustomer{
				Base:     model.Base{ID: history.CustomerID},
				TenantID: history.TenantID,
			}, &parsed, valueOrNow(history.AnalyzedAt), history.ID)
			resultValue := h.toResponse(&current)
			result = &resultValue
		}
	}
	return IntentHistoryResponse{
		ID:                 history.ID,
		CustomerID:         history.CustomerID,
		TriggerUserID:      history.TriggerUserID,
		Result:             result,
		Status:             history.Status,
		ErrorMessage:       history.ErrorMessage,
		Provider:           history.Provider,
		Model:              history.Model,
		ActualModel:        history.ActualModel,
		ModelConfigVersion: history.ModelConfigVersion,
		AdapterVersion:     history.AdapterVersion,
		PromptVersion:      history.PromptVersion,
		InputTokens:        history.InputTokens,
		OutputTokens:       history.OutputTokens,
		TotalTokens:        history.TotalTokens,
		ProviderRequestID:  history.ProviderRequestID,
		ErrorType:          history.ErrorType,
		InputHash:          history.InputHash,
		CostMillis:         history.CostMillis,
		AnalyzedAt:         history.AnalyzedAt,
		InputSummary:       summarizeIntentInput(history.InputSnapshot),
		CreatedAt:          history.CreatedAt,
	}
}

func followUpStatus(nextAt *time.Time) string {
	if nextAt == nil {
		return "none"
	}
	now := time.Now()
	if nextAt.Before(now) {
		return "overdue"
	}
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if !nextAt.Before(start.AddDate(0, 0, 1)) {
		return "future"
	}
	return "today"
}

func effectiveIntentLevel(intent *model.CrmCustomerIntent) string {
	if intent != nil && intent.ManualOverride {
		switch intent.ManualIntentLevel {
		case "high", "medium", "low", "unknown":
			return intent.ManualIntentLevel
		}
	}
	if intent == nil {
		return ""
	}
	return intent.IntentLevel
}

func isCurrentIntentAnalysis(intent *model.CrmCustomerIntent, analysis *model.CrmCustomerIntentAnalysis) bool {
	if intent == nil || analysis == nil {
		return false
	}
	if intent.AnalysisID != 0 {
		return intent.AnalysisID == analysis.ID
	}
	return analysis.Status == intentStatusSuccess &&
		analysis.AnalyzedAt != nil &&
		intent.AnalyzedAt.Equal(*analysis.AnalyzedAt)
}

func summarizeIntentInput(snapshot string) string {
	if strings.TrimSpace(snapshot) == "" {
		return ""
	}
	var input ai.IntentInput
	if json.Unmarshal([]byte(snapshot), &input) != nil {
		return ""
	}
	return "客户：" + input.Customer.Name +
		"，联系人 " + strconv.Itoa(len(input.Contacts)) + " 个" +
		"，跟进记录 " + strconv.Itoa(len(input.FollowUps)) + " 条" +
		"，商机 " + strconv.Itoa(len(input.Opportunities)) + " 个"
}

func validateIntentResult(result *ai.IntentResult) error {
	if result == nil {
		return errors.New("empty intent result")
	}
	switch result.IntentLevel {
	case "high", "medium", "low", "unknown":
	default:
		return errors.New("invalid intent level")
	}
	if result.IntentScore != nil && (*result.IntentScore < 0 || *result.IntentScore > 100) {
		return errors.New("intent score out of range")
	}
	if result.Confidence != nil && (*result.Confidence < 0 || *result.Confidence > 1) {
		return errors.New("confidence out of range")
	}
	if result.IntentLevel != "unknown" && result.IntentScore == nil {
		return errors.New("intent score is required")
	}
	return nil
}

func marshalStrings(values []string) string {
	if values == nil {
		values = []string{}
	}
	data, _ := json.Marshal(values)
	return string(data)
}

func unmarshalStrings(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	var values []string
	if json.Unmarshal([]byte(value), &values) != nil {
		return []string{}
	}
	return values
}

func truncateIntentError(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func valueOrNow(value *time.Time) time.Time {
	if value != nil {
		return *value
	}
	return time.Now()
}
