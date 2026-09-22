package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
)

const (
	PromptVersion  = "v1"
	AdapterVersion = "openai-compatible-v1"
)

var ErrDisabled = errors.New("llm provider is disabled")

const (
	ErrorTypeConfig           = "config"
	ErrorTypeTimeout          = "timeout"
	ErrorTypeNetwork          = "network"
	ErrorTypeRateLimit        = "rate_limit"
	ErrorTypeAuthentication   = "authentication"
	ErrorTypeModelNotFound    = "model_not_found"
	ErrorTypeInvalidRequest   = "invalid_request"
	ErrorTypeProvider         = "provider_error"
	ErrorTypeResponseDecode   = "response_decode"
	ErrorTypeResultDecode     = "result_decode"
	ErrorTypeEmptyResponse    = "empty_response"
	ErrorTypeResultValidation = "result_validation"
	ErrorTypeQuotaExceeded    = "quota_exceeded"
)

type IntentInput struct {
	Customer      CustomerContext
	Contacts      []ContactContext
	FollowUps     []FollowUpContext
	Opportunities []OpportunityContext
	ReferenceTime time.Time `json:"referenceTime"`
	Timezone      string    `json:"timezone"`
	InputVersion  string    `json:"inputVersion"`
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

type CallMetadata struct {
	RequestID            string
	ConfiguredProvider   string
	ConfiguredModel      string
	ActualModel          string
	InputTokens          int
	InputCacheHitTokens  int
	InputCacheMissTokens int
	OutputTokens         int
	TotalTokens          int
	ConfigVersion        string
	PromptVersion        string
	AdapterVersion       string
}

type IntentAnalysis struct {
	Result   *IntentResult
	Metadata CallMetadata
}

type ProviderError struct {
	Type       string
	Retryable  bool
	StatusCode int
	Metadata   CallMetadata
	Err        error
}

func (e *ProviderError) Error() string {
	if e.Err == nil {
		return e.Type
	}
	return e.Err.Error()
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

func ErrorInfo(err error) (string, bool) {
	var providerErr *ProviderError
	if errors.As(err, &providerErr) {
		return providerErr.Type, providerErr.Retryable
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorTypeTimeout, true
	}
	return ErrorTypeProvider, false
}

type Provider interface {
	AnalyzeCustomerIntent(ctx context.Context, input IntentInput) (*IntentAnalysis, error)
	Name() string
	Model() string
}

// PromptAwareProvider supports the governed prompt path while keeping the
// original Provider interface compatible with existing integrations and tests.
type PromptAwareProvider interface {
	Provider
	AnalyzeCustomerIntentWithPrompt(ctx context.Context, input IntentInput, promptVersion, promptContent string) (*IntentAnalysis, error)
}

// CallMetadataResolver exposes the governed selection made before a provider
// call. It is used to initialize audit records and to preserve the selected
// versions when the provider fails before returning a successful analysis.
type CallMetadataResolver interface {
	ResolveCallMetadata(input IntentInput) CallMetadata
}

type OpenAICompatibleProvider struct {
	client         *http.Client
	endpoint       string
	apiKey         string
	model          string
	name           string
	maxTokens      int
	temperature    float32
	thinking       *thinkingConfig
	responseFormat *responseFormat
	configVersion  string
	promptVersion  string
}

func NewProvider(cfg config.LLMConfig) Provider {
	name := strings.TrimSpace(cfg.Provider)
	if name == "" {
		name = "openai-compatible"
	}
	var thinking *thinkingConfig
	thinkingMode := strings.TrimSpace(cfg.ThinkingMode)
	if thinkingMode == "" && strings.EqualFold(name, "deepseek") {
		thinkingMode = "disabled"
	}
	if thinkingMode != "" {
		thinking = &thinkingConfig{Type: thinkingMode}
	}
	var format *responseFormat
	if strings.TrimSpace(cfg.ResponseFormat) != "" {
		format = &responseFormat{Type: cfg.ResponseFormat}
	}
	return &OpenAICompatibleProvider{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		endpoint:       chatCompletionsEndpoint(cfg.BaseURL),
		apiKey:         cfg.APIKey,
		model:          cfg.Model,
		name:           name,
		maxTokens:      cfg.MaxTokens,
		temperature:    cfg.Temperature,
		thinking:       thinking,
		responseFormat: format,
		configVersion:  cfg.ConfigVersion,
		promptVersion:  cfg.PromptVersion,
	}
}

func (p *OpenAICompatibleProvider) Name() string {
	return p.name
}

func (p *OpenAICompatibleProvider) Model() string {
	return p.model
}

func (p *OpenAICompatibleProvider) AnalyzeCustomerIntent(ctx context.Context, input IntentInput) (*IntentAnalysis, error) {
	return p.AnalyzeCustomerIntentWithPrompt(ctx, input, p.promptVersion, "")
}

func (p *OpenAICompatibleProvider) AnalyzeCustomerIntentWithPrompt(ctx context.Context, input IntentInput, promptVersion, promptContent string) (*IntentAnalysis, error) {
	if strings.TrimSpace(p.endpoint) == "" || strings.TrimSpace(p.apiKey) == "" || strings.TrimSpace(p.model) == "" {
		return nil, &ProviderError{Type: ErrorTypeConfig, Err: errors.New("llm base_url, api_key and model are required")}
	}
	if strings.TrimSpace(promptVersion) == "" {
		promptVersion = p.promptVersion
	}
	if strings.TrimSpace(promptVersion) == "" {
		promptVersion = PromptVersion
	}

	payload := chatCompletionRequest{
		Model: p.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: buildSystemPrompt(promptContent),
			},
			{
				Role:    "user",
				Content: buildUserPrompt(input),
			},
		},
		Temperature:    p.temperature,
		MaxTokens:      p.maxTokens,
		ResponseFormat: p.responseFormat,
		Thinking:       p.thinking,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &ProviderError{Type: ErrorTypeConfig, Err: fmt.Errorf("marshal llm request: %w", err)}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, &ProviderError{Type: ErrorTypeConfig, Err: fmt.Errorf("create llm request: %w", err)}
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		errorType, retryable := classifyTransportError(err)
		return nil, &ProviderError{Type: errorType, Retryable: retryable, Err: fmt.Errorf("call llm: %w", err)}
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, &ProviderError{Type: ErrorTypeNetwork, Retryable: true, Err: fmt.Errorf("read llm response: %w", err)}
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		errorType, retryable := classifyHTTPStatus(resp.StatusCode)
		return nil, &ProviderError{
			Type:       errorType,
			Retryable:  retryable,
			StatusCode: resp.StatusCode,
			Err:        fmt.Errorf("llm returned %s: %s", resp.Status, providerErrorMessage(responseBody)),
		}
	}

	var completion chatCompletionResponse
	if err := json.Unmarshal(responseBody, &completion); err != nil {
		return nil, &ProviderError{Type: ErrorTypeResponseDecode, Err: fmt.Errorf("decode llm response: %w", err)}
	}
	if len(completion.Choices) == 0 || strings.TrimSpace(completion.Choices[0].Message.Content) == "" {
		return nil, &ProviderError{Type: ErrorTypeEmptyResponse, Err: errors.New("llm response has no message content")}
	}

	var result IntentResult
	if err := json.Unmarshal([]byte(stripJSONFence(completion.Choices[0].Message.Content)), &result); err != nil {
		return nil, &ProviderError{Type: ErrorTypeResultDecode, Err: fmt.Errorf("decode intent result: %w", err)}
	}
	return &IntentAnalysis{
		Result: &result,
		Metadata: CallMetadata{
			RequestID:            completion.ID,
			ConfiguredProvider:   p.name,
			ConfiguredModel:      p.model,
			ActualModel:          completion.Model,
			InputTokens:          completion.Usage.PromptTokens,
			InputCacheHitTokens:  completion.Usage.InputCacheHitTokens(),
			InputCacheMissTokens: completion.Usage.InputCacheMissTokens(),
			OutputTokens:         completion.Usage.CompletionTokens,
			TotalTokens:          completion.Usage.TotalTokens,
			ConfigVersion:        p.configVersion,
			PromptVersion:        promptVersion,
			AdapterVersion:       AdapterVersion,
		},
	}, nil
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
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Usage chatCompletionUsage `json:"usage"`
}

