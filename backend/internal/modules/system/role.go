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

// Roles are global: shared by all tenants, managed on the platform side.
type RoleHandler struct {
	db *gorm.DB
}

func NewRoleHandler(db *gorm.DB) *RoleHandler {
	return &RoleHandler{db: db}
}

// @Summary  角色分页列表
// @Tags     系统管理-角色
// @Description 需要权限：system:role:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/role [get]
func (h *RoleHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)
	name := strings.TrimSpace(c.Query("name"))

	query := h.db.Model(&model.SysRole{})
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var roles []model.SysRole
	if err := query.Preload("Menus").Order("sys_role.sort, sys_role.id").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&roles).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, roles, total, page.PageNum, page.PageSize)
}

// All returns every enabled role, used by the user form's role selector.
// @Summary  全部角色（下拉用）
// @Tags     系统管理-角色
// @Description 需要权限：system:role:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/role/all [get]
func (h *RoleHandler) All(c *gin.Context) {
	var roles []model.SysRole
	if err := h.db.Where("status = 1").Order("sort, id").Find(&roles).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, roles)
}

type RoleSaveRequest struct {
	Name    string   `json:"name" binding:"required,max=64"`
	Code    string   `json:"code" binding:"required,max=64"`
	Sort    int      `json:"sort"`
	Status  int8     `json:"status" binding:"oneof=0 1"`
	Remark  string   `json:"remark" binding:"max=255"`
	MenuIDs []uint64 `json:"menuIds"`
}

func (h *RoleHandler) validateMenuIDs(c *gin.Context, menuIDs []uint64) ([]uint64, bool) {
	if len(menuIDs) == 0 {
		return nil, true
	}
	unique := make([]uint64, 0, len(menuIDs))
	seen := make(map[uint64]struct{}, len(menuIDs))
	for _, id := range menuIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	var count int64
	if err := h.db.Model(&model.SysMenu{}).Where("id IN ?", unique).Count(&count).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return nil, false
	}
	if count != int64(len(unique)) {
		common.FailMsg(c, common.CodeParamInvalid, "菜单不存在")
		return nil, false
	}
	return unique, true
}

// @Summary  新增角色
// @Tags     系统管理-角色
// @Description 需要权限：system:role:create
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/role [post]
func (h *RoleHandler) Create(c *gin.Context) {
	var req RoleSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	menuIDs, ok := h.validateMenuIDs(c, req.MenuIDs)
	if !ok {
		return
	}
	var exists int64
	h.db.Model(&model.SysRole{}).Where("code = ?", req.Code).Count(&exists)
	if exists > 0 {
		common.Fail(c, common.CodeRoleExists)
		return
	}
	role := model.SysRole{Name: req.Name, Code: req.Code, Sort: req.Sort, Status: req.Status, Remark: req.Remark}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		return replaceRoleMenus(tx, role.ID, menuIDs)
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": role.ID})
}

// @Summary  编辑角色（含菜单授权）
// @Tags     系统管理-角色
// @Description 需要权限：system:role:update
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/role/{id} [put]
func (h *RoleHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	var role model.SysRole
	if err := h.db.First(&role, id).Error; err != nil {
		common.Fail(c, common.CodeRoleNotFound)
		return
	}
	if (role.Code == middleware.RoleCodeAdmin || role.Code == middleware.RoleCodeSuperAdmin) && !middleware.IsSuper(c) {
		common.Fail(c, common.CodeForbidden)
		return
	}
	var req RoleSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	menuIDs, ok := h.validateMenuIDs(c, req.MenuIDs)
	if !ok {
		return
	}
	if role.Code == middleware.RoleCodeSuperAdmin && req.Code != middleware.RoleCodeSuperAdmin {
		common.FailMsg(c, common.CodeParamInvalid, "内置超级管理员角色标识不可修改")
		return
	}
	if role.Code == middleware.RoleCodeAdmin && req.Code != middleware.RoleCodeAdmin {
		common.FailMsg(c, common.CodeParamInvalid, "内置管理员角色标识不可修改")
		return
	}
	if req.Code != role.Code {
		var dup int64
		h.db.Model(&model.SysRole{}).Where("code = ? AND id <> ?", req.Code, role.ID).Count(&dup)
		if dup > 0 {
			common.Fail(c, common.CodeRoleExists)
			return
		}
	}
	role.Name = req.Name
	role.Code = req.Code
	role.Sort = req.Sort
	role.Remark = req.Remark
	if req.Status == 0 || req.Status == 1 {
		role.Status = req.Status
	}
	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&role).Error; err != nil {
			return err
		}
		return replaceRoleMenus(tx, role.ID, menuIDs)
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

// @Summary  删除角色
// @Tags     系统管理-角色
// @Description 需要权限：system:role:delete
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/role/{id} [delete]
func (h *RoleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	var role model.SysRole
	if err := h.db.First(&role, id).Error; err != nil {
		common.Fail(c, common.CodeRoleNotFound)
		return
	}
	if role.Code == middleware.RoleCodeSuperAdmin || role.Code == middleware.RoleCodeAdmin {
		common.FailMsg(c, common.CodeParamInvalid, "内置管理员角色不可删除")
		return
	}
	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("role_id = ?", role.ID).Delete(&model.SysRoleMenu{}).Error; err != nil {
			return err
		}
		if err := tx.Where("role_id = ?", role.ID).Delete(&model.SysUserRole{}).Error; err != nil {
			return err
		}
		// Hard delete: the code unique index would otherwise keep
		// blocking re-creation after a soft delete.
		return tx.Unscoped().Delete(&role).Error
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

func replaceRoleMenus(tx *gorm.DB, roleID uint64, menuIDs []uint64) error {
	if err := tx.Where("role_id = ?", roleID).Delete(&model.SysRoleMenu{}).Error; err != nil {
		return err
	}
	if len(menuIDs) == 0 {
		return nil
	}
	links := make([]model.SysRoleMenu, 0, len(menuIDs))
	for _, mid := range menuIDs {
		links = append(links, model.SysRoleMenu{RoleID: roleID, MenuID: mid})
	}
	return tx.Create(&links).Error
}
