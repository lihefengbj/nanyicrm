package crm

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/ai"
	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

const (
	intentStatusRunning = "running"
	intentStatusSuccess = "success"
	intentStatusFailed  = "failed"
)

var ErrIntentAnalysisUnavailable = errors.New("intent analysis unavailable")

type IntentHandler struct {
	db           *gorm.DB
	enabled      bool
	provider     ai.Provider
	taskEnqueuer func(context.Context, IntentTask) error
}

func NewIntentHandler(db *gorm.DB, enabled bool, provider ai.Provider) *IntentHandler {
	return &IntentHandler{db: db, enabled: enabled, provider: provider}
}

func (h *IntentHandler) SetTaskEnqueuer(enqueuer func(context.Context, IntentTask) error) {
	h.taskEnqueuer = enqueuer
}

type IntentResponse struct {
	ID                uint64     `json:"id"`
	TenantID          uint64     `json:"tenantId"`
	CustomerID        uint64     `json:"customerId"`
	IntentLevel       string     `json:"intentLevel"`
	IntentScore       *int       `json:"intentScore"`
	Confidence        *float64   `json:"confidence"`
	Summary           string     `json:"summary"`
	Needs             []string   `json:"needs"`
	PainPoints        []string   `json:"painPoints"`
	Budget            string     `json:"budget"`
	PurchaseTimeline  string     `json:"purchaseTimeline"`
	DecisionRole      string     `json:"decisionRole"`
	Risks             []string   `json:"risks"`
	NextAction        string     `json:"nextAction"`
	SuggestedNextAt   *time.Time `json:"suggestedNextAt"`
	AnalyzedAt        time.Time  `json:"analyzedAt"`
	Provider          string     `json:"provider"`
	Model             string     `json:"model"`
	PromptVersion     string     `json:"promptVersion"`
	Status            string     `json:"status"`
	ManualOverride    bool       `json:"manualOverride"`
	ManualIntentLevel string     `json:"manualIntentLevel"`
	FollowUpStatus    string     `json:"followUpStatus"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type IntentListResponse struct {
	ID                uint64     `json:"id"`
	CustomerID        uint64     `json:"customerId"`
	IntentLevel       string     `json:"intentLevel"`
	IntentScore       *int       `json:"intentScore"`
	Confidence        *float64   `json:"confidence"`
	Summary           string     `json:"summary"`
	NextAction        string     `json:"nextAction"`
	SuggestedNextAt   *time.Time `json:"suggestedNextAt"`
	AnalyzedAt        time.Time  `json:"analyzedAt"`
	Provider          string     `json:"provider"`
	Model             string     `json:"model"`
	Status            string     `json:"status"`
	ManualOverride    bool       `json:"manualOverride"`
	ManualIntentLevel string     `json:"manualIntentLevel"`
	FollowUpStatus    string     `json:"followUpStatus"`
}

type IntentHistoryResponse struct {
	ID            uint64          `json:"id"`
	CustomerID    uint64          `json:"customerId"`
	TriggerUserID uint64          `json:"triggerUserId"`
	Result        *IntentResponse `json:"result"`
	Status        string          `json:"status"`
	ErrorMessage  string          `json:"errorMessage"`
	Provider      string          `json:"provider"`
	Model         string          `json:"model"`
	PromptVersion string          `json:"promptVersion"`
	CostMillis    int64           `json:"costMillis"`
	AnalyzedAt    *time.Time      `json:"analyzedAt"`
	InputSummary  string          `json:"inputSummary"`
	CreatedAt     time.Time       `json:"createdAt"`
}

type IntentFeedbackRequest struct {
	AnalysisID        uint64 `json:"analysisId"`
	FeedbackType      string `json:"feedbackType" binding:"required,oneof=accurate partial inaccurate"`
	Accepted          *bool  `json:"accepted"`
	ManualIntentLevel string `json:"manualIntentLevel" binding:"omitempty,oneof=high medium low unknown"`
	Note              string `json:"note" binding:"max=1024"`
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
	intent, err := h.analyzeCustomer(c.Request.Context(), customer, middleware.CurrentUserID(c))
	if err != nil {
		if errors.Is(err, ErrIntentAnalysisUnavailable) {
			common.Fail(c, common.CodeAIUnavailable)
		} else {
			common.Fail(c, common.CodeDBError)
		}
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
	intent, err := h.analyzeCustomer(c.Request.Context(), customer, middleware.CurrentUserID(c))
	if err != nil {
		if errors.Is(err, ErrIntentAnalysisUnavailable) {
			common.Fail(c, common.CodeAIUnavailable)
		} else {
			common.Fail(c, common.CodeDBError)
		}
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
		current.ManualOverride = true
		current.ManualIntentLevel = req.ManualIntentLevel
		return tx.Save(&current).Error
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": feedback.ID})
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
	input, err := h.buildInput(customer)
	if err != nil {
		return nil, err
	}
	inputSnapshot, _ := json.Marshal(input)
	history := model.CrmCustomerIntentAnalysis{
		TenantID:      customer.TenantID,
		CustomerID:    customer.ID,
		TriggerUserID: triggerUserID,
		InputSnapshot: string(inputSnapshot),
		Status:        intentStatusRunning,
		Provider:      h.provider.Name(),
		Model:         h.provider.Model(),
		PromptVersion: ai.PromptVersion,
	}
	if err := h.db.Create(&history).Error; err != nil {
		return nil, err
	}

	start := time.Now()
	result, err := h.provider.AnalyzeCustomerIntent(ctx, input)
	history.CostMillis = time.Since(start).Milliseconds()
	if err != nil {
		history.Status = intentStatusFailed
		history.ErrorMessage = truncateIntentError(err.Error(), 1024)
		_ = h.db.Save(&history).Error
		return nil, ErrIntentAnalysisUnavailable
	}
	if err := validateIntentResult(result); err != nil {
		history.Status = intentStatusFailed
		history.ErrorMessage = truncateIntentError(err.Error(), 1024)
		_ = h.db.Save(&history).Error
		return nil, ErrIntentAnalysisUnavailable
	}

	resultSnapshot, _ := json.Marshal(result)
	analyzedAt := time.Now()
	history.Status = intentStatusSuccess
	history.ResultSnapshot = string(resultSnapshot)
	history.AnalyzedAt = &analyzedAt
	intent := h.toModel(customer, result, analyzedAt)
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&history).Error; err != nil {
			return err
		}
		var current model.CrmCustomerIntent
		findErr := tx.Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).First(&current).Error
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return tx.Create(&intent).Error
		}
		if findErr != nil {
			return findErr
		}
		intent.ID = current.ID
		intent.CreatedAt = current.CreatedAt
		return tx.Save(&intent).Error
	}); err != nil {
		return nil, err
	}
	return &intent, nil
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
	input := ai.IntentInput{
		Customer: ai.CustomerContext{
			Name:     customer.Name,
			Industry: customer.Industry,
			Source:   customer.Source,
			Level:    customer.Level,
			Status:   customer.Status,
			Remark:   customer.Remark,
		},
	}
	var contacts []model.CrmContact
	if err := h.db.Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).
		Order("is_primary DESC, id DESC").Limit(20).Find(&contacts).Error; err != nil {
		return input, err
	}
	for _, contact := range contacts {
		input.Contacts = append(input.Contacts, ai.ContactContext{Name: contact.Name, Position: contact.Position})
	}

	var follows []model.CrmFollowUp
	if err := h.db.Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).
		Order("id DESC").Limit(10).Find(&follows).Error; err != nil {
		return input, err
	}
	for _, follow := range follows {
		input.FollowUps = append(input.FollowUps, ai.FollowUpContext{
			Type:      follow.Type,
			Content:   follow.Content,
			NextAt:    follow.NextAt,
			CreatedAt: follow.CreatedAt,
		})
	}

	var opportunities []model.CrmOpportunity
	if err := h.db.Where("tenant_id = ? AND customer_id = ?", customer.TenantID, customer.ID).
		Order("id DESC").Limit(10).Find(&opportunities).Error; err != nil {
		return input, err
	}
	for _, opportunity := range opportunities {
		input.Opportunities = append(input.Opportunities, ai.OpportunityContext{
			Name:       opportunity.Name,
			Stage:      opportunity.Stage,
			Amount:     opportunity.Amount,
			ExpectDate: opportunity.ExpectDate,
			Remark:     opportunity.Remark,
		})
	}
	return input, nil
}

func (h *IntentHandler) toModel(customer *model.CrmCustomer, result *ai.IntentResult, analyzedAt time.Time) model.CrmCustomerIntent {
	return model.CrmCustomerIntent{
		TenantID:         customer.TenantID,
		CustomerID:       customer.ID,
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
		ID:                intent.ID,
		TenantID:          intent.TenantID,
		CustomerID:        intent.CustomerID,
		IntentLevel:       intent.IntentLevel,
		IntentScore:       intent.IntentScore,
		Confidence:        intent.Confidence,
		Summary:           intent.Summary,
		Needs:             unmarshalStrings(intent.Needs),
		PainPoints:        unmarshalStrings(intent.PainPoints),
		Budget:            intent.Budget,
		PurchaseTimeline:  intent.PurchaseTimeline,
		DecisionRole:      intent.DecisionRole,
		Risks:             unmarshalStrings(intent.Risks),
		NextAction:        intent.NextAction,
		SuggestedNextAt:   intent.SuggestedNextAt,
		AnalyzedAt:        intent.AnalyzedAt,
		Provider:          intent.Provider,
		Model:             intent.Model,
		PromptVersion:     intent.PromptVersion,
		Status:            intent.Status,
		ManualOverride:    intent.ManualOverride,
		ManualIntentLevel: intent.ManualIntentLevel,
		FollowUpStatus:    followUpStatus(intent.SuggestedNextAt),
		CreatedAt:         intent.CreatedAt,
		UpdatedAt:         intent.UpdatedAt,
	}
}

func intentListResponse(intent *model.CrmCustomerIntent) *IntentListResponse {
	if intent == nil {
		return nil
	}
	return &IntentListResponse{
		ID:                intent.ID,
		CustomerID:        intent.CustomerID,
		IntentLevel:       intent.IntentLevel,
		IntentScore:       intent.IntentScore,
		Confidence:        intent.Confidence,
		Summary:           intent.Summary,
		NextAction:        intent.NextAction,
		SuggestedNextAt:   intent.SuggestedNextAt,
		AnalyzedAt:        intent.AnalyzedAt,
		Provider:          intent.Provider,
		Model:             intent.Model,
		Status:            intent.Status,
		ManualOverride:    intent.ManualOverride,
		ManualIntentLevel: intent.ManualIntentLevel,
		FollowUpStatus:    followUpStatus(intent.SuggestedNextAt),
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
			}, &parsed, valueOrNow(history.AnalyzedAt))
			resultValue := h.toResponse(&current)
			result = &resultValue
		}
	}
	return IntentHistoryResponse{
		ID:            history.ID,
		CustomerID:    history.CustomerID,
		TriggerUserID: history.TriggerUserID,
		Result:        result,
		Status:        history.Status,
		ErrorMessage:  history.ErrorMessage,
		Provider:      history.Provider,
		Model:         history.Model,
		PromptVersion: history.PromptVersion,
		CostMillis:    history.CostMillis,
		AnalyzedAt:    history.AnalyzedAt,
		InputSummary:  summarizeIntentInput(history.InputSnapshot),
		CreatedAt:     history.CreatedAt,
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
