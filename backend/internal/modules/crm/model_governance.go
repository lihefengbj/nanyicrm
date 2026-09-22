package crm

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
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
	"github.com/lihefengbj/nanyicrm/backend/internal/security"
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
	cfg, err := p.resolveConfig(input)
	if err != nil {
		return nil, &ai.ProviderError{Type: ai.ErrorTypeConfig, Err: err}
	}
	provider := ai.NewProvider(cfg)
	if promptProvider, ok := provider.(ai.PromptAwareProvider); ok {
		promptVersion, promptContent := p.resolvePrompt(input, cfg.PromptVersion)
		analysis, err := promptProvider.AnalyzeCustomerIntentWithPrompt(ctx, input, promptVersion, promptContent)
		if err != nil {
			var providerErr *ai.ProviderError
			if errors.As(err, &providerErr) {
				providerErr.Metadata = ai.CallMetadata{
					ConfiguredProvider: cfg.Provider,
					ConfiguredModel:    cfg.Model,
					ConfigVersion:      cfg.ConfigVersion,
					PromptVersion:      promptVersion,
					AdapterVersion:     ai.AdapterVersion,
				}
			}
		}
		return analysis, err
	}
	return provider.AnalyzeCustomerIntent(ctx, input)
}

func (p *GovernedProvider) ResolveCallMetadata(input ai.IntentInput) ai.CallMetadata {
	cfg, err := p.resolveConfig(input)
	if err != nil {
		cfg = p.base
	}
	promptVersion, _ := p.resolvePrompt(input, cfg.PromptVersion)
	return ai.CallMetadata{
		ConfiguredProvider: cfg.Provider,
		ConfiguredModel:    cfg.Model,
		ConfigVersion:      cfg.ConfigVersion,
		PromptVersion:      promptVersion,
		AdapterVersion:     ai.AdapterVersion,
	}
}

func (p *GovernedProvider) selectedConfig(input ai.IntentInput) config.LLMConfig {
	cfg, err := p.resolveConfig(input)
	if err != nil {
		return p.base
	}
	return cfg
}

