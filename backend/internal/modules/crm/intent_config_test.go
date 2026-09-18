package crm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lihefengbj/nanyicrm/backend/internal/ai"
	"github.com/lihefengbj/nanyicrm/backend/internal/common"
)

type configTestProvider struct {
	analysis *ai.IntentAnalysis
	err      error
}

func (p *configTestProvider) AnalyzeCustomerIntent(context.Context, ai.IntentInput) (*ai.IntentAnalysis, error) {
	return p.analysis, p.err
}

func (p *configTestProvider) Name() string  { return "test-provider" }
func (p *configTestProvider) Model() string { return "configured-model" }

func TestIntentConfigTestSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	score := 72
	confidence := 0.86
	handler := NewIntentHandler(nil, true, &configTestProvider{
		analysis: &ai.IntentAnalysis{
			Result: &ai.IntentResult{
				IntentLevel: "medium",
				IntentScore: &score,
				Confidence:  &confidence,
			},
			Metadata: ai.CallMetadata{
				ActualModel:    "actual-model",
				InputTokens:    30,
				OutputTokens:   20,
				TotalTokens:    50,
				ConfigVersion:  "test-v2",
				AdapterVersion: "test-adapter",
			},
		},
	}, nil)

	response := performIntentConfigTest(handler)
	if response.Code != common.CodeSuccess {
		t.Fatalf("response code = %d", response.Code)
	}
	if !response.Data.Available || !response.Data.StructuredOutput {
		t.Fatalf("unexpected response: %+v", response.Data)
	}
	if response.Data.ActualModel != "actual-model" || response.Data.TotalTokens != 50 {
		t.Fatalf("unexpected metadata: %+v", response.Data)
	}
}

func TestIntentConfigTestReturnsSafeErrorClassification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewIntentHandler(nil, true, &configTestProvider{
		err: &ai.ProviderError{
			Type:      ai.ErrorTypeAuthentication,
			Retryable: false,
			Err:       errors.New("provider response containing sensitive diagnostic"),
		},
	}, nil)

	response := performIntentConfigTest(handler)
	if response.Code != common.CodeSuccess {
		t.Fatalf("response code = %d", response.Code)
	}
	if response.Data.Available || response.Data.ErrorType != ai.ErrorTypeAuthentication {
		t.Fatalf("unexpected response: %+v", response.Data)
	}
	if response.Data.Message != "模型服务鉴权失败，请检查 API Key" {
		t.Fatalf("unexpected safe message: %q", response.Data.Message)
	}
}

func TestIntentConfigTestDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := performIntentConfigTest(NewIntentHandler(nil, false, nil, nil))
	if response.Data.Available || response.Data.ErrorType != ai.ErrorTypeConfig {
		t.Fatalf("unexpected response: %+v", response.Data)
	}
}

func performIntentConfigTest(handler *IntentHandler) struct {
	Code int                      `json:"code"`
	Data IntentConfigTestResponse `json:"data"`
} {
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/crm/intent/config/test", nil)
	handler.ConfigTest(context)

	var response struct {
		Code int                      `json:"code"`
		Data IntentConfigTestResponse `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		panic(err)
	}
	return response
}
