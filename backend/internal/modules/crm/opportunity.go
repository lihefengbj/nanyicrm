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

type OpportunityHandler struct {
	db *gorm.DB
}

func NewOpportunityHandler(db *gorm.DB) *OpportunityHandler {
	return &OpportunityHandler{db: db}
}

func (h *OpportunityHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)
	name := strings.TrimSpace(c.Query("name"))

	query := middleware.TenantScope(c, h.db.Model(&model.CrmOpportunity{}), "crm_opportunity")
	if name != "" {
		query = query.Where("crm_opportunity.name LIKE ?", "%"+name+"%")
	}
	if cid, err := strconv.ParseUint(c.Query("customerId"), 10, 64); err == nil && cid > 0 {
		query = query.Where("customer_id = ?", cid)
	}
	if s, err := strconv.Atoi(c.Query("stage")); err == nil && s > 0 {
		query = query.Where("stage = ?", s)
	}
	if c.Query("mine") == "1" {
		query = query.Where("owner_id = ?", middleware.CurrentUserID(c))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var list []model.CrmOpportunity
	if err := query.Preload("Customer").Preload("Owner").
		Order("crm_opportunity.id DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&list).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, list, total, page.PageNum, page.PageSize)
}

// All returns open opportunities (stages 1-4) for dropdowns.
func (h *OpportunityHandler) All(c *gin.Context) {
	var list []model.CrmOpportunity
	query := middleware.TenantScope(c, h.db.Model(&model.CrmOpportunity{}), "crm_opportunity")
	if cid, err := strconv.ParseUint(c.Query("customerId"), 10, 64); err == nil && cid > 0 {
		query = query.Where("customer_id = ?", cid)
	}
	if err := query.Select("id", "name", "customer_id").
		Where("stage BETWEEN 1 AND 4").
		Order("id DESC").Limit(500).Find(&list).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, list)
}

type OpportunitySaveRequest struct {
	CustomerID uint64     `json:"customerId" binding:"required"`
	Name       string     `json:"name" binding:"required,max=128"`
	Stage      int8       `json:"stage"`
	Amount     float64    `json:"amount" binding:"gte=0"`
	ExpectDate *time.Time `json:"expectDate"`
	OwnerID    *uint64    `json:"ownerId"`
	Remark     string     `json:"remark" binding:"max=255"`
}

func (h *OpportunityHandler) resolveOwner(c *gin.Context, req *OpportunitySaveRequest) (*uint64, bool) {
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

func validStage(s int8) int8 {
	if s < 1 || s > 6 {
		return 1
	}
	return s
}

func (h *OpportunityHandler) Create(c *gin.Context) {
	var req OpportunitySaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if _, ok := findCustomerInTenant(c, h.db, req.CustomerID); !ok {
		return
	}
	ownerID, ok := h.resolveOwner(c, &req)
	if !ok {
		return
	}
	opp := model.CrmOpportunity{
		TenantID:   middleware.CurrentTenantID(c),
		CustomerID: req.CustomerID,
		Name:       req.Name,
		Stage:      validStage(req.Stage),
		Amount:     req.Amount,
		ExpectDate: req.ExpectDate,
		OwnerID:    ownerID,
		Remark:     req.Remark,
	}
	if err := h.db.Create(&opp).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": opp.ID})
}

func (h *OpportunityHandler) findInTenant(c *gin.Context, id uint64) (*model.CrmOpportunity, bool) {
	var opp model.CrmOpportunity
	query := h.db
	if !middleware.IsPrivileged(c) {
		query = query.Where("tenant_id = ?", middleware.CurrentTenantID(c))
	}
	if err := query.First(&opp, id).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return nil, false
	}
	return &opp, true
}

func (h *OpportunityHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	opp, ok := h.findInTenant(c, id)
	if !ok {
		return
	}
	var req OpportunitySaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if _, ok := findCustomerInTenant(c, h.db, req.CustomerID); !ok {
		return
	}
	ownerID, ok := h.resolveOwner(c, &req)
	if !ok {
		return
	}
	opp.CustomerID = req.CustomerID
	opp.Name = req.Name
	opp.Stage = validStage(req.Stage)
	opp.Amount = req.Amount
	opp.ExpectDate = req.ExpectDate
	opp.OwnerID = ownerID
	opp.Remark = req.Remark
	if err := h.db.Save(opp).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

func (h *OpportunityHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	opp, ok := h.findInTenant(c, id)
	if !ok {
		return
	}
	// Contracts keep their own lifecycle; only unlink them from the opportunity.
	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.CrmContract{}).
			Where("opportunity_id = ?", opp.ID).
			Update("opportunity_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(opp).Error
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}