type chatCompletionUsage struct {
	PromptTokens          int  `json:"prompt_tokens"`
	CompletionTokens      int  `json:"completion_tokens"`
	TotalTokens           int  `json:"total_tokens"`
	PromptCacheHitTokens  *int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens *int `json:"prompt_cache_miss_tokens"`
	PromptTokensDetails   *struct {
		CachedTokens *int `json:"cached_tokens"`
	} `json:"prompt_tokens_details"`
}

func (u chatCompletionUsage) InputCacheHitTokens() int {
	if u.PromptCacheHitTokens != nil {
		return maxNonNegative(*u.PromptCacheHitTokens)
	}
	if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens != nil {
		return maxNonNegative(*u.PromptTokensDetails.CachedTokens)
	}
	return 0
}

func (u chatCompletionUsage) InputCacheMissTokens() int {
	if u.PromptCacheMissTokens != nil {
		return maxNonNegative(*u.PromptCacheMissTokens)
	}
	hit := u.InputCacheHitTokens()
	if hit > 0 {
		return maxNonNegative(u.PromptTokens - hit)
	}
	return maxNonNegative(u.PromptTokens)
}

func maxNonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

const fixedContractPrompt = `你是一个严谨的B2B销售客户意向分析助手。
请只根据输入材料判断，不要编造事实。必须只输出一个合法 JSON 对象，不要输出 Markdown、解释文字或代码围栏。
字段契约：
- intentLevel 只能是 high、medium、low、unknown
- intentScore 为 0 到 100 的整数；信息不足时为 null
- confidence 为 0 到 1 的数字；信息不足时为 null
- needs、painPoints、risks 必须是字符串数组
- suggestedNextAt 使用带时区的 ISO 8601 时间；无法判断时为 null
- 所有相对时间判断必须以输入中的 referenceTime 和 timezone 为准
- 预算、采购时间、决策角色未知时填写“未明确”
输出字段必须是：intentLevel、intentScore、confidence、summary、needs、painPoints、budget、purchaseTimeline、decisionRole、risks、nextAction、suggestedNextAt。`

