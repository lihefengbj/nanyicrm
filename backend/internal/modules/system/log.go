package system

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type LogHandler struct {
	db *gorm.DB
}

func NewLogHandler(db *gorm.DB) *LogHandler {
	return &LogHandler{db: db}
}

func (h *LogHandler) OperList(c *gin.Context) {
	page := common.ParsePageQuery(c)
	username := strings.TrimSpace(c.Query("username"))

	query := h.db.Model(&model.SysOperLog{})
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var logs []model.SysOperLog
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.PageSize).Find(&logs).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, logs, total, page.PageNum, page.PageSize)
}

func (h *LogHandler) LoginList(c *gin.Context) {
	page := common.ParsePageQuery(c)
	username := strings.TrimSpace(c.Query("username"))

	query := h.db.Model(&model.SysLoginLog{})
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var logs []model.SysLoginLog
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.PageSize).Find(&logs).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, logs, total, page.PageNum, page.PageSize)
}
