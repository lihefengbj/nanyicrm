package crm

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
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
	promptStatusDraft    = "draft"
	promptStatusApproved = "approved"
	promptStatusActive   = "active"
	promptStatusCanary   = "canary"
	promptStatusRetired  = "retired"
)

type PromptHandler struct {
	db   *gorm.DB
	base config.LLMConfig
}

func NewPromptHandler(db *gorm.DB, base config.LLMConfig) *PromptHandler {
	return &PromptHandler{db: db, base: base}
}

type PromptRequest struct {
	Name    string `json:"name" binding:"required,max=64"`
	Content string `json:"content" binding:"required,max=20000"`
}

type PromptActionRequest struct {
	Reason         string `json:"reason"`
	CanaryPercent  int    `json:"canaryPercent"`
	TargetPromptID uint64 `json:"targetPromptId"`
}

func (h *PromptHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)
	var total int64
	if err := h.db.Model(&model.SysAIPrompt{}).Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var rows []model.SysAIPrompt
	if err := h.db.Order("id DESC").Offset(page.Offset()).Limit(page.PageSize).Find(&rows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, rows, total, page.PageNum, page.PageSize)
}

func (h *PromptHandler) Create(c *gin.Context) {
	var req PromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Content = normalizePromptContent(req.Content)
	if req.Name == "" || req.Content == "" {
		common.FailMsg(c, common.CodeParamInvalid, "提示词名称和研判规则不能为空")
		return
	}
	hash := promptContentHash(req.Content)
	version := promptVersionFromHash(hash)
	entry := model.SysAIPrompt{
		Name: req.Name, Content: req.Content, ContentHash: hash, Version: version,
		Status: promptStatusDraft, CreatedBy: middleware.CurrentUserID(c),
		UpdatedBy: middleware.CurrentUserID(c),
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&entry).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIPromptChange{
			PromptID: entry.ID, ToPromptID: entry.ID, ActorID: middleware.CurrentUserID(c),
			Action: "create", ToVersion: entry.Version, Reason: "创建提示词草稿", CreatedAt: time.Now(),
		}).Error
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			common.FailMsg(c, common.CodeParamInvalid, duplicatePromptMessage(h.db, hash))
		} else {
			common.Fail(c, common.CodeDBError)
		}
		return
	}
	common.OK(c, entry)
}

func (h *PromptHandler) Update(c *gin.Context) {
	var entry model.SysAIPrompt
	if err := h.db.First(&entry, strings.TrimSpace(c.Param("id"))).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return
	}
	if entry.Status != promptStatusDraft {
		common.FailMsg(c, common.CodeParamInvalid, "已通过或已发布的提示词版本不可编辑，请创建新版本")
		return
	}
	var req PromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Content = normalizePromptContent(req.Content)
	if req.Name == "" || req.Content == "" {
		common.FailMsg(c, common.CodeParamInvalid, "提示词名称和研判规则不能为空")
		return
	}
	hash := promptContentHash(req.Content)
	version := promptVersionFromHash(hash)
	now := time.Now()
	actorID := middleware.CurrentUserID(c)
	oldVersion := entry.Version
	entry.Name, entry.Content, entry.ContentHash, entry.Version, entry.UpdatedBy = req.Name, req.Content, hash, version, actorID
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&entry).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIPromptChange{
			PromptID: entry.ID, ToPromptID: entry.ID, ActorID: actorID,
			Action: "update", FromVersion: oldVersion, ToVersion: entry.Version,
			Reason: "编辑提示词草稿", CreatedAt: now,
		}).Error
	}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			common.FailMsg(c, common.CodeParamInvalid, duplicatePromptMessage(h.db, hash))
		} else {
			common.Fail(c, common.CodeDBError)
		}
		return
	}
	common.OK(c, entry)
}

func (h *PromptHandler) QualityGate(c *gin.Context) {
	var entry model.SysAIPrompt
	if err := h.db.First(&entry, strings.TrimSpace(c.Param("id"))).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return
	}
	cfg := h.base
	var active model.SysAIModelConfig
	if err := h.db.Where("status = ?", modelConfigActive).Order("id DESC").First(&active).Error; err == nil {
		var resolveErr error
		cfg, resolveErr = resolveModelConfig(h.db, h.base, active)
		if resolveErr != nil {
			common.FailMsg(c, common.CodeParamInvalid, "当前激活模型凭证不可用，无法执行提示词质量门禁")
			return
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.Fail(c, common.CodeDBError)
		return
	}
	passed, summary, metrics := runQualityGateWithPrompt(c.Request.Context(), ai.NewProvider(cfg), entry.Version, entry.Content)
	now := time.Now()
	status := promptStatusDraft
	if passed {
		status = promptStatusApproved
	}
	actorID := middleware.CurrentUserID(c)
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&entry).Updates(map[string]interface{}{
			"status": status, "quality_passed": passed, "quality_summary": summary,
			"quality_metrics": metrics, "quality_checked_at": now, "updated_by": actorID,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIPromptChange{
			PromptID: entry.ID, ToPromptID: entry.ID, ActorID: actorID,
			Action: "quality_gate", ToVersion: entry.Version,
			QualitySummary: summary, CreatedAt: now,
		}).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	entry.Status, entry.QualityPassed, entry.QualitySummary = status, passed, summary
	entry.QualityMetrics, entry.QualityCheckedAt = metrics, &now
	common.OK(c, gin.H{"passed": passed, "prompt": entry})
}

