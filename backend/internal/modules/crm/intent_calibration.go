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
	Provider                string                 `json:"provider"`
	Model                   string                 `json:"model"`
	ActualModel             string                 `json:"actualModel"`
	ModelConfigVersion      string                 `json:"modelConfigVersion"`
	PromptVersion           string                 `json:"promptVersion"`
	SampleCount             int64                  `json:"sampleCount"`
	ScoredCount             int64                  `json:"scoredCount"`
	AverageScore            float64                `json:"averageScore"`
	MedianScore             float64                `json:"medianScore"`
	P95Score                float64                `json:"p95Score"`
	LevelDistribution       map[string]int64       `json:"levelDistribution"`
	FeedbackCount           int64                  `json:"feedbackCount"`
	ConsistentFeedbackCount int64                  `json:"consistentFeedbackCount"`
	ConsistencyRate         float64                `json:"consistencyRate"`
	ScoreCalibration        IntentScoreCalibration `json:"scoreCalibration"`
}

type IntentScoreCalibration struct {
	Status                 string  `json:"status"`
	Method                 string  `json:"method"`
	LabeledSampleCount     int64   `json:"labeledSampleCount"`
	Slope                  float64 `json:"slope"`
	Intercept              float64 `json:"intercept"`
	MeanAbsoluteError      float64 `json:"meanAbsoluteError"`
	CalibratedAverageScore float64 `json:"calibratedAverageScore"`
}

type IntentCalibrationResponse struct {
	From   time.Time                `json:"from"`
	To     time.Time                `json:"to"`
	Groups []IntentCalibrationGroup `json:"groups"`
}

type intentCalibrationAccumulator struct {
	group          IntentCalibrationGroup
	scores         []int
	labeledSamples []intentScoreCalibrationSample
}

type intentScoreCalibrationSample struct {
	rawScore    float64
	targetScore float64
}

const (
	intentScoreCalibrationReady             = "ready"
	intentScoreCalibrationInsufficientData  = "insufficient_data"
	intentScoreCalibrationInsufficientRange = "insufficient_range"
	intentScoreCalibrationMethod            = "linear_business_anchor_v1"
	intentScoreCalibrationMinSamples        = 5
)

var intentScoreCalibrationAnchors = map[string]float64{
	"high":   90,
	"medium": 65,
	"low":    25,
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
	latestFeedback := make(map[uint64]model.CrmCustomerIntentFeedback)
	for _, item := range feedback {
		acc := analysisGroups[item.AnalysisID]
		if acc == nil {
			continue
		}
		previous, alreadySeen := latestFeedback[item.AnalysisID]
		if !alreadySeen || item.CreatedAt.After(previous.CreatedAt) ||
			(item.CreatedAt.Equal(previous.CreatedAt) && item.ID > previous.ID) {
			latestFeedback[item.AnalysisID] = item
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
	for analysisID, item := range latestFeedback {
		acc := analysisGroups[analysisID]
		result, exists := analysisResults[analysisID]
		if acc == nil || !exists {
			continue
		}
		appendIntentScoreCalibrationSample(acc, result, item)
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
		acc.group.ScoreCalibration = fitIntentScoreCalibration(acc.labeledSamples, acc.scores)
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

func fitIntentScoreCalibration(samples []intentScoreCalibrationSample, allScores []int) IntentScoreCalibration {
	calibration := IntentScoreCalibration{
		Status:             intentScoreCalibrationInsufficientData,
		Method:             intentScoreCalibrationMethod,
		LabeledSampleCount: int64(len(samples)),
	}
	if len(samples) < intentScoreCalibrationMinSamples {
		return calibration
	}

	var sumX, sumY float64
	targets := make(map[float64]struct{})
	for _, sample := range samples {
		sumX += sample.rawScore
		sumY += sample.targetScore
		targets[sample.targetScore] = struct{}{}
	}
	if len(targets) < 2 {
		calibration.Status = intentScoreCalibrationInsufficientRange
		return calibration
	}

	meanX := sumX / float64(len(samples))
	meanY := sumY / float64(len(samples))
	var covariance, variance float64
	for _, sample := range samples {
		dx := sample.rawScore - meanX
		covariance += dx * (sample.targetScore - meanY)
		variance += dx * dx
	}
	if variance == 0 {
		calibration.Status = intentScoreCalibrationInsufficientRange
		return calibration
	}

	calibration.Slope = covariance / variance
	calibration.Intercept = meanY - calibration.Slope*meanX
	if calibration.Slope <= 0 {
		calibration.Status = intentScoreCalibrationInsufficientRange
		calibration.Slope = 0
		calibration.Intercept = 0
		return calibration
	}

	var absoluteError float64
	for _, sample := range samples {
		absoluteError += absFloat(clampIntentScore(calibration.Slope*sample.rawScore+calibration.Intercept) - sample.targetScore)
	}
	calibration.MeanAbsoluteError = absoluteError / float64(len(samples))

	if len(allScores) > 0 {
		var total float64
		for _, score := range allScores {
			total += clampIntentScore(calibration.Slope*float64(score) + calibration.Intercept)
		}
		calibration.CalibratedAverageScore = total / float64(len(allScores))
	}
	calibration.Status = intentScoreCalibrationReady
	return calibration
}

func appendIntentScoreCalibrationSample(acc *intentCalibrationAccumulator, result ai.IntentResult, feedback model.CrmCustomerIntentFeedback) {
	target, ok := intentScoreCalibrationAnchors[feedback.ManualIntentLevel]
	if !ok || result.IntentScore == nil {
		return
	}
	acc.labeledSamples = append(acc.labeledSamples, intentScoreCalibrationSample{
		rawScore:    float64(*result.IntentScore),
		targetScore: target,
	})
}

func clampIntentScore(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func percentileScore(values []int, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * percentile)
	return float64(values[index])
}
