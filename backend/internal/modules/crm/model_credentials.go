package crm

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
	"github.com/lihefengbj/nanyicrm/backend/internal/security"
)

var ErrCredentialUnavailable = errors.New("AI 凭证不存在或已停用")

type ModelCredentialHandler struct {
	db            *gorm.DB
	encryptionKey string
}

type ModelCredentialRequest struct {
	Name     string `json:"name" binding:"required,max=64"`
	Provider string `json:"provider" binding:"required,max=64"`
	APIKey   string `json:"apiKey" binding:"required,max=4096"`
}

type ModelCredentialView struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Provider  string    `json:"provider"`
	KeyLast4  string    `json:"keyLast4"`
	Status    int8      `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewModelCredentialHandler(db *gorm.DB, encryptionKey string) *ModelCredentialHandler {
	return &ModelCredentialHandler{db: db, encryptionKey: encryptionKey}
}

func (h *ModelCredentialHandler) List(c *gin.Context) {
	var rows []model.SysAICredential
	if err := h.db.Order("id DESC").Find(&rows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	result := make([]ModelCredentialView, 0, len(rows))
	for _, row := range rows {
		result = append(result, credentialView(row))
	}
	common.OK(c, result)
}

func (h *ModelCredentialHandler) Create(c *gin.Context) {
	var req ModelCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Provider = strings.TrimSpace(req.Provider)
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.Name == "" || req.Provider == "" || req.APIKey == "" {
		common.FailMsg(c, common.CodeParamInvalid, "凭证名称、Provider 和 API Key 不能为空")
		return
	}
	encrypted, err := security.EncryptSecret(h.encryptionKey, req.APIKey)
	if err != nil {
		common.FailMsg(c, common.CodeInternalError, "AI 凭证加密配置不可用")
		return
	}
	entry := model.SysAICredential{
		Name:            req.Name,
		Provider:        req.Provider,
		EncryptedAPIKey: encrypted,
		KeyLast4:        security.KeyLast4(req.APIKey),
		Status:          1,
	}
	if err := h.db.Create(&entry).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, credentialView(entry))
}

func (h *ModelCredentialHandler) Rotate(c *gin.Context) {
	var req struct {
		APIKey string `json:"apiKey" binding:"required,max=4096"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	req.APIKey = strings.TrimSpace(req.APIKey)
	if req.APIKey == "" {
		common.FailMsg(c, common.CodeParamInvalid, "API Key 不能为空")
		return
	}
	var entry model.SysAICredential
	if err := h.db.First(&entry, strings.TrimSpace(c.Param("id"))).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return
	}
	encrypted, err := security.EncryptSecret(h.encryptionKey, req.APIKey)
	if err != nil {
		common.FailMsg(c, common.CodeInternalError, "AI 凭证加密配置不可用")
		return
	}
	if err := h.db.Model(&entry).Updates(map[string]interface{}{
		"encrypted_api_key": encrypted,
		"key_last4":         security.KeyLast4(req.APIKey),
		"updated_at":        time.Now(),
	}).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	entry.EncryptedAPIKey = encrypted
	entry.KeyLast4 = security.KeyLast4(req.APIKey)
	entry.UpdatedAt = time.Now()
	common.OK(c, credentialView(entry))
}

func credentialView(row model.SysAICredential) ModelCredentialView {
	return ModelCredentialView{
		ID: row.ID, Name: row.Name, Provider: row.Provider,
		KeyLast4: row.KeyLast4, Status: row.Status,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func validateCredentialBinding(db *gorm.DB, credentialID uint64) error {
	if credentialID == 0 {
		return nil
	}
	var credential model.SysAICredential
	if err := db.Where("status = ?", 1).First(&credential, credentialID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCredentialUnavailable
		}
		return err
	}
	return nil
}
