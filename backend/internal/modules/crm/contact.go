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

type ContactHandler struct {
	db *gorm.DB
}

func NewContactHandler(db *gorm.DB) *ContactHandler {
	return &ContactHandler{db: db}
}

// @Summary  联系人分页列表
// @Tags     CRM-联系人
// @Description 需要权限：crm:contact:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/contact [get]
func (h *ContactHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)
	name := strings.TrimSpace(c.Query("name"))

	query := middleware.TenantScope(c, h.db.Model(&model.CrmContact{}), "crm_contact")
	if name != "" {
		query = query.Where("crm_contact.name LIKE ?", "%"+name+"%")
	}
	if cid, err := strconv.ParseUint(c.Query("customerId"), 10, 64); err == nil && cid > 0 {
		query = query.Where("customer_id = ?", cid)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var contacts []model.CrmContact
	if err := query.Preload("Customer").
		Order("crm_contact.is_primary DESC, crm_contact.id DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&contacts).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, contacts, total, page.PageNum, page.PageSize)
}

type ContactSaveRequest struct {
	CustomerID uint64 `json:"customerId" binding:"required"`
	Name       string `json:"name" binding:"required,max=64"`
	Phone      string `json:"phone" binding:"max=32"`
	Email      string `json:"email" binding:"omitempty,email,max=128"`
	Position   string `json:"position" binding:"max=64"`
	IsPrimary  int8   `json:"isPrimary" binding:"oneof=0 1"`
	Remark     string `json:"remark" binding:"max=255"`
}

// checkCustomer verifies the target customer exists in the caller's tenant.
func (h *ContactHandler) checkCustomer(c *gin.Context, customerID uint64) (*model.CrmCustomer, bool) {
	return findCustomerInTenant(c, h.db, customerID)
}

// @Summary  新增联系人
// @Tags     CRM-联系人
// @Description 需要权限：crm:contact:create
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/contact [post]
func (h *ContactHandler) Create(c *gin.Context) {
	var req ContactSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	customer, ok := h.checkCustomer(c, req.CustomerID)
	if !ok {
		return
	}
	contact := model.CrmContact{
		TenantID:   customer.TenantID,
		CustomerID: req.CustomerID,
		Name:       req.Name,
		Phone:      req.Phone,
		Email:      req.Email,
		Position:   req.Position,
		IsPrimary:  req.IsPrimary,
		Remark:     req.Remark,
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if contact.IsPrimary == 1 {
			if err := tx.Model(&model.CrmContact{}).
				Where("tenant_id = ? AND customer_id = ?", contact.TenantID, contact.CustomerID).
				Update("is_primary", 0).Error; err != nil {
				return err
			}
		}
		return tx.Create(&contact).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": contact.ID})
}

func (h *ContactHandler) findInTenant(c *gin.Context, id uint64) (*model.CrmContact, bool) {
	var contact model.CrmContact
	query := h.db
	if !middleware.IsPrivileged(c) {
		query = query.Where("tenant_id = ?", middleware.CurrentTenantID(c))
	}
	if err := query.First(&contact, id).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return nil, false
	}
	return &contact, true
}

// @Summary  编辑联系人
// @Tags     CRM-联系人
// @Description 需要权限：crm:contact:update
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/contact/{id} [put]
func (h *ContactHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	contact, ok := h.findInTenant(c, id)
	if !ok {
		return
	}
	var req ContactSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	customer, ok := h.checkCustomer(c, req.CustomerID)
	if !ok {
		return
	}
	contact.TenantID = customer.TenantID
	contact.CustomerID = req.CustomerID
	contact.Name = req.Name
	contact.Phone = req.Phone
	contact.Email = req.Email
	contact.Position = req.Position
	contact.IsPrimary = req.IsPrimary
	contact.Remark = req.Remark
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if contact.IsPrimary == 1 {
			if err := tx.Model(&model.CrmContact{}).
				Where("tenant_id = ? AND customer_id = ? AND id <> ?", contact.TenantID, contact.CustomerID, contact.ID).
				Update("is_primary", 0).Error; err != nil {
				return err
			}
		}
		return tx.Save(contact).Error
	}); err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

// @Summary  删除联系人
// @Tags     CRM-联系人
// @Description 需要权限：crm:contact:delete
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/contact/{id} [delete]
func (h *ContactHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	contact, ok := h.findInTenant(c, id)
	if !ok {
		return
	}
	if err := h.db.Delete(contact).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}
