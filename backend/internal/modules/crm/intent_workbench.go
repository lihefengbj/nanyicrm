package crm

import (
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/config"
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
	Provider             string  `json:"provider"`
	Model                string  `json:"model"`
	ActualModel          string  `json:"actualModel"`
	ModelConfigVersion   string  `json:"modelConfigVersion"`
	PromptVersion        string  `json:"promptVersion"`
	Count                int64   `json:"count"`
	SuccessCount         int64   `json:"successCount"`
	FailedCount          int64   `json:"failedCount"`
	IncompleteCount      int64   `json:"incompleteCount"`
	InputTokens          int64   `json:"inputTokens"`
	InputCacheHitTokens  int64   `json:"inputCacheHitTokens"`
	InputCacheMissTokens int64   `json:"inputCacheMissTokens"`
	OutputTokens         int64   `json:"outputTokens"`
	TotalTokens          int64   `json:"totalTokens"`
	InputCacheHitCost    float64 `json:"inputCacheHitCost"`
	InputCacheMissCost   float64 `json:"inputCacheMissCost"`
	OutputCost           float64 `json:"outputCost"`
	EstimatedCost        float64 `json:"estimatedCost"`
	BillingPeriod        string  `json:"billingPeriod"`
}

type IntentFailureGroup struct {
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}

