package system

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

const roleCodeAdmin = "admin"

type RoleHandler struct {
	db *gorm.DB
}

func NewRoleHandler(db *gorm.DB) *RoleHandler {
	return &RoleHandler{db: db}
}

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
	Status  int8     `json:"status"`
	Remark  string   `json:"remark" binding:"max=255"`
	MenuIDs []uint64 `json:"menuIds"`
}

func (h *RoleHandler) Create(c *gin.Context) {
	var req RoleSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	var exists int64
	h.db.Model(&model.SysRole{}).Where("code = ?", req.Code).Count(&exists)
	if exists > 0 {
		common.Fail(c, common.CodeRoleExists)
		return
	}
	role := model.SysRole{Name: req.Name, Code: req.Code, Sort: req.Sort, Status: req.Status, Remark: req.Remark}
	if role.Status == 0 {
		role.Status = 1
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&role).Error; err != nil {
			return err
		}
		return replaceRoleMenus(tx, role.ID, req.MenuIDs)
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": role.ID})
}

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
	var req RoleSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if role.Code == roleCodeAdmin && req.Code != roleCodeAdmin {
		common.FailMsg(c, common.CodeParamInvalid, "内置管理员角色标识不可修改")
		return
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
		return replaceRoleMenus(tx, role.ID, req.MenuIDs)
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

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
	if role.Code == roleCodeAdmin {
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
		return tx.Delete(&role).Error
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
