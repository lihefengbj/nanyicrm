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
	ParentID uint64 `json:"parentId"`
	Name     string `json:"name" binding:"required,max=64"`
	Leader   string `json:"leader" binding:"max=64"`
	Sort     int    `json:"sort"`
	Status   int8   `json:"status"`
}

func (h *DeptHandler) Create(c *gin.Context) {
	var req DeptSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	dept := model.SysDept{TenantID: middleware.CurrentTenantID(c), ParentID: req.ParentID, Name: req.Name, Leader: req.Leader, Sort: req.Sort, Status: req.Status}
	if dept.Status == 0 {
		dept.Status = 1
	}
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
	if req.ParentID == dept.ID {
		common.FailMsg(c, common.CodeParamInvalid, "上级部门不能是自身")
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
