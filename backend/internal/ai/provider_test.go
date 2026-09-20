package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
)

func TestChatCompletionsEndpoint(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "v1 base", base: "https://example.com/v1", want: "https://example.com/v1/chat/completions"},
		{name: "full endpoint", base: "https://example.com/v1/chat/completions", want: "https://example.com/v1/chat/completions"},
		{name: "trailing slash", base: "https://example.com/v1/", want: "https://example.com/v1/chat/completions"},
		{name: "empty", base: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chatCompletionsEndpoint(tt.base); got != tt.want {
				t.Fatalf("chatCompletionsEndpoint(%q) = %q, want %q", tt.base, got, tt.want)
			}
		})
	}
}

func TestNewProvider(t *testing.T) {
	provider := NewProvider(config.LLMConfig{Timeout: 1})
	if provider == nil {
		t.Fatal("NewProvider returned nil")
	}
	if provider.Name() != "openai-compatible" {
		t.Fatalf("provider name = %q", provider.Name())
	}
}

func TestOpenAICompatibleProviderAnalyze(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"req-123","model":"actual-model","usage":{"prompt_tokens":30,"prompt_cache_hit_tokens":12,"prompt_cache_miss_tokens":18,"completion_tokens":20,"total_tokens":50},"choices":[{"message":{"content":"{\"intentLevel\":\"medium\",\"intentScore\":66,\"confidence\":0.8,\"summary\":\"有明确兴趣\",\"needs\":[\"客户管理\"],\"painPoints\":[],\"budget\":\"未明确\",\"purchaseTimeline\":\"未明确\",\"decisionRole\":\"未明确\",\"risks\":[],\"nextAction\":\"安排演示\",\"suggestedNextAt\":null}"}}]}`))
	}))
	defer server.Close()

	provider := NewProvider(config.LLMConfig{
		BaseURL:        server.URL + "/v1",
		APIKey:         "test-key",
		Model:          "test-model",
		Timeout:        time.Second,
		MaxTokens:      100,
		Temperature:    0.2,
		ConfigVersion:  "test-v1",
		ResponseFormat: "json_object",
	})
	analysis, err := provider.AnalyzeCustomerIntent(context.Background(), IntentInput{})
	if err != nil {
		t.Fatalf("AnalyzeCustomerIntent() error = %v", err)
	}
	result := analysis.Result
	if result.IntentLevel != "medium" || result.IntentScore == nil || *result.IntentScore != 66 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if analysis.Metadata.RequestID != "req-123" || analysis.Metadata.ActualModel != "actual-model" ||
		analysis.Metadata.TotalTokens != 50 ||
		analysis.Metadata.InputCacheHitTokens != 12 ||
		analysis.Metadata.InputCacheMissTokens != 18 ||
		analysis.Metadata.ConfigVersion != "test-v1" {
		t.Fatalf("unexpected metadata: %+v", analysis.Metadata)
	}
}

func TestChatCompletionUsagePromptTokensDetails(t *testing.T) {
	usage := chatCompletionUsage{
		PromptTokens: 30,
		PromptTokensDetails: &struct {
			CachedTokens *int `json:"cached_tokens"`
		}{CachedTokens: intPointer(12)},
	}
	if usage.InputCacheHitTokens() != 12 || usage.InputCacheMissTokens() != 18 {
		t.Fatalf("unexpected cache usage: hit=%d miss=%d", usage.InputCacheHitTokens(), usage.InputCacheMissTokens())
	}
}

func intPointer(value int) *int {
	return &value
}

func TestClassifyHTTPStatus(t *testing.T) {
	tests := []struct {
		status    int
		errorType string
		retryable bool
	}{
		{status: http.StatusTooManyRequests, errorType: ErrorTypeRateLimit, retryable: true},
		{status: http.StatusUnauthorized, errorType: ErrorTypeAuthentication, retryable: false},
		{status: http.StatusNotFound, errorType: ErrorTypeModelNotFound, retryable: false},
		{status: http.StatusBadRequest, errorType: ErrorTypeInvalidRequest, retryable: false},
		{status: http.StatusServiceUnavailable, errorType: ErrorTypeProvider, retryable: true},
	}
	for _, tt := range tests {
		errorType, retryable := classifyHTTPStatus(tt.status)
		if errorType != tt.errorType || retryable != tt.retryable {
			t.Fatalf("status %d = (%q, %v), want (%q, %v)", tt.status, errorType, retryable, tt.errorType, tt.retryable)
		}
	}
}