type IntentMetricsResponse struct {
	From                 time.Time            `json:"from"`
	To                   time.Time            `json:"to"`
	RequestCount         int64                `json:"requestCount"`
	SuccessCount         int64                `json:"successCount"`
	FailedCount          int64                `json:"failedCount"`
	IncompleteCount      int64                `json:"incompleteCount"`
	SuccessRate          float64              `json:"successRate"`
	AverageCostMillis    float64              `json:"averageCostMillis"`
	P95CostMillis        int64                `json:"p95CostMillis"`
	InputTokens          int64                `json:"inputTokens"`
	InputCacheHitTokens  int64                `json:"inputCacheHitTokens"`
	InputCacheMissTokens int64                `json:"inputCacheMissTokens"`
	OutputTokens         int64                `json:"outputTokens"`
	TotalTokens          int64                `json:"totalTokens"`
	InputCacheHitCost    float64              `json:"inputCacheHitCost"`
	InputCacheMissCost   float64              `json:"inputCacheMissCost"`
	InputCost            float64              `json:"inputCost"`
	OutputCost           float64              `json:"outputCost"`
	EstimatedCost        float64              `json:"estimatedCost"`
	CostCurrency         string               `json:"costCurrency"`
	CostPeriod           string               `json:"costPeriod"`
	CostMode             string               `json:"costMode"`
	CostFixedPeriod      string               `json:"costFixedPeriod"`
	CostConfigured       bool                 `json:"costConfigured"`
	ByProvider           []IntentMetricGroup  `json:"byProvider"`
	FailureReasons       []IntentFailureGroup `json:"failureReasons"`
	LevelDistribution    map[string]int64     `json:"levelDistribution"`
	FeedbackCount        int64                `json:"feedbackCount"`
	AcceptedFeedback     int64                `json:"acceptedFeedback"`
	AcceptanceRate       float64              `json:"acceptanceRate"`
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
	analysisQuery := func() *gorm.DB {
		return h.db.Model(&model.CrmCustomerIntentAnalysis{}).
			Where("tenant_id = ? AND created_at >= ? AND created_at < ?", tenantID, from, to)
	}

	var response IntentMetricsResponse
	response.From, response.To = from, to
	if err := analysisQuery().Count(&response.RequestCount).Error; err != nil {
		failIntentMetricsDB(c, "request_count", err)
		return
	}
	if err := analysisQuery().Where("status = ?", intentStatusSuccess).Count(&response.SuccessCount).Error; err != nil {
		failIntentMetricsDB(c, "success_count", err)
		return
	}
	if err := analysisQuery().Where("status = ?", intentStatusFailed).Count(&response.FailedCount).Error; err != nil {
		failIntentMetricsDB(c, "failed_count", err)
		return
	}
	if response.RequestCount > 0 {
		response.SuccessRate = float64(response.SuccessCount) / float64(response.RequestCount)
	}

	type aggregate struct {
		Provider             string
		Model                string
		ActualModel          string
		ModelConfigVersion   string
		PromptVersion        string
		Count                int64
		SuccessCount         int64
		FailedCount          int64
		IncompleteCount      int64
		InputTokens          int64
		InputCacheHitTokens  int64
		InputCacheMissTokens int64
		OutputTokens         int64
		TotalTokens          int64
		BillingPeriod        string
		InputCacheHitCost    float64
		InputCacheMissCost   float64
		OutputCost           float64
	}

	var rows []model.CrmCustomerIntentAnalysis
	if err := analysisQuery().Select("created_at, cost_millis, status, error_type, error_message, provider, model, actual_model, model_config_version, prompt_version, input_tokens, input_cache_hit_tokens, input_cache_miss_tokens, output_tokens, total_tokens, billing_period").
		Order("cost_millis ASC").Find(&rows).Error; err != nil {
		failIntentMetricsDB(c, "analysis_rows", err)
		return
	}
	if len(rows) > 0 {
		var total int64
		costs := make([]int64, 0, len(rows))
		var inputCacheHitCost, inputCacheMissCost, outputCost float64
		type groupKey struct {
			Provider           string
			Model              string
			ActualModel        string
			ModelConfigVersion string
			PromptVersion      string
			BillingPeriod      string
		}
		groupsByKey := make(map[groupKey]*aggregate)
		for _, row := range rows {
			total += row.CostMillis
			response.InputTokens += int64(row.InputTokens)
			inputCacheHitTokens, inputCacheMissTokens := normalizeInputCacheTokens(
				int64(row.InputTokens), int64(row.InputCacheHitTokens), int64(row.InputCacheMissTokens),
			)
			response.InputCacheHitTokens += inputCacheHitTokens
			response.InputCacheMissTokens += inputCacheMissTokens
			response.OutputTokens += int64(row.OutputTokens)
			response.TotalTokens += int64(row.TotalTokens)
			billingPeriod := effectiveBillingPeriod(h.pricing, row.BillingPeriod, row.CreatedAt)
			hitCost, missCost, rowOutputCost := estimateIntentCostByPeriod(
				h.pricing, billingPeriod, inputCacheHitTokens, inputCacheMissTokens, int64(row.OutputTokens),
			)
			inputCacheHitCost += hitCost
			inputCacheMissCost += missCost
			outputCost += rowOutputCost
			costs = append(costs, row.CostMillis)

			key := groupKey{
				Provider:           row.Provider,
				Model:              row.Model,
				ActualModel:        row.ActualModel,
				ModelConfigVersion: row.ModelConfigVersion,
				PromptVersion:      row.PromptVersion,
				BillingPeriod:      billingPeriod,
			}
			group := groupsByKey[key]
			if group == nil {
				group = &aggregate{
					Provider:           row.Provider,
					Model:              row.Model,
					ActualModel:        row.ActualModel,
					ModelConfigVersion: row.ModelConfigVersion,
					PromptVersion:      row.PromptVersion,
					BillingPeriod:      billingPeriod,
				}
				groupsByKey[key] = group
			}
			group.Count++
			if row.Status == intentStatusSuccess {
				group.SuccessCount++
			}
			if row.Status == intentStatusFailed {
				group.FailedCount++
			}
			if row.Status != intentStatusSuccess && row.Status != intentStatusFailed {
				response.IncompleteCount++
				group.IncompleteCount++
			}
			group.InputTokens += int64(row.InputTokens)
			group.InputCacheHitTokens += inputCacheHitTokens
			group.InputCacheMissTokens += inputCacheMissTokens
			group.OutputTokens += int64(row.OutputTokens)
			group.TotalTokens += int64(row.TotalTokens)
			group.InputCacheHitCost += hitCost
			group.InputCacheMissCost += missCost
			group.OutputCost += rowOutputCost
		}
		response.AverageCostMillis = float64(total) / float64(len(rows))
		index := int(float64(len(costs)-1) * 0.95)
		response.P95CostMillis = costs[index]
		response.InputCacheHitCost = inputCacheHitCost
		response.InputCacheMissCost = inputCacheMissCost
		response.OutputCost = outputCost

		groups := make([]aggregate, 0, len(groupsByKey))
		for _, group := range groupsByKey {
			groups = append(groups, *group)
		}
		sort.SliceStable(groups, func(i, j int) bool {
			if groups[i].Count != groups[j].Count {
				return groups[i].Count > groups[j].Count
			}
			if groups[i].BillingPeriod != groups[j].BillingPeriod {
				return groups[i].BillingPeriod < groups[j].BillingPeriod
			}
			return groups[i].Provider < groups[j].Provider
		})
		for _, group := range groups {
			response.ByProvider = append(response.ByProvider, IntentMetricGroup{
				Provider: group.Provider, Model: group.Model, ActualModel: group.ActualModel,
				ModelConfigVersion: group.ModelConfigVersion, PromptVersion: group.PromptVersion,
				Count: group.Count, SuccessCount: group.SuccessCount, FailedCount: group.FailedCount,
				IncompleteCount: group.IncompleteCount,
				InputTokens:     group.InputTokens, InputCacheHitTokens: group.InputCacheHitTokens,
				InputCacheMissTokens: group.InputCacheMissTokens, OutputTokens: group.OutputTokens,
				TotalTokens: group.TotalTokens, InputCacheHitCost: group.InputCacheHitCost,
				InputCacheMissCost: group.InputCacheMissCost, OutputCost: group.OutputCost,
				EstimatedCost: group.InputCacheHitCost + group.InputCacheMissCost + group.OutputCost,
				BillingPeriod: group.BillingPeriod,
			})
		}
	}
	response.FailureReasons = failureGroups(rows)
	response.CostCurrency = h.pricing.Currency
	response.CostMode = h.pricing.ModeValue()
	response.CostFixedPeriod = h.pricing.FixedPeriodValue()
	response.CostPeriod = response.CostMode
	response.CostConfigured = h.pricing.IsConfigured()
	response.InputCost = response.InputCacheHitCost + response.InputCacheMissCost
	response.EstimatedCost = response.InputCost + response.OutputCost

	response.LevelDistribution = map[string]int64{"high": 0, "medium": 0, "low": 0, "unknown": 0}
	var levels []struct {
		Level string
		Count int64
	}
	if err := h.db.Model(&model.CrmCustomerIntent{}).Where("tenant_id = ?", tenantID).
		Select("intent_level AS level, COUNT(*) AS count").Group("intent_level").Scan(&levels).Error; err != nil {
		failIntentMetricsDB(c, "level_distribution", err)
		return
	}
	for _, level := range levels {
		response.LevelDistribution[level.Level] = level.Count
	}

	feedbackQuery := h.db.Model(&model.CrmCustomerIntentFeedback{}).
		Where("tenant_id = ? AND created_at >= ? AND created_at < ?", tenantID, from, to)
	if err := feedbackQuery.Count(&response.FeedbackCount).Error; err != nil {
		failIntentMetricsDB(c, "feedback_count", err)
		return
	}
	if err := feedbackQuery.Where("accepted = ?", true).Count(&response.AcceptedFeedback).Error; err != nil {
		failIntentMetricsDB(c, "accepted_feedback", err)
		return
	}
	if response.FeedbackCount > 0 {
		response.AcceptanceRate = float64(response.AcceptedFeedback) / float64(response.FeedbackCount)
	}
	common.OK(c, response)
}

