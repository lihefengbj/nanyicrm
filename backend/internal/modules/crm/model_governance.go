package crm

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/ai"
	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

const (
	modelConfigDraft    = "draft"
	modelConfigApproved = "approved"
	modelConfigActive   = "active"
	modelConfigCanary   = "canary"
	modelConfigRetired  = "retired"
)

// GovernedProvider resolves the active model configuration for every call.
// A canary is selected deterministically from the input hash, so retries for
// the same customer stay on the same model during a rollout.
type GovernedProvider struct {
	db   *gorm.DB
	base config.LLMConfig
}

func NewGovernedProvider(db *gorm.DB, base config.LLMConfig) ai.Provider {
	return &GovernedProvider{db: db, base: base}
}

func (p *GovernedProvider) Name() string {
	cfg := p.selectedConfig(ai.IntentInput{})
	return cfg.Provider
}

func (p *GovernedProvider) Model() string {
	cfg := p.selectedConfig(ai.IntentInput{})
	return cfg.Model
}

func (p *GovernedProvider) AnalyzeCustomerIntent(ctx context.Context, input ai.IntentInput) (*ai.IntentAnalysis, error) {
	cfg := p.selectedConfig(input)
	return ai.NewProvider(cfg).AnalyzeCustomerIntent(ctx, input)
}

func (p *GovernedProvider) selectedConfig(input ai.IntentInput) config.LLMConfig {
	if p.db == nil {
		return p.base
	}
	var active model.SysAIModelConfig
	if err := p.db.Where("status = ?", modelConfigActive).Order("id DESC").First(&active).Error; err != nil {
		return p.base
	}
	selected := active
	var canary model.SysAIModelConfig
	if err := p.db.Where("status = ? AND quality_passed = ?", modelConfigCanary, true).
		Order("id DESC").First(&canary).Error; err == nil && canary.CanaryPercent > 0 {
		data, _ := json.Marshal(input)
		sum := sha256.Sum256(data)
		bucket := int(sum[0]) % 100
		if bucket < canary.CanaryPercent {
			selected = canary
		}
	}
	return modelConfigToLLM(p.base, selected)
}

func modelConfigToLLM(base config.LLMConfig, value model.SysAIModelConfig) config.LLMConfig {
	if value.Provider != "" {
		base.Provider = value.Provider
	}
	if value.BaseURL != "" {
		base.BaseURL = value.BaseURL
	}
	if value.Model != "" {
		base.Model = value.Model
	}
	if value.ConfigVersion != "" {
		base.ConfigVersion = value.ConfigVersion
	}
	if value.PromptVersion != "" {
		base.PromptVersion = value.PromptVersion
	}
	if value.ResponseFormat != "" {
		base.ResponseFormat = value.ResponseFormat
	}
	if value.ThinkingMode != "" {
		base.ThinkingMode = value.ThinkingMode
	}
	if value.MaxTokens > 0 {
		base.MaxTokens = value.MaxTokens
	}
	if value.Temperature >= 0 {
		base.Temperature = value.Temperature
	}
	return base
}

// EnsureDefaultModelConfig makes the deployment configuration auditable from
// the first request while preserving the environment-provided API key.
func EnsureDefaultModelConfig(db *gorm.DB, cfg config.LLMConfig) error {
	if db == nil || !cfg.Enabled {
		return nil
	}
	var active model.SysAIModelConfig
	if err := db.Where("status = ?", modelConfigActive).First(&active).Error; err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	entry := model.SysAIModelConfig{
		Name:           "environment-default",
		Provider:       cfg.Provider,
		BaseURL:        cfg.BaseURL,
		Model:          cfg.Model,
		ConfigVersion:  cfg.ConfigVersion,
		PromptVersion:  cfg.PromptVersion,
		ResponseFormat: cfg.ResponseFormat,
		ThinkingMode:   cfg.ThinkingMode,
		MaxTokens:      cfg.MaxTokens,
		Temperature:    cfg.Temperature,
		Status:         modelConfigActive,
		QualityPassed:  true,
		QualitySummary: "由部署配置初始化；后续切换必须通过固定样本质量门禁",
		ActivatedAt:    ptrTime(time.Now()),
	}
	if err := db.Create(&entry).Error; err != nil {
		var existing model.SysAIModelConfig
		if db.Where("config_version = ?", cfg.ConfigVersion).First(&existing).Error == nil {
			return db.Model(&existing).Updates(map[string]interface{}{
				"status":         modelConfigActive,
				"quality_passed": true,
				"activated_at":   time.Now(),
			}).Error
		}
		return err
	}
	return nil
}

type ModelConfigRequest struct {
	Name           string  `json:"name" binding:"required,max=64"`
	Provider       string  `json:"provider" binding:"required,max=64"`
	BaseURL        string  `json:"baseUrl" binding:"required,max=255"`
	Model          string  `json:"model" binding:"required,max=128"`
	ConfigVersion  string  `json:"configVersion" binding:"required,max=64"`
	PromptVersion  string  `json:"promptVersion" binding:"required,max=32"`
	ResponseFormat string  `json:"responseFormat"`
	ThinkingMode   string  `json:"thinkingMode"`
	MaxTokens      int     `json:"maxTokens"`
	Temperature    float32 `json:"temperature"`
}

