package crm

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type IntentWorkbenchCustomer struct {
	CustomerID      uint64     `json:"customerId"`
	CustomerName    string     `json:"customerName"`
	IntentLevel     string     `json:"intentLevel"`
	IntentScore     *int       `json:"intentScore"`
	SuggestedNextAt *time.Time `json:"suggestedNextAt"`
	FollowUpStatus  string     `json:"followUpStatus"`
	Summary         string     `json:"summary"`
	ScoreDiff       *int       `json:"scoreDiff,omitempty"`
}

type IntentWorkbenchResponse struct {
	HighIntentCount      int64                     `json:"highIntentCount"`
	TodayFollowUpCount   int64                     `json:"todayFollowUpCount"`
	OverdueFollowUpCount int64                     `json:"overdueFollowUpCount"`
	FailedAnalysisCount  int64                     `json:"failedAnalysisCount"`
	RisingCustomers      []IntentWorkbenchCustomer `json:"risingCustomers"`
	FallingCustomers     []IntentWorkbenchCustomer `json:"fallingCustomers"`
	FailedCustomers      []IntentWorkbenchCustomer `json:"failedCustomers"`
}

type IntentMetricGroup struct {
	Provider           string `json:"provider"`
	Model              string `json:"model"`
	ActualModel        string `json:"actualModel"`
	ModelConfigVersion string `json:"modelConfigVersion"`
	PromptVersion      string `json:"promptVersion"`
	Count              int64  `json:"count"`
}

type IntentFailureGroup struct {
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}

type IntentMetricsResponse struct {
	From              time.Time            `json:"from"`
	To                time.Time            `json:"to"`
	RequestCount      int64                `json:"requestCount"`
	SuccessCount      int64                `json:"successCount"`
	FailedCount       int64                `json:"failedCount"`
	SuccessRate       float64              `json:"successRate"`
	AverageCostMillis float64              `json:"averageCostMillis"`
	P95CostMillis     int64                `json:"p95CostMillis"`
	InputTokens       int64                `json:"inputTokens"`
	OutputTokens      int64                `json:"outputTokens"`
	TotalTokens       int64                `json:"totalTokens"`
	ByProvider        []IntentMetricGroup  `json:"byProvider"`
	FailureReasons    []IntentFailureGroup `json:"failureReasons"`
	LevelDistribution map[string]int64     `json:"levelDistribution"`
	FeedbackCount     int64                `json:"feedbackCount"`
	AcceptedFeedback  int64                `json:"acceptedFeedback"`
	AcceptanceRate    float64              `json:"acceptanceRate"`
}

// Workbench returns tenant-scoped AI follow-up counters and a small set of
// customers that need attention. It intentionally exposes only derived
// operational data; it does not create CRM follow-up records automatically.
// @Summary  AI意向待跟进工作台
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:workbench
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/intent/workbench [get]
func (h *IntentHandler) Workbench(c *gin.Context) {
	tenantID, ok := resolveIntentTenant(c)
	if !ok {
		return
	}
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	query := h.db.Model(&model.CrmCustomerIntent{}).Where("tenant_id = ?", tenantID)

	var response IntentWorkbenchResponse
	if err := query.Where(effectiveIntentLevelSQL("crm_customer_intent")+" = ?", "high").
		Count(&response.HighIntentCount).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	if err := query.Where("suggested_next_at >= ? AND suggested_next_at < ?", start, start.AddDate(0, 0, 1)).
		Count(&response.TodayFollowUpCount).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	if err := query.Where("suggested_next_at < ?", now).Count(&response.OverdueFollowUpCount).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	if err := h.db.Model(&model.CrmCustomerIntentAnalysis{}).
		Where("tenant_id = ? AND status = ? AND created_at >= ?", tenantID, intentStatusFailed, now.AddDate(0, 0, -7)).
		Count(&response.FailedAnalysisCount).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}

	var customers []model.CrmCustomer
	if err := h.db.Model(&model.CrmCustomer{}).Select("id, name").
		Where("tenant_id = ?", tenantID).Limit(5000).Find(&customers).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	names := make(map[uint64]string, len(customers))
	for _, customer := range customers {
		names[customer.ID] = customer.Name
	}

	response.RisingCustomers, response.FallingCustomers = h.intentChanges(tenantID, names)
	response.FailedCustomers = h.failedCustomers(tenantID, names)
	common.OK(c, response)
}