func (h *PromptHandler) Canary(c *gin.Context) {
	entry, ok := h.find(c)
	if !ok {
		return
	}
	var req PromptActionRequest
	_ = c.ShouldBindJSON(&req)
	if !entry.QualityPassed {
		common.FailMsg(c, common.CodeParamInvalid, "提示词必须先通过质量门禁")
		return
	}
	if req.CanaryPercent < 1 || req.CanaryPercent > 99 {
		common.FailMsg(c, common.CodeParamInvalid, "灰度比例必须为 1 到 99")
		return
	}
	var active model.SysAIPrompt
	_ = h.db.Where("status = ?", promptStatusActive).Order("id DESC").First(&active).Error
	if active.ID == entry.ID {
		common.FailMsg(c, common.CodeParamInvalid, "当前激活版本不能再次设置为灰度")
		return
	}
	now := time.Now()
	actorID := middleware.CurrentUserID(c)
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SysAIPrompt{}).
			Where("status = ? AND id <> ?", promptStatusCanary, entry.ID).
			Updates(map[string]interface{}{"status": promptStatusRetired, "canary_percent": 0, "retired_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&entry).Updates(map[string]interface{}{
			"status": promptStatusCanary, "canary_percent": req.CanaryPercent,
			"updated_by": actorID,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIPromptChange{
			PromptID: entry.ID, FromPromptID: active.ID, ToPromptID: entry.ID,
			ActorID: actorID, Action: "canary", FromVersion: active.Version,
			ToVersion: entry.Version, Reason: req.Reason, QualitySummary: entry.QualitySummary,
			CreatedAt: now,
		}).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"status": promptStatusCanary, "canaryPercent": req.CanaryPercent, "promptId": entry.ID})
}

func (h *PromptHandler) Activate(c *gin.Context) {
	entry, ok := h.find(c)
	if !ok {
		return
	}
	var req PromptActionRequest
	_ = c.ShouldBindJSON(&req)
	if !entry.QualityPassed {
		common.FailMsg(c, common.CodeParamInvalid, "提示词必须先通过质量门禁")
		return
	}
	var previous model.SysAIPrompt
	_ = h.db.Where("status = ?", promptStatusActive).Order("id DESC").First(&previous).Error
	if previous.ID == entry.ID {
		common.FailMsg(c, common.CodeParamInvalid, "当前版本已经是激活版本")
		return
	}
	now := time.Now()
	actorID := middleware.CurrentUserID(c)
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SysAIPrompt{}).
			Where("status IN ?", []string{promptStatusActive, promptStatusCanary}).
			Updates(map[string]interface{}{"status": promptStatusRetired, "canary_percent": 0, "retired_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&entry).Updates(map[string]interface{}{
			"status": promptStatusActive, "canary_percent": 0, "activated_at": now,
			"retired_at": nil, "updated_by": actorID,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIPromptChange{
			PromptID: entry.ID, FromPromptID: previous.ID, ToPromptID: entry.ID,
			ActorID: actorID, Action: "activate", FromVersion: previous.Version,
			ToVersion: entry.Version, Reason: req.Reason, QualitySummary: entry.QualitySummary,
			CreatedAt: now,
		}).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"status": promptStatusActive, "promptId": entry.ID})
}

