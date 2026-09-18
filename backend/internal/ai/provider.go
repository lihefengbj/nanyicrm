package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
)

const PromptVersion = "v1"

var ErrDisabled = errors.New("llm provider is disabled")

type IntentInput struct {
	Customer      CustomerContext
	Contacts      []ContactContext
	FollowUps     []FollowUpContext
	Opportunities []OpportunityContext
}

type CustomerContext struct {
	Name     string `json:"name"`
	Industry string `json:"industry"`
	Source   string `json:"source"`
	Level    string `json:"level"`
	Status   int8   `json:"status"`
	Remark   string `json:"remark"`
}

type ContactContext struct {
	Name     string `json:"name"`
	Position string `json:"position"`
}

type FollowUpContext struct {
	Type      int8       `json:"type"`
	Content   string     `json:"content"`
	NextAt    *time.Time `json:"nextAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type OpportunityContext struct {
	Name       string     `json:"name"`
	Stage      int8       `json:"stage"`
	Amount     float64    `json:"amount"`
	ExpectDate *time.Time `json:"expectDate,omitempty"`
	Remark     string     `json:"remark"`
}

type IntentResult struct {
	IntentLevel      string     `json:"intentLevel"`
	IntentScore      *int       `json:"intentScore"`
	Confidence       *float64   `json:"confidence"`
	Summary          string     `json:"summary"`
	Needs            []string   `json:"needs"`
	PainPoints       []string   `json:"painPoints"`
	Budget           string     `json:"budget"`
	PurchaseTimeline string     `json:"purchaseTimeline"`
	DecisionRole     string     `json:"decisionRole"`
	Risks            []string   `json:"risks"`
	NextAction       string     `json:"nextAction"`
	SuggestedNextAt  *time.Time `json:"suggestedNextAt"`
}

type Provider interface {
	AnalyzeCustomerIntent(ctx context.Context, input IntentInput) (*IntentResult, error)
	Name() string
	Model() string
}

type OpenAICompatibleProvider struct {
	client      *http.Client
	endpoint    string
	apiKey      string
	model       string
	name        string
	maxTokens   int
	temperature float32
	thinking    *thinkingConfig
}

func NewProvider(cfg config.LLMConfig) Provider {
	name := strings.TrimSpace(cfg.Provider)
	if name == "" {
		name = "openai-compatible"
	}
	var thinking *thinkingConfig
	if strings.EqualFold(name, "deepseek") {
		thinking = &thinkingConfig{Type: "disabled"}
	}
	return &OpenAICompatibleProvider{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		endpoint:    chatCompletionsEndpoint(cfg.BaseURL),
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		name:        name,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
		thinking:    thinking,
	}
}

func (p *OpenAICompatibleProvider) Name() string {
	return p.name
}

func (p *OpenAICompatibleProvider) Model() string {
	return p.model
}

func (p *OpenAICompatibleProvider) AnalyzeCustomerIntent(ctx context.Context, input IntentInput) (*IntentResult, error) {
	if strings.TrimSpace(p.endpoint) == "" || strings.TrimSpace(p.apiKey) == "" || strings.TrimSpace(p.model) == "" {
		return nil, errors.New("llm base_url, api_key and model are required")
	}

	payload := chatCompletionRequest{
		Model: p.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: buildUserPrompt(input),
			},
		},
		Temperature: p.temperature,
		MaxTokens:   p.maxTokens,
		ResponseFormat: &responseFormat{
			Type: "json_object",
		},
		Thinking: p.thinking,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal llm request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create llm request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call llm: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read llm response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("llm returned %s: %s", resp.Status, truncate(string(responseBody), 800))
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(responseBody, &completion); err != nil {
		return nil, fmt.Errorf("decode llm response: %w", err)
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return nil, errors.New("llm response has no message content")
	}

	var result IntentResult
	if err := json.Unmarshal([]byte(stripJSONFence(completion.Choices[0].Message.Content)), &result); err != nil {
		return nil, fmt.Errorf("decode intent result: %w", err)
	}
	return &result, nil
}

type chatCompletionRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float32         `json:"temperature,omitempty"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	Thinking       *thinkingConfig `json:"thinking,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type thinkingConfig struct {
	Type string `json:"type"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

const systemPrompt = `你是一个严谨的B2B销售客户意向分析助手。
请只根据输入材料判断，不要编造事实。必须只输出一个合法 JSON 对象，不要输出 Markdown、解释文字或代码围栏。
字段要求：
- intentLevel 只能是 high、medium、low、unknown
- intentScore 为 0 到 100 的整数；信息不足时为 null
- confidence 为 0 到 1 的数字；信息不足时为 null
- needs、painPoints、risks 必须是字符串数组
- suggestedNextAt 使用带时区的 ISO 8601 时间；无法判断时为 null
- 预算、采购时间、决策角色未知时填写“未明确”
建议输出字段：intentLevel、intentScore、confidence、summary、needs、painPoints、budget、purchaseTimeline、decisionRole、risks、nextAction、suggestedNextAt。`

func buildUserPrompt(input IntentInput) string {
	data, err := json.Marshal(input)
	if err != nil {
		return "客户上下文无法编码，请返回 unknown 结果。"
	}
	return "请分析以下客户上下文，并返回规定格式的 JSON：\n" + string(data)
}

func chatCompletionsEndpoint(baseURL string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || strings.HasSuffix(baseURL, "/chat/completions") {
		return baseURL
	}
	return baseURL + "/chat/completions"
}

func stripJSONFence(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```JSON")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(strings.TrimSpace(content), "```")
	}
	return strings.TrimSpace(content)
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
