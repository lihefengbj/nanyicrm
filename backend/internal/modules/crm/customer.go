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

type CustomerHandler struct {
	db *gorm.DB
}

type CustomerListItem struct {
	model.CrmCustomer
	Intent *IntentListResponse `json:"intent,omitempty"`
}

func NewCustomerHandler(db *gorm.DB) *CustomerHandler {
	return &CustomerHandler{db: db}
}

// @Summary  客户分页列表
// @Tags     CRM-客户
// @Description 需要权限：crm:customer:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer [get]
func (h *CustomerHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)
	name := strings.TrimSpace(c.Query("name"))
	status := strings.TrimSpace(c.Query("status"))
	level := strings.TrimSpace(c.Query("level"))

	query := middleware.TenantScope(c, h.db.Model(&model.CrmCustomer{}), "crm_customer")
	query = query.Joins(`LEFT JOIN crm_customer_intent ci
		ON ci.tenant_id = crm_customer.tenant_id
		AND ci.customer_id = crm_customer.id
		AND ci.deleted_at IS NULL`)
	query = query.Joins(`LEFT JOIN (
		SELECT tenant_id, customer_id, MAX(id) AS latest_id
		FROM crm_customer_intent_analysis
		WHERE deleted_at IS NULL
		GROUP BY tenant_id, customer_id
	) latest_ia
		ON latest_ia.tenant_id = crm_customer.tenant_id
		AND latest_ia.customer_id = crm_customer.id`)
	query = query.Joins(`LEFT JOIN crm_customer_intent_analysis ia
		ON ia.id = latest_ia.latest_id
		AND ia.deleted_at IS NULL`)
	if middleware.IsPrivileged(c) {
		if tid, err := strconv.ParseUint(c.Query("tenantId"), 10, 64); err == nil && tid > 0 {
			query = query.Where("crm_customer.tenant_id = ?", tid)
		}
	}
	if name != "" {
		query = query.Where("crm_customer.name LIKE ?", "%"+name+"%")
	}
	if level != "" {
		query = query.Where("crm_customer.level = ?", level)
	}
	if status != "" {
		if s, err := strconv.Atoi(status); err == nil {
			query = query.Where("crm_customer.status = ?", s)
		}
	}
	// mine=1 narrows the list to customers owned by the current user.
	if c.Query("mine") == "1" {
		query = query.Where("crm_customer.owner_id = ?", middleware.CurrentUserID(c))
	} else if oid, err := strconv.ParseUint(c.Query("ownerId"), 10, 64); err == nil && oid > 0 {
		query = query.Where("crm_customer.owner_id = ?", oid)
	}
	if value := strings.TrimSpace(c.Query("intentLevel")); value != "" {
		if value == "none" {
			query = query.Where("ci.id IS NULL")
		} else {
			query = query.Where("ci.intent_level = ?", value)
		}
	}
	if value := strings.TrimSpace(c.Query("intentStatus")); value != "" {
		switch value {
		case "unanalysed":
			query = query.Where("ia.id IS NULL")
		case "running", "success", "failed":
			query = query.Where("ia.status = ?", value)
		}
	}
	if value := strings.TrimSpace(c.Query("minScore")); value != "" {
		if score, err := strconv.Atoi(value); err == nil {
			query = query.Where("ci.intent_score >= ?", score)
		}
	}
	if value := strings.TrimSpace(c.Query("maxScore")); value != "" {
		if score, err := strconv.Atoi(value); err == nil {
			query = query.Where("ci.intent_score <= ?", score)
		}
	}
	if value := strings.TrimSpace(c.Query("minConfidence")); value != "" {
		if confidence, err := strconv.ParseFloat(value, 64); err == nil {
			query = query.Where("ci.confidence >= ?", confidence)
		}
	}
	if value := strings.TrimSpace(c.Query("followUpStatus")); value != "" {
		now := time.Now()
		switch value {
		case "today":
			start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			query = query.Where("ci.suggested_next_at >= ? AND ci.suggested_next_at < ?", start, start.AddDate(0, 0, 1))
		case "overdue":
			query = query.Where("ci.suggested_next_at < ?", now)
		case "future":
			query = query.Where("ci.suggested_next_at >= ?", now)
		case "none":
			query = query.Where("ci.suggested_next_at IS NULL")
		}
	}
	if value := strings.TrimSpace(c.Query("analyzedWithin")); value != "" {
		switch value {
		case "7d":
			query = query.Where("ci.analyzed_at >= ?", time.Now().AddDate(0, 0, -7))
		case "30d":
			query = query.Where("ci.analyzed_at >= ?", time.Now().AddDate(0, 0, -30))
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var customers []model.CrmCustomer
	if err := query.Preload("Owner").
		Order(customerIntentOrder(c.Query("intentSort"))).
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&customers).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	items := make([]CustomerListItem, len(customers))
	for i := range customers {
		items[i].CrmCustomer = customers[i]
	}
	if len(customers) > 0 {
		ids := make([]uint64, 0, len(customers))
		for _, customer := range customers {
			ids = append(ids, customer.ID)
		}
		var intents []model.CrmCustomerIntent
		if err := h.db.Where("tenant_id IN ? AND customer_id IN ?", customerTenantIDs(customers), ids).
			Find(&intents).Error; err != nil {
			common.Fail(c, common.CodeDBError)
			return
		}
		type intentKey struct {
			tenantID   uint64
			customerID uint64
		}
		intentMap := make(map[intentKey]model.CrmCustomerIntent, len(intents))
		for _, intent := range intents {
			intentMap[intentKey{tenantID: intent.TenantID, customerID: intent.CustomerID}] = intent
		}
		for i := range items {
			if intent, ok := intentMap[intentKey{tenantID: items[i].TenantID, customerID: items[i].ID}]; ok {
				items[i].Intent = intentListResponse(&intent)
			}
		}
	}
	common.OKPage(c, items, total, page.PageNum, page.PageSize)
}