func (h *PromptHandler) Rollback(c *gin.Context) {
	var req PromptActionRequest
	_ = c.ShouldBindJSON(&req)
	targetID := req.TargetPromptID
	if targetID == 0 {
		var change model.SysAIPromptChange
		if err := h.db.Where("action = ? AND from_prompt_id > 0", "activate").
			Order("id DESC").First(&change).Error; err != nil {
			common.FailMsg(c, common.CodeRecordNotFound, "没有可回滚的提示词版本")
			return
		}
		targetID = change.FromPromptID
	}
	var target model.SysAIPrompt
	if err := h.db.First(&target, targetID).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return
	}
	if !target.QualityPassed {
		common.FailMsg(c, common.CodeParamInvalid, "回滚目标未通过质量门禁")
		return
	}
	var previous model.SysAIPrompt
	_ = h.db.Where("status = ?", promptStatusActive).Order("id DESC").First(&previous).Error
	if previous.ID == target.ID {
		common.FailMsg(c, common.CodeParamInvalid, "回滚目标已经是当前激活版本")
		return
	}
	now := time.Now()
	actorID := middleware.CurrentUserID(c)
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.SysAIPrompt{}).
			Where("status IN ?", []string{promptStatusActive, promptStatusCanary}).
			Updates(map[string]interface{}{"status": promptStatusRetired, "canary_percent": 0, "retired_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&target).Updates(map[string]interface{}{
			"status": promptStatusActive, "canary_percent": 0, "activated_at": now,
			"retired_at": nil, "updated_by": actorID,
		}).Error; err != nil {
			return err
		}
		return tx.Create(&model.SysAIPromptChange{
			PromptID: target.ID, FromPromptID: previous.ID, ToPromptID: target.ID,
			ActorID: actorID, Action: "rollback", FromVersion: previous.Version,
			ToVersion: target.Version, Reason: req.Reason, QualitySummary: target.QualitySummary,
			CreatedAt: now,
		}).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"status": promptStatusActive, "promptId": target.ID})
}

func (h *PromptHandler) Changes(c *gin.Context) {
	var rows []model.SysAIPromptChange
	query := h.db.Order("created_at DESC").Order("id DESC").Limit(500)
	if id := strings.TrimSpace(c.Param("id")); id != "" {
		query = query.Where("prompt_id = ?", id)
	}
	if err := query.Find(&rows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, rows)
}

func (h *PromptHandler) find(c *gin.Context) (model.SysAIPrompt, bool) {
	var entry model.SysAIPrompt
	if err := h.db.First(&entry, strings.TrimSpace(c.Param("id"))).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return entry, false
	}
	return entry, true
}

func promptContentHash(content string) string {
	sum := sha256.Sum256([]byte(normalizePromptContent(content)))
	return hex.EncodeToString(sum[:])
}

func promptVersionFromHash(hash string) string {
	if len(hash) > 12 {
		hash = hash[:12]
	}
	return "p-" + hash
}

func normalizePromptContent(content string) string {
	return strings.TrimSpace(strings.ReplaceAll(content, "\r\n", "\n"))
}

func duplicatePromptMessage(db *gorm.DB, hash string) string {
	if db != nil {
		var existing model.SysAIPrompt
		if db.Where("content_hash = ?", hash).First(&existing).Error == nil {
			return fmt.Sprintf(
				"研判规则内容未发生变化，已存在版本 %s；仅修改名称不会生成新版本，请调整规则内容后再保存",
				existing.Version,
			)
		}
	}
	return "研判规则内容未发生变化，相同内容的提示词版本已存在；请调整规则内容后再保存"
}

func promptForVersion(db *gorm.DB, version string) (string, string) {
	version = strings.TrimSpace(version)
	if db != nil && version != "" {
		var entry model.SysAIPrompt
		if db.Where("version = ? AND quality_passed = ?", version, true).First(&entry).Error == nil {
			return entry.Version, entry.Content
		}
	}
	if version == "" {
		version = ai.PromptVersion
	}
	return version, ai.DefaultPromptContent
}

// EnsureDefaultPrompt makes the legacy v1 policy available when development
// uses AutoMigrate instead of the immutable production migration.
func EnsureDefaultPrompt(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	var active model.SysAIPrompt
	if err := db.Where("status = ?", promptStatusActive).First(&active).Error; err == nil {
		return nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	content := ai.DefaultPromptContent
	hash := promptContentHash(content)
	var entry model.SysAIPrompt
	if err := db.Where("version = ?", ai.PromptVersion).First(&entry).Error; err == nil {
		return db.Model(&entry).Updates(map[string]interface{}{
			"status": promptStatusActive, "quality_passed": true,
			"quality_summary": "由部署配置初始化；后续切换必须通过固定样本质量门禁",
			"activated_at":    time.Now(),
		}).Error
	}
	entry = model.SysAIPrompt{
		Name: "意向研判口径 v1", Content: content, ContentHash: hash,
		Version: ai.PromptVersion, Status: promptStatusActive,
		QualityPassed: true, QualitySummary: "由部署配置初始化；后续切换必须通过固定样本质量门禁",
		ActivatedAt: ptrTime(time.Now()),
	}
	if err := db.Create(&entry).Error; err != nil {
		return fmt.Errorf("create default prompt: %w", err)
	}
	return nil
}