// Metrics returns auditable model-call and feedback metrics for a tenant and
// time range. Conversion metrics are deliberately not fabricated here because
// CRM conversion attribution requires an explicit business definition.
// @Summary  AI意向运营指标
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:metrics；from/to 支持 RFC3339 或 YYYY-MM-DD
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/intent/metrics [get]
func (h *IntentHandler) Metrics(c *gin.Context) {
	tenantID, ok := resolveIntentTenant(c)
	if !ok {
		return
	}
	from, to := metricRange(c)
	query := h.db.Model(&model.CrmCustomerIntentAnalysis{}).
		Where("tenant_id = ? AND created_at >= ? AND created_at < ?", tenantID, from, to)

	var response IntentMetricsResponse
	response.From, response.To = from, to
	if err := query.Count(&response.RequestCount).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	if err := query.Where("status = ?", intentStatusSuccess).Count(&response.SuccessCount).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	if err := query.Where("status = ?", intentStatusFailed).Count(&response.FailedCount).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	if response.RequestCount > 0 {
		response.SuccessRate = float64(response.SuccessCount) / float64(response.RequestCount)
	}

	type aggregate struct {
		Provider           string
		Model              string
		ActualModel        string
		ModelConfigVersion string
		PromptVersion      string
		Count              int64
	}
	var groups []aggregate
	if err := query.Select("provider, model, actual_model, model_config_version, prompt_version, COUNT(*) AS count").
		Group("provider, model, actual_model, model_config_version, prompt_version").Order("count DESC").Scan(&groups).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	for _, group := range groups {
		response.ByProvider = append(response.ByProvider, IntentMetricGroup{
			Provider: group.Provider, Model: group.Model, ActualModel: group.ActualModel,
			ModelConfigVersion: group.ModelConfigVersion, PromptVersion: group.PromptVersion, Count: group.Count,
		})
	}

	var rows []model.CrmCustomerIntentAnalysis
	if err := query.Select("cost_millis, status, error_type, error_message, input_tokens, output_tokens, total_tokens").
		Order("cost_millis ASC").Find(&rows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	if len(rows) > 0 {
		var total int64
		costs := make([]int64, 0, len(rows))
		for _, row := range rows {
			total += row.CostMillis
			response.InputTokens += int64(row.InputTokens)
			response.OutputTokens += int64(row.OutputTokens)
			response.TotalTokens += int64(row.TotalTokens)
			costs = append(costs, row.CostMillis)
		}
		response.AverageCostMillis = float64(total) / float64(len(rows))
		index := int(float64(len(costs)-1) * 0.95)
		response.P95CostMillis = costs[index]
	}
	response.FailureReasons = failureGroups(rows)

	response.LevelDistribution = map[string]int64{"high": 0, "medium": 0, "low": 0, "unknown": 0}
	var levels []struct {
		Level string
		Count int64
	}
	if err := h.db.Model(&model.CrmCustomerIntent{}).Where("tenant_id = ?", tenantID).
		Select("intent_level AS level, COUNT(*) AS count").Group("intent_level").Scan(&levels).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	for _, level := range levels {
		response.LevelDistribution[level.Level] = level.Count
	}

	feedbackQuery := h.db.Model(&model.CrmCustomerIntentFeedback{}).
		Where("tenant_id = ? AND created_at >= ? AND created_at < ?", tenantID, from, to)
	if err := feedbackQuery.Count(&response.FeedbackCount).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	if err := feedbackQuery.Where("accepted = ?", true).Count(&response.AcceptedFeedback).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	if response.FeedbackCount > 0 {
		response.AcceptanceRate = float64(response.AcceptedFeedback) / float64(response.FeedbackCount)
	}
	common.OK(c, response)
}

func resolveIntentTenant(c *gin.Context) (uint64, bool) {
	if !middleware.IsPrivileged(c) {
		return middleware.CurrentTenantID(c), true
	}
	tenantID, err := strconv.ParseUint(c.Query("tenantId"), 10, 64)
	if err != nil || tenantID == 0 {
		common.FailMsg(c, common.CodeParamInvalid, "特权用户查询AI意向工作台时必须提供tenantId")
		return 0, false
	}
	return tenantID, true
}

func metricRange(c *gin.Context) (time.Time, time.Time) {
	now := time.Now()
	from := now.AddDate(0, 0, -7)
	to := now.Add(time.Second)
	if value := strings.TrimSpace(c.Query("from")); value != "" {
		if parsed, err := parseMetricTime(value, false); err == nil {
			from = parsed
		}
	}
	if value := strings.TrimSpace(c.Query("to")); value != "" {
		if parsed, err := parseMetricTime(value, true); err == nil {
			to = parsed
		}
	}
	return from, to
}

func parseMetricTime(value string, endOfDay bool) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", value, time.Local)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDay {
		return parsed.AddDate(0, 0, 1), nil
	}
	return parsed, nil
}

