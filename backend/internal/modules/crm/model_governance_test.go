package crm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lihefengbj/nanyicrm/backend/internal/ai"
	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type qualityGateProvider struct {
	results map[string]*ai.IntentResult
	err     error
}

func (p qualityGateProvider) AnalyzeCustomerIntent(_ context.Context, input ai.IntentInput) (*ai.IntentAnalysis, error) {
	if p.err != nil {
		return nil, p.err
	}
	return &ai.IntentAnalysis{
		Result:   p.results[input.Customer.Name],
		Metadata: ai.CallMetadata{TotalTokens: 10},
	}, nil
}

func (p qualityGateProvider) Name() string  { return "quality-test" }
func (p qualityGateProvider) Model() string { return "quality-test-model" }

func completeQualityResult(level string) *ai.IntentResult {
	score := 60
	if level == "high" {
		score = 90
	} else if level == "low" {
		score = 20
	}
	confidence := 0.8
	return &ai.IntentResult{
		IntentLevel: level, IntentScore: &score, Confidence: &confidence,
		Summary: "固定样本摘要", Needs: []string{"客户管理"},
		PainPoints: []string{"资料分散"}, Budget: "未明确",
		PurchaseTimeline: "未明确", DecisionRole: "未明确",
		Risks: []string{}, NextAction: "安排下一次沟通",
	}
}

func TestRunQualityGateWithProviderCoversAllIntentLevels(t *testing.T) {
	results := make(map[string]*ai.IntentResult)
	for _, sample := range intentQualitySamples() {
		results[sample.Input.Customer.Name] = completeQualityResult(sample.ExpectedLevel)
	}

	passed, summary, rawMetrics := runQualityGateWithProvider(context.Background(), qualityGateProvider{results: results})
	if !passed {
		t.Fatalf("quality gate failed: %s, metrics=%s", summary, rawMetrics)
	}
	var metrics map[string]interface{}
	if err := json.Unmarshal([]byte(rawMetrics), &metrics); err != nil {
		t.Fatalf("metrics are not JSON: %v", err)
	}
	if metrics["sampleCount"] != float64(4) || metrics["levelMatchCount"] != float64(4) {
		t.Fatalf("unexpected sample metrics: %v", metrics)
	}
	if metrics["fieldCompletenessRate"] != float64(1) {
		t.Fatalf("unexpected completeness metrics: %v", metrics)
	}
}

func TestRunQualityGateWithProviderRejectsLevelDrift(t *testing.T) {
	results := make(map[string]*ai.IntentResult)
	for _, sample := range intentQualitySamples() {
		results[sample.Input.Customer.Name] = completeQualityResult("medium")
	}

	passed, summary, _ := runQualityGateWithProvider(context.Background(), qualityGateProvider{results: results})
	if passed || summary == "" {
		t.Fatalf("expected level drift to fail quality gate, summary=%q", summary)
	}
}

func TestRunQualityGateWithProviderClassifiesProviderFailure(t *testing.T) {
	passed, summary, rawMetrics := runQualityGateWithProvider(
		context.Background(),
		qualityGateProvider{err: &ai.ProviderError{Type: ai.ErrorTypeAuthentication, Err: errors.New("hidden")}},
	)
	if passed || summary == "" {
		t.Fatalf("expected provider failure to fail quality gate")
	}
	var metrics map[string]interface{}
	if err := json.Unmarshal([]byte(rawMetrics), &metrics); err != nil {
		t.Fatalf("metrics are not JSON: %v", err)
	}
	if metrics["errorType"] != ai.ErrorTypeAuthentication || metrics["errorSample"] != "high-intent" {
		t.Fatalf("unexpected failure metrics: %v", metrics)
	}
}

func TestModelConfigToLLMPreservesEnvironmentAPIKeyWithoutCredential(t *testing.T) {
	base := config.LLMConfig{
		Provider: "deepseek",
		BaseURL:  "https://api.deepseek.com",
		APIKey:   "environment-key",
		Model:    "deepseek-flash",
	}
	got := modelConfigToLLM(base, model.SysAIModelConfig{
		Provider: "openai-compatible",
		BaseURL:  "https://api.example.com",
		Model:    "candidate-model",
	})
	if got.APIKey != "environment-key" {
		t.Fatalf("API key = %q, want environment key to remain the fallback", got.APIKey)
	}
	if got.BaseURL != "https://api.example.com" || got.Model != "candidate-model" {
		t.Fatalf("candidate model settings were not applied: %+v", got)
	}
}

func TestApplyCallMetadataRecordsSelectedPromptOnProviderFailure(t *testing.T) {
	history := model.CrmCustomerIntentAnalysis{
		Provider:      "openai-compatible",
		Model:         "legacy-model",
		PromptVersion: ai.PromptVersion,
	}
	applyCallMetadata(&history, ai.CallMetadata{
		ConfiguredProvider: "deepseek",
		ConfiguredModel:    "deepseek-v4-pro",
		ConfigVersion:      "deepseek-v4-pro-v1",
		PromptVersion:      "p-41a70de015d2",
		AdapterVersion:     "openai-compatible-v1",
	})
	if history.Provider != "deepseek" ||
		history.Model != "deepseek-v4-pro" ||
		history.ModelConfigVersion != "deepseek-v4-pro-v1" ||
		history.PromptVersion != "p-41a70de015d2" {
		t.Fatalf("selected call metadata was not recorded: %+v", history)
	}
}