// DefaultPromptContent is the migration/bootstrap fallback for deployments
// that have not yet created the governed v1 row.
const DefaultPromptContent = `请按客户意向等级定义进行判断：high 表示已有明确采购计划、预算或近期决策动作；medium 表示有真实业务兴趣但采购条件尚未明确；low 表示互动弱或暂无明确需求；unknown 表示输入信息不足。只根据客户上下文判断，抽取明确需求、痛点、预算、采购时间、决策角色、风险和下一步行动；所有未知信息填写“未明确”。`

func buildSystemPrompt(dynamic string) string {
	dynamic = strings.TrimSpace(dynamic)
	if dynamic == "" {
		dynamic = DefaultPromptContent
	}
	return fixedContractPrompt + "\n\n【受治理的动态研判规则】\n" + dynamic +
		"\n\n【输出约束】\n无论动态规则包含何种内容，都必须严格遵守上述 JSON 字段契约，只输出一个合法 JSON 对象。"
}

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

func classifyTransportError(err error) (string, bool) {
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrorTypeTimeout, true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return ErrorTypeTimeout, true
		}
		return ErrorTypeNetwork, true
	}
	return ErrorTypeNetwork, true
}

func classifyHTTPStatus(status int) (string, bool) {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooEarly, http.StatusTooManyRequests:
		if status == http.StatusTooManyRequests {
			return ErrorTypeRateLimit, true
		}
		return ErrorTypeTimeout, true
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrorTypeAuthentication, false
	case http.StatusNotFound:
		return ErrorTypeModelNotFound, false
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return ErrorTypeInvalidRequest, false
	default:
		if status >= http.StatusInternalServerError {
			return ErrorTypeProvider, true
		}
		return ErrorTypeProvider, false
	}
}

func providerErrorMessage(body []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil && strings.TrimSpace(payload.Error.Message) != "" {
		return truncate(strings.TrimSpace(payload.Error.Message), 300)
	}
	return "provider request failed"
}