func (p *GovernedProvider) resolveConfig(input ai.IntentInput) (config.LLMConfig, error) {
	if p.db == nil {
		return p.base, nil
	}
	var active model.SysAIModelConfig
	if err := p.db.Where("status = ?", modelConfigActive).Order("id DESC").First(&active).Error; err != nil {
		return p.base, nil
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
	return resolveModelConfig(p.db, p.base, selected)
}

// resolvePrompt selects the active prompt and, when present, its canary using
// the same stable input-hash strategy as model canaries. The configured model
// prompt version is retained as a safe fallback for older deployments.
func (p *GovernedProvider) resolvePrompt(input ai.IntentInput, fallbackVersion string) (string, string) {
	if p.db == nil {
		if fallbackVersion == "" {
			fallbackVersion = ai.PromptVersion
		}
		return fallbackVersion, ai.DefaultPromptContent
	}
	var active model.SysAIPrompt
	if err := p.db.Where("status = ?", promptStatusActive).Order("id DESC").First(&active).Error; err != nil {
		if fallbackVersion != "" {
			var configured model.SysAIPrompt
			if p.db.Where("version = ? AND quality_passed = ?", fallbackVersion, true).First(&configured).Error == nil {
				return configured.Version, configured.Content
			}
		}
		return ai.PromptVersion, ai.DefaultPromptContent
	}
	selected := active
	var canary model.SysAIPrompt
	if err := p.db.Where("status = ? AND quality_passed = ?", promptStatusCanary, true).
		Order("id DESC").First(&canary).Error; err == nil && canary.CanaryPercent > 0 {
		data, _ := json.Marshal(input)
		sum := sha256.Sum256(data)
		if int(sum[0])%100 < canary.CanaryPercent {
			selected = canary
		}
	}
	return selected.Version, selected.Content
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
// the first request. Its API key may be empty when all active configurations
// bind encrypted database-managed credentials.
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
	CredentialID   uint64  `json:"credentialId"`
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

type ModelConfigCredentialRequest struct {
	CredentialID uint64 `json:"credentialId"`
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
	if err := validateCredentialBinding(h.db, req.CredentialID); err != nil {
		if errors.Is(err, ErrCredentialUnavailable) {
			common.FailMsg(c, common.CodeParamInvalid, err.Error())
		} else {
			common.Fail(c, common.CodeDBError)
		}
		return
	}
	entry := model.SysAIModelConfig{
		Name: req.Name, Provider: req.Provider, BaseURL: req.BaseURL, Model: req.Model,
		CredentialID:  req.CredentialID,
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

func (h *ModelConfigHandler) BindCredential(c *gin.Context) {
	entry, ok := h.find(c)
	if !ok {
		return
	}
	if entry.Status == modelConfigActive || entry.Status == modelConfigCanary {
		common.FailMsg(c, common.CodeParamInvalid, "激活或灰度配置不能直接更换凭证，请创建新候选配置")
		return
	}
	var req ModelConfigCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if req.CredentialID == entry.CredentialID {
		common.FailMsg(c, common.CodeParamInvalid, "模型配置已经绑定该凭证")
		return
	}
	if err := validateCredentialBinding(h.db, req.CredentialID); err != nil {
		if errors.Is(err, ErrCredentialUnavailable) {
			common.FailMsg(c, common.CodeParamInvalid, err.Error())
		} else {
			common.Fail(c, common.CodeDBError)
		}
		return
	}

	now := time.Now()
	summary := "凭证已变更，请重新执行质量门禁"
	if req.CredentialID == 0 {
		summary = "已解除凭证绑定，将使用环境变量 LLM_API_KEY；请重新执行质量门禁"
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entry).Updates(map[string]interface{}{
			"credential_id":      req.CredentialID,
			"status":             modelConfigDraft,
			"quality_passed":     false,
			"quality_summary":    summary,
			"quality_metrics":    "",
			"quality_checked_at": nil,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIModelChange{
			ConfigID: entry.ID, ToConfigID: entry.ID, ActorID: middleware.CurrentUserID(c),
			Action: "bind_credential", Reason: summary, QualitySummary: summary, CreatedAt: now,
		}).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{
		"configId":      entry.ID,
		"credentialId":  req.CredentialID,
		"status":        modelConfigDraft,
		"qualityPassed": false,
	})
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
	query := h.db.Order("created_at DESC").Order("id DESC").Limit(500)
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
	cfg, err := resolveModelConfig(h.db, h.base, entry)
	if err != nil {
		body, _ := json.Marshal(map[string]interface{}{
			"errorType": ai.ErrorTypeConfig,
			"message":   "AI 凭证不可用",
		})
		return false, "AI 凭证不可用，未通过质量门禁", string(body)
	}
	provider := ai.NewProvider(cfg)
	promptVersion, promptContent := promptForVersion(h.db, entry.PromptVersion)
	return runQualityGateWithPrompt(ctx, provider, promptVersion, promptContent)
}

func resolveModelConfig(db *gorm.DB, base config.LLMConfig, value model.SysAIModelConfig) (config.LLMConfig, error) {
	cfg := modelConfigToLLM(base, value)
	if value.CredentialID == 0 {
		return cfg, nil
	}
	var credential model.SysAICredential
	if err := db.Where("status = ?", 1).First(&credential, value.CredentialID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return cfg, ErrCredentialUnavailable
		}
		return cfg, err
	}
	apiKey, err := security.DecryptSecret(base.CredentialEncryptionKey, credential.EncryptedAPIKey)
	if err != nil {
		return cfg, errors.New("AI 凭证解密失败")
	}
	cfg.APIKey = apiKey
	return cfg, nil
}

func ptrTime(value time.Time) *time.Time { return &value }
