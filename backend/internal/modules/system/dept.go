package system

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type DeptHandler struct {
	db *gorm.DB
}

func NewDeptHandler(db *gorm.DB) *DeptHandler {
	return &DeptHandler{db: db}
}

// @Summary  部门树
// @Tags     系统管理-部门
// @Description 需要权限：system:dept:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/dept/tree [get]
func (h *DeptHandler) Tree(c *gin.Context) {
	query := middleware.TenantScope(c, h.db, "sys_dept")
	if middleware.IsPrivileged(c) {
		if tid, err := strconv.ParseUint(c.Query("tenantId"), 10, 64); err == nil {
			query = query.Where("sys_dept.tenant_id = ?", tid)
		}
	}
	var depts []model.SysDept
	if err := query.Order("sort, id").Find(&depts).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, buildDeptTree(depts, 0))
}

type DeptSaveRequest struct {
	TenantID uint64 `json:"tenantId"`
	ParentID uint64 `json:"parentId"`
	Name     string `json:"name" binding:"required,max=64"`
	Leader   string `json:"leader" binding:"max=64"`
	Sort     int    `json:"sort"`
	Status   int8   `json:"status" binding:"oneof=0 1"`
}

func (h *DeptHandler) validateParent(c *gin.Context, parentID, tenantID, selfID uint64) bool {
	seen := map[uint64]struct{}{}
	for parentID != 0 {
		if parentID == selfID {
			common.FailMsg(c, common.CodeParamInvalid, "上级部门不能是自身或子部门")
			return false
		}
		if _, ok := seen[parentID]; ok {
			common.FailMsg(c, common.CodeParamInvalid, "部门层级存在循环")
			return false
		}
		seen[parentID] = struct{}{}
		var parent model.SysDept
		if err := h.db.Select("id", "parent_id", "tenant_id").First(&parent, parentID).Error; err != nil {
			common.FailMsg(c, common.CodeParamInvalid, "上级部门不存在")
			return false
		}
		if parent.TenantID != tenantID {
			common.FailMsg(c, common.CodeParamInvalid, "上级部门不属于当前租户")
			return false
		}
		parentID = parent.ParentID
	}
	return true
}

// @Summary  新增部门
// @Tags     系统管理-部门
// @Description 需要权限：system:dept:create
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/dept [post]
func (h *DeptHandler) Create(c *gin.Context) {
	var req DeptSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	if middleware.IsPrivileged(c) {
		tenantID = req.TenantID
	}
	if tenantID != 0 {
		var count int64
		if err := h.db.Model(&model.SysTenant{}).Where("id = ?", tenantID).Count(&count).Error; err != nil {
			common.Fail(c, common.CodeDBError)
			return
		}
		if count == 0 {
			common.Fail(c, common.CodeTenantNotFound)
			return
		}
	}
	if !h.validateParent(c, req.ParentID, tenantID, 0) {
		return
	}
	dept := model.SysDept{TenantID: tenantID, ParentID: req.ParentID, Name: req.Name, Leader: req.Leader, Sort: req.Sort, Status: req.Status}
	if err := h.db.Create(&dept).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": dept.ID})
}

// findInTenant loads a dept scoped to the caller's tenant (super = any).
func (h *DeptHandler) findInTenant(c *gin.Context, id uint64) (*model.SysDept, bool) {
	var dept model.SysDept
	query := h.db
	if !middleware.IsPrivileged(c) {
		query = query.Where("tenant_id = ?", middleware.CurrentTenantID(c))
	}
	if err := query.First(&dept, id).Error; err != nil {
		common.Fail(c, common.CodeDeptNotFound)
		return nil, false
	}
	return &dept, true
}

// @Summary  编辑部门
// @Tags     系统管理-部门
// @Description 需要权限：system:dept:update
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/dept/{id} [put]
func (h *DeptHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	dept, ok := h.findInTenant(c, id)
	if !ok {
		return
	}
	var req DeptSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if !h.validateParent(c, req.ParentID, dept.TenantID, dept.ID) {
		return
	}
	dept.ParentID = req.ParentID
	dept.Name = req.Name
	dept.Leader = req.Leader
	dept.Sort = req.Sort
	if req.Status == 0 || req.Status == 1 {
		dept.Status = req.Status
	}
	if err := h.db.Save(dept).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

// @Summary  删除部门
// @Tags     系统管理-部门
// @Description 需要权限：system:dept:delete
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /system/dept/{id} [delete]
func (h *DeptHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if _, ok := h.findInTenant(c, id); !ok {
		return
	}
	var children int64
	h.db.Model(&model.SysDept{}).Where("parent_id = ?", id).Count(&children)
	if children > 0 {
		common.FailMsg(c, common.CodeParamInvalid, "请先删除子部门")
		return
	}
	var users int64
	h.db.Model(&model.SysUser{}).Where("dept_id = ?", id).Count(&users)
	if users > 0 {
		common.FailMsg(c, common.CodeParamInvalid, "部门下存在用户，不可删除")
		return
	}
	// Hard delete for the same unique-index reason; dept is guarded above
	// (no children, no users) so this stays safe.
	result := h.db.Unscoped().Delete(&model.SysDept{}, id)
	if result.RowsAffected == 0 {
		common.Fail(c, common.CodeDeptNotFound)
		return
	}
	common.OK(c, nil)
}

func buildDeptTree(depts []model.SysDept, parentID uint64) []model.SysDept {
	tree := make([]model.SysDept, 0)
	for _, d := range depts {
		if d.ParentID == parentID {
			node := d
			node.Children = buildDeptTree(depts, d.ID)
			tree = append(tree, node)
		}
	}
	return tree
}
