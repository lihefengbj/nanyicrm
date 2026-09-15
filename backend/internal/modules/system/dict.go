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

type DictHandler struct {
	db *gorm.DB
}

func NewDictHandler(db *gorm.DB) *DictHandler {
	return &DictHandler{db: db}
}

func (h *DictHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)
	keyword := strings.TrimSpace(c.Query("keyword"))

	query := middleware.TenantScope(c, h.db.Model(&model.SysDict{}), "sys_dict")
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR type LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var dicts []model.SysDict
	if err := query.Preload("Items", func(db *gorm.DB) *gorm.DB { return db.Order("sort, id") }).
		Order("sys_dict.id DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&dicts).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, dicts, total, page.PageNum, page.PageSize)
}

// Items returns enabled items of one dict type for dropdowns. Available to
// any authenticated user (no button perm), still tenant scoped with
// platform-level (tenant 0) dictionaries shared to everyone.
func (h *DictHandler) Items(c *gin.Context) {
	dictType := strings.TrimSpace(c.Param("type"))
	if dictType == "" {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	var dict model.SysDict
	tenantID := middleware.CurrentTenantID(c)
	if middleware.IsPrivileged(c) {
		tenantID = 0 // privileged users default to platform dictionaries
	}
	if err := h.db.Where("type = ? AND tenant_id IN (0, ?) AND status = 1", dictType, tenantID).
		Order("tenant_id DESC").First(&dict).Error; err != nil {
		common.OK(c, []model.SysDictItem{})
		return
	}
	var items []model.SysDictItem
	if err := h.db.Where("dict_id = ? AND status = 1", dict.ID).Order("sort, id").Find(&items).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, items)
}

type DictSaveRequest struct {
	Name   string `json:"name" binding:"required,max=64"`
	Type   string `json:"type" binding:"required,max=64"`
	Status int8   `json:"status"`
	Remark string `json:"remark" binding:"max=255"`
}

func (h *DictHandler) Create(c *gin.Context) {
	var req DictSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	tenantID := middleware.CurrentTenantID(c)
	var dup int64
	h.db.Model(&model.SysDict{}).Where("tenant_id = ? AND type = ?", tenantID, req.Type).Count(&dup)
	if dup > 0 {
		common.FailMsg(c, common.CodeParamInvalid, "字典类型已存在")
		return
	}
	dict := model.SysDict{TenantID: tenantID, Name: req.Name, Type: req.Type, Status: req.Status, Remark: req.Remark}
	if dict.Status == 0 {
		dict.Status = 1
	}
	if err := h.db.Create(&dict).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": dict.ID})
}

func (h *DictHandler) findInTenant(c *gin.Context, id uint64) (*model.SysDict, bool) {
	var dict model.SysDict
	query := h.db
	if !middleware.IsPrivileged(c) {
		query = query.Where("tenant_id = ?", middleware.CurrentTenantID(c))
	}
	if err := query.First(&dict, id).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return nil, false
	}
	return &dict, true
}

func (h *DictHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	dict, ok := h.findInTenant(c, id)
	if !ok {
		return
	}
	var req DictSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if req.Type != dict.Type {
		var dup int64
		h.db.Model(&model.SysDict{}).Where("tenant_id = ? AND type = ? AND id <> ?", dict.TenantID, req.Type, dict.ID).Count(&dup)
		if dup > 0 {
			common.FailMsg(c, common.CodeParamInvalid, "字典类型已存在")
			return
		}
	}
	dict.Name = req.Name
	dict.Type = req.Type
	dict.Status = req.Status
	dict.Remark = req.Remark
	if err := h.db.Save(dict).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

func (h *DictHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	dict, ok := h.findInTenant(c, id)
	if !ok {
		return
	}
	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("dict_id = ?", dict.ID).Delete(&model.SysDictItem{}).Error; err != nil {
			return err
		}
		return tx.Delete(dict).Error
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

// ---- dict items ----

type DictItemSaveRequest struct {
	DictID uint64 `json:"dictId" binding:"required"`
	Label  string `json:"label" binding:"required,max=64"`
	Value  string `json:"value" binding:"required,max=64"`
	Sort   int    `json:"sort"`
	Status int8   `json:"status"`
}

func (h *DictHandler) CreateItem(c *gin.Context) {
	var req DictItemSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if _, ok := h.findInTenant(c, req.DictID); !ok {
		return
	}
	item := model.SysDictItem{DictID: req.DictID, Label: req.Label, Value: req.Value, Sort: req.Sort, Status: req.Status}
	if item.Status == 0 {
		item.Status = 1
	}
	if err := h.db.Create(&item).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": item.ID})
}

func (h *DictHandler) findItemInTenant(c *gin.Context, id uint64) (*model.SysDictItem, bool) {
	var item model.SysDictItem
	if err := h.db.First(&item, id).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return nil, false
	}
	if _, ok := h.findInTenant(c, item.DictID); !ok {
		return nil, false
	}
	return &item, true
}

func (h *DictHandler) UpdateItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	item, ok := h.findItemInTenant(c, id)
	if !ok {
		return
	}
	var req DictItemSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	item.Label = req.Label
	item.Value = req.Value
	item.Sort = req.Sort
	item.Status = req.Status
	if err := h.db.Save(item).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

func (h *DictHandler) DeleteItem(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	item, ok := h.findItemInTenant(c, id)
	if !ok {
		return
	}
	if err := h.db.Delete(item).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}