type ModelConfigActionRequest struct {
	Reason         string `json:"reason"`
	CanaryPercent  int    `json:"canaryPercent"`
	TargetConfigID uint64 `json:"targetConfigId"`
}

type ModelConfigHandler struct {
	db   *gorm.DB
	base config.LLMConfig
	prod bool
}

func NewModelConfigHandler(db *gorm.DB, base config.LLMConfig, prod bool) *ModelConfigHandler {
	return &ModelConfigHandler{db: db, base: base, prod: prod}
}

func (h *ModelConfigHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)
	var total int64
	if err := h.db.Model(&model.SysAIModelConfig{}).Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var rows []model.SysAIModelConfig
	if err := h.db.Order("id DESC").Offset(page.Offset()).Limit(page.PageSize).Find(&rows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, rows, total, page.PageNum, page.PageSize)
}

func (h *ModelConfigHandler) Create(c *gin.Context) {
	var req ModelConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	parsedURL, err := url.Parse(req.BaseURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		common.FailMsg(c, common.CodeParamInvalid, "模型 BaseURL 不合法")
		return
	}
	if h.prod && parsedURL.Scheme != "https" {
		common.FailMsg(c, common.CodeParamInvalid, "生产模型 BaseURL 必须使用 HTTPS")
		return
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = h.base.MaxTokens
	}
	entry := model.SysAIModelConfig{
		Name: req.Name, Provider: req.Provider, BaseURL: req.BaseURL, Model: req.Model,
		ConfigVersion: req.ConfigVersion, PromptVersion: req.PromptVersion,
		ResponseFormat: req.ResponseFormat, ThinkingMode: req.ThinkingMode,
		MaxTokens: req.MaxTokens, Temperature: req.Temperature, Status: modelConfigDraft,
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIModelChange{
			ConfigID: entry.ID, ToConfigID: entry.ID, ActorID: middleware.CurrentUserID(c),
			Action: "create", Reason: "创建候选模型配置", CreatedAt: time.Now(),
		}).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, entry)
}

func (h *ModelConfigHandler) QualityGate(c *gin.Context) {
	entry, ok := h.find(c)
	if !ok {
		return
	}
	passed, summary, metrics := h.runQualityGate(c.Request.Context(), entry)
	now := time.Now()
	status := modelConfigDraft
	if passed {
		status = modelConfigApproved
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entry).Updates(map[string]interface{}{
			"status": status, "quality_passed": passed, "quality_summary": summary,
			"quality_metrics": metrics, "quality_checked_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIModelChange{
			ConfigID: entry.ID, ToConfigID: entry.ID, ActorID: middleware.CurrentUserID(c),
			Action: "quality_gate", QualitySummary: summary, CreatedAt: now,
		}).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	entry.Status, entry.QualityPassed, entry.QualitySummary = status, passed, summary
	entry.QualityMetrics, entry.QualityCheckedAt = metrics, &now
	common.OK(c, gin.H{"passed": passed, "config": entry})
}

func (h *ModelConfigHandler) Canary(c *gin.Context) {
	entry, ok := h.find(c)
	if !ok {
		return
	}
	var req ModelConfigActionRequest
	_ = c.ShouldBindJSON(&req)
	if !entry.QualityPassed {
		common.FailMsg(c, common.CodeParamInvalid, "模型必须先通过固定样本质量门禁")
		return
	}
	var active model.SysAIModelConfig
	if err := h.db.Where("status = ?", modelConfigActive).First(&active).Error; err == nil && active.ID == entry.ID {
		common.FailMsg(c, common.CodeParamInvalid, "当前激活版本不能再次设置为灰度")
		return
	}
	if req.CanaryPercent < 1 || req.CanaryPercent > 99 {
		common.FailMsg(c, common.CodeParamInvalid, "灰度比例必须为 1 到 99")
		return
	}
	now := time.Now()
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		var active model.SysAIModelConfig
		_ = tx.Where("status = ?", modelConfigActive).First(&active).Error
		if active.ID == entry.ID {
			return errors.New("active model cannot be canary")
		}
		if err := tx.Model(&model.SysAIModelConfig{}).
			Where("status = ? AND id <> ?", modelConfigCanary, entry.ID).
			Updates(map[string]interface{}{
				"status": modelConfigRetired, "canary_percent": 0,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&entry).Updates(map[string]interface{}{
			"status": modelConfigCanary, "canary_percent": req.CanaryPercent,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIModelChange{
			ConfigID: entry.ID, FromConfigID: active.ID, ToConfigID: entry.ID,
			ActorID: middleware.CurrentUserID(c), Action: "canary", Reason: req.Reason,
			QualitySummary: entry.QualitySummary, CreatedAt: now,
		}).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"status": modelConfigCanary, "canaryPercent": req.CanaryPercent})
}

