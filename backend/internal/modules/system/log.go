package system

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type LogHandler struct {
	db *gorm.DB
}

func NewLogHandler(db *gorm.DB) *LogHandler {
	return &LogHandler{db: db}
}

// scope builds the base log query: tenant users see only their own tenant,
// the super admin sees everything and may filter by tenantId.
func (h *LogHandler) scope(c *gin.Context, model interface{}, table string) *gorm.DB {
	query := middleware.TenantScope(c, h.db.Model(model), table)
	if middleware.IsPrivileged(c) {
		if tid, err := strconv.ParseUint(c.Query("tenantId"), 10, 64); err == nil && tid > 0 {
			query = query.Where(table+".tenant_id = ?", tid)
		}
	}
	if username := strings.TrimSpace(c.Query("username")); username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	return query
}

// @Summary  操作日志分页列表
// @Tags     系统管理-日志
// @Description 需要权限：system:log:oper
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/log/oper [get]
func (h *LogHandler) OperList(c *gin.Context) {
	page := common.ParsePageQuery(c)
	query := h.scope(c, &model.SysOperLog{}, "sys_oper_log")
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

// @Summary  登录日志分页列表
// @Tags     系统管理-日志
// @Description 需要权限：system:log:login
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/log/login [get]
func (h *LogHandler) LoginList(c *gin.Context) {
	page := common.ParsePageQuery(c)
	query := h.scope(c, &model.SysLoginLog{}, "sys_login_log")
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
