package crm

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lihefengbj/nanyicrm/backend/internal/ai"
	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type IntentCalibrationGroup struct {
	Provider                string           `json:"provider"`
	Model                   string           `json:"model"`
	ActualModel             string           `json:"actualModel"`
	ModelConfigVersion      string           `json:"modelConfigVersion"`
	PromptVersion           string           `json:"promptVersion"`
	SampleCount             int64            `json:"sampleCount"`
	ScoredCount             int64            `json:"scoredCount"`
	AverageScore            float64          `json:"averageScore"`
	MedianScore             float64          `json:"medianScore"`
	P95Score                float64          `json:"p95Score"`
	LevelDistribution       map[string]int64 `json:"levelDistribution"`
	FeedbackCount           int64            `json:"feedbackCount"`
	ConsistentFeedbackCount int64            `json:"consistentFeedbackCount"`
	ConsistencyRate         float64          `json:"consistencyRate"`
}

type IntentCalibrationResponse struct {
	From   time.Time                `json:"from"`
	To     time.Time                `json:"to"`
	Groups []IntentCalibrationGroup `json:"groups"`
}

type intentCalibrationAccumulator struct {
	group  IntentCalibrationGroup
	scores []int
}

// Calibration reports score and feedback consistency by governed model
// combination. It intentionally uses explicit feedback as the business
// calibration signal; it does not treat conversion as a model label.
// @Summary  AI意向评分校准指标
// @Tags     CRM-客户意向
// @Description 需要权限：crm:intent:metrics；按 Provider、模型配置版本和 Prompt 版本分组
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/intent/calibration [get]
func (h *IntentHandler) Calibration(c *gin.Context) {
	tenantID, ok := resolveIntentTenant(c)
	if !ok {
		return
	}
	from, to := metricRange(c)

	var analyses []model.CrmCustomerIntentAnalysis
	if err := h.db.Where(
		"tenant_id = ? AND status = ? AND created_at >= ? AND created_at < ?",
		tenantID, intentStatusSuccess, from, to,
	).Order("id ASC").Find(&analyses).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}

	type groupKey struct {
		Provider           string
		Model              string
		ActualModel        string
		ModelConfigVersion string
		PromptVersion      string
	}
	groups := make(map[groupKey]*intentCalibrationAccumulator)
	analysisGroups := make(map[uint64]*intentCalibrationAccumulator)
	analysisResults := make(map[uint64]ai.IntentResult)
	for _, analysis := range analyses {
		var result ai.IntentResult
		if json.Unmarshal([]byte(analysis.ResultSnapshot), &result) != nil {
			continue
		}
		key := groupKey{
			Provider:           analysis.Provider,
			Model:              analysis.Model,
			ActualModel:        analysis.ActualModel,
			ModelConfigVersion: analysis.ModelConfigVersion,
			PromptVersion:      analysis.PromptVersion,
		}
		acc := groups[key]
		if acc == nil {
			acc = &intentCalibrationAccumulator{
				group: IntentCalibrationGroup{
					Provider:           key.Provider,
					Model:              key.Model,
					ActualModel:        key.ActualModel,
					ModelConfigVersion: key.ModelConfigVersion,
					PromptVersion:      key.PromptVersion,
					LevelDistribution:  map[string]int64{"high": 0, "medium": 0, "low": 0, "unknown": 0},
				},
			}
			groups[key] = acc
		}
		acc.group.SampleCount++
		if _, exists := acc.group.LevelDistribution[result.IntentLevel]; exists {
			acc.group.LevelDistribution[result.IntentLevel]++
		}
		if result.IntentScore != nil {
			acc.group.ScoredCount++
			acc.scores = append(acc.scores, *result.IntentScore)
		}
		analysisGroups[analysis.ID] = acc
		analysisResults[analysis.ID] = result
	}

	var feedback []model.CrmCustomerIntentFeedback
	if err := h.db.Where(
		"tenant_id = ? AND created_at >= ? AND created_at < ?",
		tenantID, from, to,
	).Find(&feedback).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	for _, item := range feedback {
		acc := analysisGroups[item.AnalysisID]
		if acc == nil {
			continue
		}
		acc.group.FeedbackCount++
		result, exists := analysisResults[item.AnalysisID]
		if !exists {
			continue
		}
		consistent := item.ManualIntentLevel != "" && item.ManualIntentLevel == result.IntentLevel
		if item.ManualIntentLevel == "" && item.FeedbackType == "accurate" {
			consistent = true
		}
		if consistent {
			acc.group.ConsistentFeedbackCount++
		}
	}

	result := IntentCalibrationResponse{From: from, To: to, Groups: make([]IntentCalibrationGroup, 0, len(groups))}
	for _, acc := range groups {
		sort.Ints(acc.scores)
		if len(acc.scores) > 0 {
			var total int
			for _, score := range acc.scores {
				total += score
			}
			acc.group.AverageScore = float64(total) / float64(len(acc.scores))
			acc.group.MedianScore = percentileScore(acc.scores, 0.50)
			acc.group.P95Score = percentileScore(acc.scores, 0.95)
		}
		if acc.group.FeedbackCount > 0 {
			acc.group.ConsistencyRate = float64(acc.group.ConsistentFeedbackCount) / float64(acc.group.FeedbackCount)
		}
		result.Groups = append(result.Groups, acc.group)
	}
	sort.SliceStable(result.Groups, func(i, j int) bool {
		if result.Groups[i].SampleCount != result.Groups[j].SampleCount {
			return result.Groups[i].SampleCount > result.Groups[j].SampleCount
		}
		if result.Groups[i].PromptVersion != result.Groups[j].PromptVersion {
			return result.Groups[i].PromptVersion < result.Groups[j].PromptVersion
		}
		return result.Groups[i].ModelConfigVersion < result.Groups[j].ModelConfigVersion
	})
	common.OK(c, result)
}

func percentileScore(values []int, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * percentile)
	return float64(values[index])
}