func (h *ModelConfigHandler) Activate(c *gin.Context) {
	entry, ok := h.find(c)
	if !ok {
		return
	}
	var req ModelConfigActionRequest
	_ = c.ShouldBindJSON(&req)
	if !entry.QualityPassed {
		common.FailMsg(c, common.CodeParamInvalid, "模型必须先通过固定样本质量门禁")
		return
	}
	now := time.Now()
	var previous model.SysAIModelConfig
	if err := h.db.Where("status = ?", modelConfigActive).First(&previous).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		common.Fail(c, common.CodeDBError)
		return
	}
	if previous.ID == entry.ID {
		common.FailMsg(c, common.CodeParamInvalid, "当前版本已经是激活版本")
		return
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SysAIModelConfig{}).
			Where("status IN ?", []string{modelConfigActive, modelConfigCanary}).
			Updates(map[string]interface{}{
				"status": modelConfigRetired, "canary_percent": 0,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&entry).Updates(map[string]interface{}{
			"status": modelConfigActive, "canary_percent": 0, "activated_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIModelChange{
			ConfigID: entry.ID, FromConfigID: previous.ID, ToConfigID: entry.ID,
			ActorID: middleware.CurrentUserID(c), Action: "activate", Reason: req.Reason,
			QualitySummary: entry.QualitySummary, CreatedAt: now,
		}).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"status": modelConfigActive, "configId": entry.ID})
}

func (h *ModelConfigHandler) Rollback(c *gin.Context) {
	var req ModelConfigActionRequest
	_ = c.ShouldBindJSON(&req)
	var targetID = req.TargetConfigID
	if targetID == 0 {
		var change model.SysAIModelChange
		if err := h.db.Where("action = ? AND from_config_id > 0", "activate").
			Order("id DESC").First(&change).Error; err != nil {
			common.FailMsg(c, common.CodeRecordNotFound, "没有可回滚的模型版本")
			return
		}
		targetID = change.FromConfigID
	}
	var target model.SysAIModelConfig
	if err := h.db.First(&target, targetID).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return
	}
	if !target.QualityPassed {
		common.FailMsg(c, common.CodeParamInvalid, "回滚目标未通过质量门禁")
		return
	}
	var previous model.SysAIModelConfig
	_ = h.db.Where("status = ?", modelConfigActive).First(&previous).Error
	if previous.ID == target.ID {
		common.FailMsg(c, common.CodeParamInvalid, "回滚目标已经是当前激活版本")
		return
	}
	now := time.Now()
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SysAIModelConfig{}).
			Where("status IN ?", []string{modelConfigActive, modelConfigCanary}).
			Updates(map[string]interface{}{
				"status": modelConfigRetired, "canary_percent": 0,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&target).Updates(map[string]interface{}{
			"status": modelConfigActive, "canary_percent": 0, "activated_at": now,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIModelChange{
			ConfigID: target.ID, FromConfigID: previous.ID, ToConfigID: target.ID,
			ActorID: middleware.CurrentUserID(c), Action: "rollback", Reason: req.Reason,
			QualitySummary: target.QualitySummary, CreatedAt: now,
		}).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"status": modelConfigActive, "configId": target.ID})
}

func (h *ModelConfigHandler) Changes(c *gin.Context) {
	var rows []model.SysAIModelChange
	query := h.db.Order("id DESC").Limit(500)
	if id := strings.TrimSpace(c.Query("configId")); id != "" {
		query = query.Where("config_id = ?", id)
	}
	if err := query.Find(&rows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, rows)
}

func (h *ModelConfigHandler) find(c *gin.Context) (model.SysAIModelConfig, bool) {
	var entry model.SysAIModelConfig
	id := strings.TrimSpace(c.Param("id"))
	if err := h.db.First(&entry, id).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return entry, false
	}
	return entry, true
}

func (h *ModelConfigHandler) runQualityGate(ctx context.Context, entry model.SysAIModelConfig) (bool, string, string) {
	start := time.Now()
	provider := ai.NewProvider(modelConfigToLLM(h.base, entry))
	analysis, err := provider.AnalyzeCustomerIntent(ctx, intentConfigTestInput())
	metrics := map[string]interface{}{"latencyMillis": time.Since(start).Milliseconds()}
	if err != nil {
		metrics["errorType"], metrics["retryable"] = ai.ErrorInfo(err)
		body, _ := json.Marshal(metrics)
		return false, "模型调用失败，未通过质量门禁", string(body)
	}
	if analysis == nil || analysis.Result == nil {
		body, _ := json.Marshal(metrics)
		return false, "模型返回空结果，未通过质量门禁", string(body)
	}
	metrics["actualModel"] = analysis.Metadata.ActualModel
	metrics["totalTokens"] = analysis.Metadata.TotalTokens
	if err := validateIntentResult(analysis.Result); err != nil {
		metrics["errorType"] = ai.ErrorTypeResultValidation
		body, _ := json.Marshal(metrics)
		return false, "模型结构化结果校验失败，未通过质量门禁", string(body)
	}
	metrics["structuredOutput"] = true
	body, _ := json.Marshal(metrics)
	return true, fmt.Sprintf("固定样本通过；延迟 %dms，Token %d", metrics["latencyMillis"], analysis.Metadata.TotalTokens), string(body)
}

func ptrTime(value time.Time) *time.Time { return &value }