func (h *IntentHandler) intentChanges(tenantID uint64, names map[uint64]string) ([]IntentWorkbenchCustomer, []IntentWorkbenchCustomer) {
	var rows []model.CrmCustomerIntentAnalysis
	if err := h.db.Where("tenant_id = ? AND status = ? AND created_at >= ?", tenantID, intentStatusSuccess, time.Now().AddDate(0, 0, -7)).
		Order("customer_id ASC, id DESC").Limit(2000).Find(&rows).Error; err != nil {
		return nil, nil
	}
	type pair struct {
		current, previous *model.CrmCustomerIntentAnalysis
	}
	pairs := make(map[uint64]pair)
	for i := range rows {
		value := pairs[rows[i].CustomerID]
		if value.current == nil {
			value.current = &rows[i]
		} else if value.previous == nil {
			value.previous = &rows[i]
		}
		pairs[rows[i].CustomerID] = value
	}
	var rising, falling []IntentWorkbenchCustomer
	for customerID, pair := range pairs {
		if pair.current == nil || pair.previous == nil {
			continue
		}
		current := h.historyResponse(pair.current)
		previous := h.historyResponse(pair.previous)
		if current.Result == nil || previous.Result == nil || current.Result.IntentScore == nil || previous.Result.IntentScore == nil {
			continue
		}
		diff := *current.Result.IntentScore - *previous.Result.IntentScore
		if diff == 0 {
			continue
		}
		item := IntentWorkbenchCustomer{
			CustomerID: customerID, CustomerName: names[customerID],
			IntentLevel: current.Result.IntentLevel, IntentScore: current.Result.IntentScore,
			SuggestedNextAt: current.Result.SuggestedNextAt, FollowUpStatus: current.Result.FollowUpStatus,
			Summary: current.Result.Summary, ScoreDiff: &diff,
		}
		if diff > 0 {
			rising = append(rising, item)
		} else {
			falling = append(falling, item)
		}
	}
	sort.Slice(rising, func(i, j int) bool { return *rising[i].ScoreDiff > *rising[j].ScoreDiff })
	sort.Slice(falling, func(i, j int) bool { return *falling[i].ScoreDiff < *falling[j].ScoreDiff })
	if len(rising) > 10 {
		rising = rising[:10]
	}
	if len(falling) > 10 {
		falling = falling[:10]
	}
	return rising, falling
}

func (h *IntentHandler) failedCustomers(tenantID uint64, names map[uint64]string) []IntentWorkbenchCustomer {
	var rows []model.CrmCustomerIntentAnalysis
	if err := h.db.Where("tenant_id = ? AND status = ? AND created_at >= ?", tenantID, intentStatusFailed, time.Now().AddDate(0, 0, -7)).
		Order("id DESC").Limit(20).Find(&rows).Error; err != nil {
		return nil
	}
	result := make([]IntentWorkbenchCustomer, 0, len(rows))
	for _, row := range rows {
		result = append(result, IntentWorkbenchCustomer{
			CustomerID: row.CustomerID, CustomerName: names[row.CustomerID], Summary: row.ErrorMessage,
			IntentLevel: "unknown", FollowUpStatus: "none",
		})
	}
	return result
}

func failureGroups(rows []model.CrmCustomerIntentAnalysis) []IntentFailureGroup {
	counts := make(map[string]int64)
	for _, row := range rows {
		if row.Status != intentStatusFailed {
			continue
		}
		reason := strings.TrimSpace(row.ErrorMessage)
		if row.ErrorType != "" {
			reason = row.ErrorType
		}
		if reason == "" {
			reason = "unknown"
		}
		if len(reason) > 80 {
			reason = reason[:80]
		}
		counts[reason]++
	}
	result := make([]IntentFailureGroup, 0, len(counts))
	for reason, count := range counts {
		result = append(result, IntentFailureGroup{Reason: reason, Count: count})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Count > result[j].Count })
	return result
}
