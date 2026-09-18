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
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"intentLevel\":\"medium\",\"intentScore\":66,\"confidence\":0.8,\"summary\":\"有明确兴趣\",\"needs\":[\"客户管理\"],\"painPoints\":[],\"budget\":\"未明确\",\"purchaseTimeline\":\"未明确\",\"decisionRole\":\"未明确\",\"risks\":[],\"nextAction\":\"安排演示\",\"suggestedNextAt\":null}"}}]}`))
	}))
	defer server.Close()

	provider := NewProvider(config.LLMConfig{
		BaseURL:     server.URL + "/v1",
		APIKey:      "test-key",
		Model:       "test-model",
		Timeout:     time.Second,
		MaxTokens:   100,
		Temperature: 0.2,
	})
	result, err := provider.AnalyzeCustomerIntent(context.Background(), IntentInput{})
	if err != nil {
		t.Fatalf("AnalyzeCustomerIntent() error = %v", err)
	}
	if result.IntentLevel != "medium" || result.IntentScore == nil || *result.IntentScore != 66 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