func customerIntentOrder(sort string) string {
	switch strings.TrimSpace(sort) {
	case "score_asc":
		return "ci.intent_score IS NULL, ci.intent_score ASC, crm_customer.id DESC"
	case "score_desc":
		return "ci.intent_score IS NULL, ci.intent_score DESC, crm_customer.id DESC"
	case "confidence_desc":
		return "ci.confidence IS NULL, ci.confidence DESC, crm_customer.id DESC"
	case "analyzed_desc":
		return "ci.analyzed_at IS NULL, ci.analyzed_at DESC, crm_customer.id DESC"
	case "follow_up_asc":
		return "ci.suggested_next_at IS NULL, ci.suggested_next_at ASC, crm_customer.id DESC"
	default:
		return "crm_customer.id DESC"
	}
}

func customerTenantIDs(customers []model.CrmCustomer) []uint64 {
	ids := make([]uint64, 0, len(customers))
	seen := make(map[uint64]struct{}, len(customers))
	for _, customer := range customers {
		if _, ok := seen[customer.TenantID]; ok {
			continue
		}
		seen[customer.TenantID] = struct{}{}
		ids = append(ids, customer.TenantID)
	}
	return ids
}

// All returns the tenant's customers for dropdowns (id + name only).
// @Summary  全部客户（下拉用）
// @Tags     CRM-客户
// @Description 需要权限：crm:customer:list
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/all [get]
func (h *CustomerHandler) All(c *gin.Context) {
	var customers []model.CrmCustomer
	query := middleware.TenantScope(c, h.db.Model(&model.CrmCustomer{}), "crm_customer")
	if middleware.IsPrivileged(c) {
		if tid, err := strconv.ParseUint(c.Query("tenantId"), 10, 64); err == nil && tid > 0 {
			query = query.Where("crm_customer.tenant_id = ?", tid)
		}
	}
	if err := query.Select("id", "name").Order("id DESC").Limit(500).Find(&customers).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, customers)
}

type CustomerSaveRequest struct {
	TenantID uint64  `json:"tenantId"`
	Name     string  `json:"name" binding:"required,max=128"`
	Phone    string  `json:"phone" binding:"max=32"`
	Source   string  `json:"source" binding:"max=32"`
	Industry string  `json:"industry" binding:"max=64"`
	Level    string  `json:"level" binding:"omitempty,oneof=A B C"`
	Status   int8    `json:"status" binding:"oneof=1 2 3"`
	OwnerID  *uint64 `json:"ownerId"`
	Address  string  `json:"address" binding:"max=255"`
	Remark   string  `json:"remark" binding:"max=255"`
}

// @Summary  新增客户
// @Tags     CRM-客户
// @Description 需要权限：crm:customer:create
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer [post]
func (h *CustomerHandler) Create(c *gin.Context) {
	var req CustomerSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	tenantID, ok := resolveBusinessTenant(c, h.db, req.TenantID, 0)
	if !ok {
		return
	}
	ownerID, ok := resolveOwnerForTenant(c, h.db, req.OwnerID, tenantID)
	if !ok {
		return
	}
	customer := model.CrmCustomer{
		TenantID: tenantID,
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

// @Summary  编辑客户
// @Tags     CRM-客户
// @Description 需要权限：crm:customer:update
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/{id} [put]
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
	tenantID, ok := resolveBusinessTenant(c, h.db, req.TenantID, customer.TenantID)
	if !ok {
		return
	}
	ownerID, ok := resolveOwnerForTenant(c, h.db, req.OwnerID, tenantID)
	if !ok {
		return
	}
	customer.TenantID = tenantID
	customer.Name = req.Name
	customer.Phone = req.Phone
	customer.Source = req.Source
	customer.Industry = req.Industry
	customer.Level = req.Level
	customer.OwnerID = ownerID
	customer.Address = req.Address
	customer.Remark = req.Remark
	customer.Status = req.Status
	if err := h.db.Save(customer).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

// @Summary  删除客户（级联联系人与跟进）
// @Tags     CRM-客户
// @Description 需要权限：crm:customer:delete
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /crm/customer/{id} [delete]
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
	var refs int64
	if err := h.db.Model(&model.CrmOpportunity{}).Where("customer_id = ?", customer.ID).Count(&refs).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	if refs == 0 {
		if err := h.db.Model(&model.CrmContract{}).Where("customer_id = ?", customer.ID).Count(&refs).Error; err != nil {
			common.Fail(c, common.CodeDBError)
			return
		}
	}
	if refs > 0 {
		common.FailMsg(c, common.CodeParamInvalid, "客户存在商机或合同，不可删除")
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
