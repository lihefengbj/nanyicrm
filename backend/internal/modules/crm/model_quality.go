package crm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lihefengbj/nanyicrm/backend/internal/ai"
)

const intentQualityGateMinFieldCompleteness = 0.75

type intentQualitySample struct {
	Name          string
	Input         ai.IntentInput
	ExpectedLevel string
}

// intentQualitySamples is a deterministic baseline used before a model can be
// published. It covers every supported intent level instead of only checking
// whether one response can be decoded as JSON.
func intentQualitySamples() []intentQualitySample {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	referenceTime := time.Date(2026, 1, 15, 10, 0, 0, 0, location)
	return []intentQualitySample{
		{
			Name: "high-intent",
			Input: ai.IntentInput{
				Customer: ai.CustomerContext{
					Name: "固定样本-明确采购客户", Industry: "企业服务", Source: "销售转介绍",
					Level: "A", Status: 1, Remark: "已确认本季度采购客户管理系统，预算约20万元，希望两周内完成评审。",
				},
				Contacts: []ai.ContactContext{{Name: "业务负责人", Position: "采购决策人"}},
				FollowUps: []ai.FollowUpContext{{
					Type: 3, Content: "客户确认方案方向，要求本周安排报价评审并邀请采购负责人参加。",
					CreatedAt: referenceTime.Add(-2 * time.Hour),
				}},
				Opportunities: []ai.OpportunityContext{{
					Name: "客户管理系统采购", Stage: 3, Amount: 200000,
					ExpectDate: ptrTime(referenceTime.Add(14 * 24 * time.Hour)), Remark: "等待报价评审",
				}},
				ReferenceTime: referenceTime, Timezone: "Asia/Shanghai", InputVersion: intentInputVersion,
			},
			ExpectedLevel: "high",
		},
		{
			Name: "medium-intent",
			Input: ai.IntentInput{
				Customer: ai.CustomerContext{
					Name: "固定样本-方案了解客户", Industry: "制造业", Source: "官网咨询",
					Level: "B", Status: 1, Remark: "客户对销售协同功能感兴趣，正在了解不同方案。",
				},
				Contacts: []ai.ContactContext{{Name: "王经理", Position: "销售负责人"}},
				FollowUps: []ai.FollowUpContext{{
					Type: 1, Content: "客户希望先看产品资料，预算和采购时间暂未明确。",
					CreatedAt: referenceTime.Add(-24 * time.Hour),
				}},
				ReferenceTime: referenceTime, Timezone: "Asia/Shanghai", InputVersion: intentInputVersion,
			},
			ExpectedLevel: "medium",
		},
		{
			Name: "low-intent",
			Input: ai.IntentInput{
				Customer: ai.CustomerContext{
					Name: "固定样本-低互动客户", Industry: "零售业", Source: "活动名单",
					Level: "C", Status: 1, Remark: "曾经下载过资料，近期没有明确需求。",
				},
				FollowUps: []ai.FollowUpContext{{
					Type: 4, Content: "销售两次联系未收到回复，客户没有给出下一步安排。",
					CreatedAt: referenceTime.Add(-10 * 24 * time.Hour),
				}},
				ReferenceTime: referenceTime, Timezone: "Asia/Shanghai", InputVersion: intentInputVersion,
			},
			ExpectedLevel: "low",
		},
		{
			Name: "unknown-intent",
			Input: ai.IntentInput{
				Customer: ai.CustomerContext{
					Name: "固定样本-信息不足客户", Industry: "", Source: "未知",
					Level: "", Status: 1, Remark: "",
				},
				ReferenceTime: referenceTime, Timezone: "Asia/Shanghai", InputVersion: intentInputVersion,
			},
			ExpectedLevel: "unknown",
		},
	}
}

func runQualityGateWithProvider(ctx context.Context, provider ai.Provider) (bool, string, string) {
	return runQualityGateWithPrompt(ctx, provider, "", "")
}

