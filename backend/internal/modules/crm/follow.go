package crm

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type FollowUpHandler struct {
	db *gorm.DB
}

func NewFollowUpHandler(db *gorm.DB) *FollowUpHandler {
	return &FollowUpHandler{db: db}
}

// @Summary  跟进记录分页列表
// @Tags     CRM-跟进
// @Description 需要权限：crm:follow:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/follow [get]
func (h *FollowUpHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)

	query := middleware.TenantScope(c, h.db.Model(&model.CrmFollowUp{}), "crm_follow_up")
	if cid, err := strconv.ParseUint(c.Query("customerId"), 10, 64); err == nil && cid > 0 {
		query = query.Where("customer_id = ?", cid)
	}
	if t, err := strconv.Atoi(c.Query("type")); err == nil && t > 0 {
		query = query.Where("type = ?", t)
	}
	// mine=1 narrows to follow-ups written by the current user.
	if c.Query("mine") == "1" {
		query = query.Where("creator_id = ?", middleware.CurrentUserID(c))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var follows []model.CrmFollowUp
	if err := query.Preload("Customer").Preload("Contact").
		Order("crm_follow_up.id DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&follows).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, follows, total, page.PageNum, page.PageSize)
}

type FollowUpSaveRequest struct {
	CustomerID uint64     `json:"customerId" binding:"required"`
	ContactID  *uint64    `json:"contactId"`
	Type       int8       `json:"type"`
	Content    string     `json:"content" binding:"required,max=1024"`
	NextAt     *time.Time `json:"nextAt"`
}

func (h *FollowUpHandler) validateRefs(c *gin.Context, req *FollowUpSaveRequest) bool {
	customer, ok := findCustomerInTenant(c, h.db, req.CustomerID)
	if !ok {
		return false
	}
	if req.ContactID != nil && *req.ContactID > 0 {
		var contact model.CrmContact
		if err := h.db.First(&contact, *req.ContactID).Error; err != nil || contact.CustomerID != customer.ID {
			common.FailMsg(c, common.CodeParamInvalid, "联系人不属于该客户")
			return false
		}
	}
	return true
}

// @Summary  新增跟进记录
// @Tags     CRM-跟进
// @Description 需要权限：crm:follow:create
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/follow [post]
func (h *FollowUpHandler) Create(c *gin.Context) {
	var req FollowUpSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if !h.validateRefs(c, &req) {
		return
	}
	if req.Type < 1 || req.Type > 4 {
		req.Type = 1
	}
	follow := model.CrmFollowUp{
		TenantID:   middleware.CurrentTenantID(c),
		CustomerID: req.CustomerID,
		ContactID:  req.ContactID,
		Type:       req.Type,
		Content:    req.Content,
		NextAt:     req.NextAt,
		CreatorID:  middleware.CurrentUserID(c),
		Creator:    middleware.CurrentUsername(c),
	}
	if err := h.db.Create(&follow).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": follow.ID})
}

// findOwned loads a follow-up: tenant users may only touch their own records.
func (h *FollowUpHandler) findOwned(c *gin.Context, id uint64) (*model.CrmFollowUp, bool) {
	var follow model.CrmFollowUp
	query := h.db
	if !middleware.IsPrivileged(c) {
		query = query.Where("tenant_id = ? AND creator_id = ?",
			middleware.CurrentTenantID(c), middleware.CurrentUserID(c))
	}
	if err := query.First(&follow, id).Error; err != nil {
		common.Fail(c, common.CodeRecordNotFound)
		return nil, false
	}
	return &follow, true
}

// @Summary  编辑跟进记录（仅本人）
// @Tags     CRM-跟进
// @Description 需要权限：crm:follow:update
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/follow/{id} [put]
func (h *FollowUpHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	follow, ok := h.findOwned(c, id)
	if !ok {
		return
	}
	var req FollowUpSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if !h.validateRefs(c, &req) {
		return
	}
	if req.Type < 1 || req.Type > 4 {
		req.Type = 1
	}
	follow.CustomerID = req.CustomerID
	follow.ContactID = req.ContactID
	follow.Type = req.Type
	follow.Content = req.Content
	follow.NextAt = req.NextAt
	if err := h.db.Save(follow).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

// @Summary  删除跟进记录（仅本人）
// @Tags     CRM-跟进
// @Description 需要权限：crm:follow:delete
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/follow/{id} [delete]
func (h *FollowUpHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	follow, ok := h.findOwned(c, id)
	if !ok {
		return
	}
	if err := h.db.Delete(follow).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}
