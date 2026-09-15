package crm

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type CustomerHandler struct {
	db *gorm.DB
}

func NewCustomerHandler(db *gorm.DB) *CustomerHandler {
	return &CustomerHandler{db: db}
}

func (h *CustomerHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)
	name := strings.TrimSpace(c.Query("name"))
	status := strings.TrimSpace(c.Query("status"))
	level := strings.TrimSpace(c.Query("level"))

	query := middleware.TenantScope(c, h.db.Model(&model.CrmCustomer{}), "crm_customer")
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if level != "" {
		query = query.Where("level = ?", level)
	}
	if status != "" {
		if s, err := strconv.Atoi(status); err == nil {
			query = query.Where("status = ?", s)
		}
	}
	// mine=1 narrows the list to customers owned by the current user.
	if c.Query("mine") == "1" {
		query = query.Where("owner_id = ?", middleware.CurrentUserID(c))
	} else if oid, err := strconv.ParseUint(c.Query("ownerId"), 10, 64); err == nil && oid > 0 {
		query = query.Where("owner_id = ?", oid)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var customers []model.CrmCustomer
	if err := query.Preload("Owner").
		Order("crm_customer.id DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&customers).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, customers, total, page.PageNum, page.PageSize)
}

// All returns the tenant's customers for dropdowns (id + name only).
func (h *CustomerHandler) All(c *gin.Context) {
	var customers []model.CrmCustomer
	query := middleware.TenantScope(c, h.db.Model(&model.CrmCustomer{}), "crm_customer")
	if err := query.Select("id", "name").Order("id DESC").Limit(500).Find(&customers).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, customers)
}

type CustomerSaveRequest struct {
	Name     string  `json:"name" binding:"required,max=128"`
	Phone    string  `json:"phone" binding:"max=32"`
	Source   string  `json:"source" binding:"max=32"`
	Industry string  `json:"industry" binding:"max=64"`
	Level    string  `json:"level" binding:"max=8"`
	Status   int8    `json:"status"`
	OwnerID  *uint64 `json:"ownerId"`
	Address  string  `json:"address" binding:"max=255"`
	Remark   string  `json:"remark" binding:"max=255"`
}

// resolveOwner defaults the owner to the current user and verifies the owner
// belongs to the same tenant.
func (h *CustomerHandler) resolveOwner(c *gin.Context, req *CustomerSaveRequest) (*uint64, bool) {
	ownerID := req.OwnerID
	if ownerID == nil || *ownerID == 0 {
		uid := middleware.CurrentUserID(c)
		ownerID = &uid
	}
	var owner model.SysUser
	if err := h.db.Select("id", "tenant_id").First(&owner, *ownerID).Error; err != nil {
		common.FailMsg(c, common.CodeParamInvalid, "归属人不存在")
		return nil, false
	}
	if !middleware.IsPrivileged(c) && owner.TenantID != middleware.CurrentTenantID(c) {
		common.FailMsg(c, common.CodeParamInvalid, "归属人不属于当前租户")
		return nil, false
	}
	return ownerID, true
}

func (h *CustomerHandler) Create(c *gin.Context) {
	var req CustomerSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	ownerID, ok := h.resolveOwner(c, &req)
	if !ok {
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}
	customer := model.CrmCustomer{
		TenantID: middleware.CurrentTenantID(c),
		Name:     req.Name,
		Phone:    req.Phone,
		Source:   req.Source,
		Industry: req.Industry,
		Level:    req.Level,
		Status:   req.Status,
		OwnerID:  ownerID,
		Address:  req.Address,
		Remark:   req.Remark,
	}
	if err := h.db.Create(&customer).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": customer.ID})
}

// findInTenant loads a customer scoped to the caller's tenant (privileged = any).
func findCustomerInTenant(c *gin.Context, db *gorm.DB, id uint64) (*model.CrmCustomer, bool) {
	var customer model.CrmCustomer
	query := db
	if !middleware.IsPrivileged(c) {
		query = query.Where("tenant_id = ?", middleware.CurrentTenantID(c))
	}
	if err := query.First(&customer, id).Error; err != nil {
		common.Fail(c, common.CodeCustomerNotFound)
		return nil, false
	}
	return &customer, true
}

func (h *CustomerHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	customer, ok := findCustomerInTenant(c, h.db, id)
	if !ok {
		return
	}
	var req CustomerSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	ownerID, ok := h.resolveOwner(c, &req)
	if !ok {
		return
	}
	customer.Name = req.Name
	customer.Phone = req.Phone
	customer.Source = req.Source
	customer.Industry = req.Industry
	customer.Level = req.Level
	customer.OwnerID = ownerID
	customer.Address = req.Address
	customer.Remark = req.Remark
	if req.Status >= 1 && req.Status <= 3 {
		customer.Status = req.Status
	}
	if err := h.db.Save(customer).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

func (h *CustomerHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	customer, ok := findCustomerInTenant(c, h.db, id)
	if !ok {
		return
	}
	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("customer_id = ?", customer.ID).Delete(&model.CrmFollowUp{}).Error; err != nil {
			return err
		}
		if err := tx.Where("customer_id = ?", customer.ID).Delete(&model.CrmContact{}).Error; err != nil {
			return err
		}
		return tx.Delete(customer).Error
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}
