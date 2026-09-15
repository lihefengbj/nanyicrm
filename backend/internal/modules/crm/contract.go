package crm

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

type ContractHandler struct {
	db *gorm.DB
}

func NewContractHandler(db *gorm.DB) *ContractHandler {
	return &ContractHandler{db: db}
}

// @Summary  合同分页列表
// @Tags     CRM-合同
// @Description 需要权限：crm:contract:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/contract [get]
func (h *ContractHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)
	keyword := strings.TrimSpace(c.Query("keyword"))

	query := middleware.TenantScope(c, h.db.Model(&model.CrmContract{}), "crm_contract")
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("crm_contract.name LIKE ? OR crm_contract.code LIKE ?", like, like)
	}
	if cid, err := strconv.ParseUint(c.Query("customerId"), 10, 64); err == nil && cid > 0 {
		query = query.Where("customer_id = ?", cid)
	}
	if s, err := strconv.Atoi(c.Query("status")); err == nil && s > 0 {
		query = query.Where("status = ?", s)
	}
	if c.Query("mine") == "1" {
		query = query.Where("owner_id = ?", middleware.CurrentUserID(c))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var list []model.CrmContract
	if err := query.Preload("Customer").Preload("Opportunity").Preload("Owner").
		Order("crm_contract.id DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&list).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, list, total, page.PageNum, page.PageSize)
}

type ContractSaveRequest struct {
	Code          string     `json:"code" binding:"max=64"`
	Name          string     `json:"name" binding:"required,max=128"`
	CustomerID    uint64     `json:"customerId" binding:"required"`
	OpportunityID *uint64    `json:"opportunityId"`
	Amount        float64    `json:"amount" binding:"gte=0"`
	SignDate      *time.Time `json:"signDate"`
	StartDate     *time.Time `json:"startDate"`
	EndDate       *time.Time `json:"endDate"`
	Status        int8       `json:"status"`
	OwnerID       *uint64    `json:"ownerId"`
	Remark        string     `json:"remark" binding:"max=255"`
}

// validateRefs checks customer/opportunity ownership and code uniqueness
// within the tenant. excludeID ignores the record being updated.
func (h *ContractHandler) validateRefs(c *gin.Context, req *ContractSaveRequest, excludeID uint64) bool {
	if _, ok := findCustomerInTenant(c, h.db, req.CustomerID); !ok {
		return false
	}
	if req.OpportunityID != nil && *req.OpportunityID > 0 {
		var opp model.CrmOpportunity
		if err := h.db.First(&opp, *req.OpportunityID).Error; err != nil || opp.CustomerID != req.CustomerID {
			common.FailMsg(c, common.CodeParamInvalid, "商机不属于该客户")
			return false
		}
	}
	if req.Code != "" {
		tenantID := middleware.CurrentTenantID(c)
		var dup int64
		h.db.Model(&model.CrmContract{}).
			Where("tenant_id = ? AND code = ? AND id <> ?", tenantID, req.Code, excludeID).
			Count(&dup)
		if dup > 0 {
			common.FailMsg(c, common.CodeParamInvalid, "合同编号已存在")
			return false
		}
	}
	return true
}

func validContractStatus(s int8) int8 {
	if s < 1 || s > 4 {
		return 1
	}
	return s
}

// @Summary  新增合同（关联商机自动赢单）
// @Tags     CRM-合同
// @Description 需要权限：crm:contract:create
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/contract [post]
func (h *ContractHandler) Create(c *gin.Context) {
	var req ContractSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if !h.validateRefs(c, &req, 0) {
		return
	}
	uid := middleware.CurrentUserID(c)
	ownerID := req.OwnerID
	if ownerID == nil || *ownerID == 0 {
		ownerID = &uid
	}
	contract := model.CrmContract{
		TenantID:      middleware.CurrentTenantID(c),
		Code:          req.Code,
		Name:          req.Name,
		CustomerID:    req.CustomerID,
		OpportunityID: req.OpportunityID,
		Amount:        req.Amount,
		SignDate:      req.SignDate,
		StartDate:     req.StartDate,
		EndDate:       req.EndDate,
		Status:        validContractStatus(req.Status),
		OwnerID:       ownerID,
		Remark:        req.Remark,
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&contract).Error; err != nil {
			return err
		}
		// A contract born from an opportunity wins it.
		if contract.OpportunityID != nil && *contract.OpportunityID > 0 {
			return tx.Model(&model.CrmOpportunity{}).
				Where("id = ? AND stage BETWEEN 1 AND 4", *contract.OpportunityID).
				Update("stage", 5).Error
		}
		return nil
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": contract.ID})
}

func (h *ContractHandler) findInTenant(c *gin.Context, id uint64) (*model.CrmContract, bool) {
	var contract model.CrmContract
	query := h.db
	if !middleware.IsPrivileged(c) {
		query = query.Where("tenant_id = ?", middleware.CurrentTenantID(c))
	}
	if err := query.First(&contract, id).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return nil, false
	}
	return &contract, true
}

// @Summary  编辑合同
// @Tags     CRM-合同
// @Description 需要权限：crm:contract:update
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/contract/{id} [put]
func (h *ContractHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	contract, ok := h.findInTenant(c, id)
	if !ok {
		return
	}
	var req ContractSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if !h.validateRefs(c, &req, contract.ID) {
		return
	}
	contract.Code = req.Code
	contract.Name = req.Name
	contract.CustomerID = req.CustomerID
	contract.OpportunityID = req.OpportunityID
	contract.Amount = req.Amount
	contract.SignDate = req.SignDate
	contract.StartDate = req.StartDate
	contract.EndDate = req.EndDate
	contract.Status = validContractStatus(req.Status)
	if req.OwnerID != nil && *req.OwnerID > 0 {
		contract.OwnerID = req.OwnerID
	}
	contract.Remark = req.Remark
	if err := h.db.Save(contract).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

// @Summary  删除合同
// @Tags     CRM-合同
// @Description 需要权限：crm:contract:delete
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/contract/{id} [delete]
func (h *ContractHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	contract, ok := h.findInTenant(c, id)
	if !ok {
		return
	}
	if err := h.db.Delete(contract).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}