func failIntentMetricsDB(c *gin.Context, stage string, err error) {
	log.Printf("intent metrics database error stage=%s: %v", stage, err)
	common.Fail(c, common.CodeDBError)
}

func estimateIntentCost(pricing config.PricingConfig, inputTokens, outputTokens int64) float64 {
	input, output := estimateIntentCostParts(pricing, inputTokens, outputTokens)
	return input + output
}

func estimateIntentCostParts(pricing config.PricingConfig, inputTokens, outputTokens int64) (float64, float64) {
	rates := pricing.Rates()
	input := float64(inputTokens) / 1_000_000 * rates.InputCacheMissPerMillion
	output := float64(outputTokens) / 1_000_000 * rates.OutputPerMillion
	return input, output
}

func effectiveBillingPeriod(pricing config.PricingConfig, stored string, createdAt time.Time) string {
	switch strings.ToLower(strings.TrimSpace(stored)) {
	case config.PricingPeriodIdle:
		return config.PricingPeriodIdle
	case config.PricingPeriodPeak:
		return config.PricingPeriodPeak
	case config.PricingPeriodUnified:
		return config.PricingPeriodUnified
	default:
		// Older rows were created before billing_period was persisted. Their
		// period must be derived from the request creation time, not from the
		// current time (which could be a weekend or a different price window).
		return pricing.PeriodAt(createdAt)
	}
}

func estimateIntentCostByCache(pricing config.PricingConfig, inputCacheHitTokens, inputCacheMissTokens, outputTokens int64) (float64, float64, float64) {
	return estimateIntentCostByPeriod(pricing, pricing.PeriodAt(time.Now()), inputCacheHitTokens, inputCacheMissTokens, outputTokens)
}

func estimateIntentCostByPeriod(pricing config.PricingConfig, period string, inputCacheHitTokens, inputCacheMissTokens, outputTokens int64) (float64, float64, float64) {
	rates := pricing.RatesForPeriod(period)
	inputCacheHitCost := float64(inputCacheHitTokens) / 1_000_000 * rates.InputCacheHitPerMillion
	inputCacheMissCost := float64(inputCacheMissTokens) / 1_000_000 * rates.InputCacheMissPerMillion
	outputCost := float64(outputTokens) / 1_000_000 * rates.OutputPerMillion
	return inputCacheHitCost, inputCacheMissCost, outputCost
}

func normalizeInputCacheTokens(inputTokens, inputCacheHitTokens, inputCacheMissTokens int64) (int64, int64) {
	if inputCacheHitTokens < 0 {
		inputCacheHitTokens = 0
	}
	if inputCacheMissTokens < 0 {
		inputCacheMissTokens = 0
	}
	if inputCacheHitTokens+inputCacheMissTokens == 0 && inputTokens > 0 {
		return 0, inputTokens
	}
	if inputCacheHitTokens+inputCacheMissTokens < inputTokens {
		inputCacheMissTokens += inputTokens - inputCacheHitTokens - inputCacheMissTokens
	}
	return inputCacheHitTokens, inputCacheMissTokens
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
