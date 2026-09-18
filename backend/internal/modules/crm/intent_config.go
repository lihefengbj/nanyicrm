package crm

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lihefengbj/nanyicrm/backend/internal/ai"
	"github.com/lihefengbj/nanyicrm/backend/internal/common"
)

const intentConfigTestTimeout = 45 * time.Second

type IntentConfigTestResponse struct {
	Available        bool      `json:"available"`
	Provider         string    `json:"provider"`
	ConfiguredModel  string    `json:"configuredModel"`
	ActualModel      string    `json:"actualModel"`
	ConfigVersion    string    `json:"configVersion"`
	AdapterVersion   string    `json:"adapterVersion"`
	StructuredOutput bool      `json:"structuredOutput"`
	InputTokens      int       `json:"inputTokens"`
	OutputTokens     int       `json:"outputTokens"`
	TotalTokens      int       `json:"totalTokens"`
	LatencyMillis    int64     `json:"latencyMillis"`
	ErrorType        string    `json:"errorType,omitempty"`
	Retryable        bool      `json:"retryable"`
	Message          string    `json:"message"`
	TestedAt         time.Time `json:"testedAt"`
}

// ConfigTest verifies the active LLM configuration with a fixed, non-customer
// sample. It intentionally does not return the API key, endpoint, prompt,
// provider response body, or raw provider error.
//
// @Summary  测试AI意向模型配置
// @Tags     CRM-客户意向
// @Description 仅管理员可用；需要权限：crm:intent:config
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/intent/config/test [post]
func (h *IntentHandler) ConfigTest(c *gin.Context) {
	response := IntentConfigTestResponse{
		TestedAt: time.Now(),
		Message:  "模型配置不可用",
	}
	if h.provider != nil {
		response.Provider = h.provider.Name()
		response.ConfiguredModel = h.provider.Model()
	}
	if !h.enabled || h.provider == nil {
		response.ErrorType = ai.ErrorTypeConfig
		response.Message = "LLM 未启用或 Provider 未初始化"
		common.OK(c, response)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), intentConfigTestTimeout)
	defer cancel()
	startedAt := time.Now()
	analysis, err := h.provider.AnalyzeCustomerIntent(ctx, intentConfigTestInput())
	response.LatencyMillis = time.Since(startedAt).Milliseconds()
	if err != nil {
		response.ErrorType, response.Retryable = ai.ErrorInfo(err)
		response.Message = intentConfigTestErrorMessage(response.ErrorType)
		common.OK(c, response)
		return
	}
	if analysis == nil {
		response.ErrorType = ai.ErrorTypeEmptyResponse
		response.Message = intentConfigTestErrorMessage(response.ErrorType)
		common.OK(c, response)
		return
	}
	response.ActualModel = analysis.Metadata.ActualModel
	if response.ActualModel == "" {
		response.ActualModel = h.provider.Model()
	}
	response.ConfigVersion = analysis.Metadata.ConfigVersion
	response.AdapterVersion = analysis.Metadata.AdapterVersion
	response.InputTokens = analysis.Metadata.InputTokens
	response.OutputTokens = analysis.Metadata.OutputTokens
	response.TotalTokens = analysis.Metadata.TotalTokens
	if err := validateIntentResult(analysis.Result); err != nil {
		response.ErrorType = ai.ErrorTypeResultValidation
		response.Message = intentConfigTestErrorMessage(response.ErrorType)
		common.OK(c, response)
		return
	}

	response.Available = true
	response.StructuredOutput = true
	response.Message = "模型配置测试通过"
	common.OK(c, response)
}

func intentConfigTestInput() ai.IntentInput {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	referenceTime := time.Now().In(location)
	return ai.IntentInput{
		Customer: ai.CustomerContext{
			Name:     "配置测试样本客户",
			Industry: "企业服务",
			Source:   "系统固定样本",
			Level:    "B",
			Status:   1,
			Remark:   "客户希望了解客户管理系统，并计划近期安排产品演示。",
		},
		Contacts: []ai.ContactContext{{
			Name:     "测试联系人",
			Position: "业务负责人",
		}},
		FollowUps: []ai.FollowUpContext{{
			Type:      1,
			Content:   "客户已确认有客户管理需求，希望进一步了解实施周期和产品能力。",
			CreatedAt: referenceTime.Add(-24 * time.Hour),
		}},
		ReferenceTime: referenceTime,
		Timezone:      "Asia/Shanghai",
		InputVersion:  intentInputVersion,
	}
}

func intentConfigTestErrorMessage(errorType string) string {
	switch errorType {
	case ai.ErrorTypeTimeout:
		return "模型请求超时"
	case ai.ErrorTypeNetwork:
		return "无法连接模型服务"
	case ai.ErrorTypeRateLimit:
		return "模型服务当前触发限流"
	case ai.ErrorTypeAuthentication:
		return "模型服务鉴权失败，请检查 API Key"
	case ai.ErrorTypeModelNotFound:
		return "配置的模型不存在或当前账号无权访问"
	case ai.ErrorTypeInvalidRequest:
		return "模型能力参数与当前模型不兼容"
	case ai.ErrorTypeResponseDecode, ai.ErrorTypeResultDecode, ai.ErrorTypeEmptyResponse:
		return "模型响应无法解析为结构化结果"
	case ai.ErrorTypeResultValidation:
		return "模型返回结果未通过意向结构校验"
	case ai.ErrorTypeConfig:
		return "模型配置不完整"
	default:
		return "模型服务返回异常"
	}
}
