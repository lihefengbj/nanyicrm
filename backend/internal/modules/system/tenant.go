package system

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type TenantHandler struct {
	db *gorm.DB
}

func NewTenantHandler(db *gorm.DB) *TenantHandler {
	return &TenantHandler{db: db}
}

func (h *TenantHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))

	query := h.db.Model(&model.SysTenant{})
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("code LIKE ? OR name LIKE ?", like, like)
	}
	if status != "" {
		if s, err := strconv.Atoi(status); err == nil {
			query = query.Where("status = ?", s)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var tenants []model.SysTenant
	if err := query.Order("id DESC").Offset(page.Offset()).Limit(page.PageSize).Find(&tenants).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, tenants, total, page.PageNum, page.PageSize)
}

// All returns enabled tenants for selectors (e.g. super admin's user form).
func (h *TenantHandler) All(c *gin.Context) {
	var tenants []model.SysTenant
	if err := h.db.Where("status = 1").Order("id").Find(&tenants).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, tenants)
}

type TenantSaveRequest struct {
	Code     string `json:"code" binding:"required,min=2,max=64"`
	Name     string `json:"name" binding:"required,max=64"`
	Contact  string `json:"contact" binding:"max=64"`
	Phone    string `json:"phone" binding:"max=32"`
	ExpireAt string `json:"expireAt"` // RFC3339 date or "" for never
	Status   int8   `json:"status"`
	Remark   string `json:"remark" binding:"max=255"`
}

func parseExpireAt(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	return nil, errInvalidDate
}

var errInvalidDate = &dateError{}

type dateError struct{}

func (e *dateError) Error() string { return "invalid expireAt, want YYYY-MM-DD" }

func (h *TenantHandler) Create(c *gin.Context) {
	var req TenantSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	expireAt, err := parseExpireAt(req.ExpireAt)
	if err != nil {
		common.FailMsg(c, common.CodeParamInvalid, "过期时间格式应为 YYYY-MM-DD")
		return
	}
	var exists int64
	h.db.Model(&model.SysTenant{}).Where("code = ?", req.Code).Count(&exists)
	if exists > 0 {
		common.Fail(c, common.CodeTenantExists)
		return
	}
	tenant := model.SysTenant{
		Code: req.Code, Name: req.Name, Contact: req.Contact, Phone: req.Phone,
		ExpireAt: expireAt, Status: req.Status, Remark: req.Remark,
	}
	if tenant.Status == 0 {
		tenant.Status = 1
	}
	if err := h.db.Create(&tenant).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": tenant.ID})
}

func (h *TenantHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	var tenant model.SysTenant
	if err := h.db.First(&tenant, id).Error; err != nil {
		common.Fail(c, common.CodeTenantNotFound)
		return
	}
	var req TenantSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	expireAt, err := parseExpireAt(req.ExpireAt)
	if err != nil {
		common.FailMsg(c, common.CodeParamInvalid, "过期时间格式应为 YYYY-MM-DD")
		return
	}
	if req.Code != tenant.Code {
		var dup int64
		h.db.Model(&model.SysTenant{}).Where("code = ? AND id <> ?", req.Code, tenant.ID).Count(&dup)
		if dup > 0 {
			common.Fail(c, common.CodeTenantExists)
			return
		}
	}
	tenant.Code, tenant.Name = req.Code, req.Name
	tenant.Contact, tenant.Phone = req.Contact, req.Phone
	tenant.ExpireAt, tenant.Remark = expireAt, req.Remark
	if req.Status == 0 || req.Status == 1 {
		tenant.Status = req.Status
	}
	if err := h.db.Save(&tenant).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

// Delete refuses to remove a tenant that still owns any data.
func (h *TenantHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	var tenant model.SysTenant
	if err := h.db.First(&tenant, id).Error; err != nil {
		common.Fail(c, common.CodeTenantNotFound)
		return
	}
	checks := []struct {
		model interface{}
		label string
	}{
		{&model.SysUser{}, "用户"},
		{&model.SysDept{}, "部门"},
		{&model.SysDict{}, "字典"},
		{&model.SysOperLog{}, "操作日志"},
		{&model.SysLoginLog{}, "登录日志"},
	}
	for _, chk := range checks {
		var count int64
		if err := h.db.Model(chk.model).Where("tenant_id = ?", tenant.ID).Count(&count).Error; err != nil {
			common.Fail(c, common.CodeDBError)
			return
		}
		if count > 0 {
			common.FailMsg(c, common.CodeTenantInUse, "租户下存在"+chk.label+"数据，不可删除")
			return
		}
	}
	if err := h.db.Unscoped().Delete(&tenant).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

// RequirePrivileged aborts the request unless the caller may manage tenants
// (superAdmin or admin role).
func RequirePrivileged() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !middleware.IsPrivileged(c) {
			common.Abort(c, common.CodeForbidden)
			return
		}
		c.Next()
	}
}