func runQualityGateWithPrompt(ctx context.Context, provider ai.Provider, promptVersion, promptContent string) (bool, string, string) {
	start := time.Now()
	samples := intentQualitySamples()
	metrics := map[string]interface{}{
		"sampleCount":                len(samples),
		"validCount":                 0,
		"levelMatchCount":            0,
		"fieldCompleteCount":         0,
		"fieldCompletenessThreshold": intentQualityGateMinFieldCompleteness,
		"totalTokens":                0,
		"structuredOutput":           true,
	}

	for _, sample := range samples {
		var (
			analysis *ai.IntentAnalysis
			err      error
		)
		if promptProvider, ok := provider.(ai.PromptAwareProvider); ok {
			analysis, err = promptProvider.AnalyzeCustomerIntentWithPrompt(ctx, sample.Input, promptVersion, promptContent)
		} else {
			analysis, err = provider.AnalyzeCustomerIntent(ctx, sample.Input)
		}
		if err != nil {
			metrics["structuredOutput"] = false
			metrics["errorSample"] = sample.Name
			metrics["errorType"], metrics["retryable"] = ai.ErrorInfo(err)
			metrics["latencyMillis"] = time.Since(start).Milliseconds()
			return false, fmt.Sprintf("固定样本 %s 调用失败，未通过质量门禁", sample.Name), marshalQualityMetrics(metrics)
		}
		if analysis == nil || analysis.Result == nil {
			metrics["structuredOutput"] = false
			metrics["errorSample"] = sample.Name
			metrics["errorType"] = ai.ErrorTypeEmptyResponse
			metrics["latencyMillis"] = time.Since(start).Milliseconds()
			return false, fmt.Sprintf("固定样本 %s 返回空结果，未通过质量门禁", sample.Name), marshalQualityMetrics(metrics)
		}
		if _, exists := metrics["actualModel"]; !exists {
			metrics["configuredProvider"] = analysis.Metadata.ConfiguredProvider
			metrics["configuredModel"] = analysis.Metadata.ConfiguredModel
			metrics["actualModel"] = analysis.Metadata.ActualModel
			metrics["configVersion"] = analysis.Metadata.ConfigVersion
			metrics["promptVersion"] = analysis.Metadata.PromptVersion
			metrics["adapterVersion"] = analysis.Metadata.AdapterVersion
		}
		metrics["totalTokens"] = metrics["totalTokens"].(int) + analysis.Metadata.TotalTokens
		if err := validateIntentResult(analysis.Result); err != nil {
			metrics["structuredOutput"] = false
			metrics["errorSample"] = sample.Name
			metrics["errorType"] = ai.ErrorTypeResultValidation
			metrics["latencyMillis"] = time.Since(start).Milliseconds()
			return false, fmt.Sprintf("固定样本 %s 结果校验失败，未通过质量门禁", sample.Name), marshalQualityMetrics(metrics)
		}

		metrics["validCount"] = metrics["validCount"].(int) + 1
		if analysis.Result.IntentLevel == sample.ExpectedLevel {
			metrics["levelMatchCount"] = metrics["levelMatchCount"].(int) + 1
		}
		if intentResultCompleteness(analysis.Result) >= intentQualityGateMinFieldCompleteness {
			metrics["fieldCompleteCount"] = metrics["fieldCompleteCount"].(int) + 1
		}
	}

	validCount := metrics["validCount"].(int)
	levelMatchCount := metrics["levelMatchCount"].(int)
	fieldCompleteCount := metrics["fieldCompleteCount"].(int)
	fieldCompletenessRate := float64(fieldCompleteCount) / float64(len(samples))
	levelMatchRate := float64(levelMatchCount) / float64(len(samples))
	metrics["fieldCompletenessRate"] = fieldCompletenessRate
	metrics["levelMatchRate"] = levelMatchRate
	metrics["latencyMillis"] = time.Since(start).Milliseconds()

	passed := validCount == len(samples) &&
		levelMatchCount == len(samples) &&
		fieldCompletenessRate >= intentQualityGateMinFieldCompleteness
	summary := fmt.Sprintf(
		"固定样本 %d/%d 通过；等级匹配 %d/%d；字段完整率 %.0f%%；总Token %d；耗时 %dms",
		validCount, len(samples), levelMatchCount, len(samples), fieldCompletenessRate*100,
		metrics["totalTokens"].(int), metrics["latencyMillis"],
	)
	if !passed {
		summary = "固定样本质量门禁未通过：" + summary
	}
	return passed, summary, marshalQualityMetrics(metrics)
}

func intentResultCompleteness(result *ai.IntentResult) float64 {
	if result == nil {
		return 0
	}
	total := 8
	present := 0
	if strings.TrimSpace(result.Summary) != "" {
		present++
	}
	if result.Needs != nil {
		present++
	}
	if result.PainPoints != nil {
		present++
	}
	if strings.TrimSpace(result.Budget) != "" {
		present++
	}
	if strings.TrimSpace(result.PurchaseTimeline) != "" {
		present++
	}
	if strings.TrimSpace(result.DecisionRole) != "" {
		present++
	}
	if result.Risks != nil {
		present++
	}
	if strings.TrimSpace(result.NextAction) != "" {
		present++
	}
	return float64(present) / float64(total)
}

func marshalQualityMetrics(metrics map[string]interface{}) string {
	body, err := json.Marshal(metrics)
	if err != nil {
		return `{"metricsError":"无法序列化质量门禁指标"}`
	}
	return string(body)
}
