package crm

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

// DashboardHandler powers the workbench: aggregate counters over the CRM
// domain, scoped to the caller's tenant (privileged users see everything).
type DashboardHandler struct {
	db *gorm.DB
}

func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{db: db}
}

type stageStat struct {
	Stage int8    `json:"stage"`
	Count int64   `json:"count"`
	Total float64 `json:"total"`
}

func (h *DashboardHandler) Summary(c *gin.Context) {
	uid := middleware.CurrentUserID(c)
	scoped := func(m interface{}, table string) *gorm.DB {
		return middleware.TenantScope(c, h.db.Model(m), table)
	}

	var customerTotal, myCustomerTotal int64
	if err := scoped(&model.CrmCustomer{}, "crm_customer").Count(&customerTotal).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	scoped(&model.CrmCustomer{}, "crm_customer").Where("owner_id = ?", uid).Count(&myCustomerTotal)

	var openOppCount int64
	var openOppAmount float64
	scoped(&model.CrmOpportunity{}, "crm_opportunity").
		Where("stage BETWEEN 1 AND 4").
		Select("COUNT(*)", "COALESCE(SUM(amount),0)").
		Row().
		Scan(&openOppCount, &openOppAmount)

	var contractTotal int64
	var contractAmount float64
	scoped(&model.CrmContract{}, "crm_contract").
		Where("status IN (2,3)").
		Select("COUNT(*)", "COALESCE(SUM(amount),0)").
		Row().
		Scan(&contractTotal, &contractAmount)

	weekAgo := time.Now().AddDate(0, 0, -7)
	var followWeekCount int64
	scoped(&model.CrmFollowUp{}, "crm_follow_up").
		Where("crm_follow_up.created_at >= ?", weekAgo).
		Count(&followWeekCount)

	// Pending follow-ups: customers whose latest planned follow time is due.
	var pendingFollowCount int64
	scoped(&model.CrmFollowUp{}, "crm_follow_up").
		Where("next_at IS NOT NULL AND next_at <= ?", time.Now()).
		Count(&pendingFollowCount)

	var stages []stageStat
	scoped(&model.CrmOpportunity{}, "crm_opportunity").
		Select("stage, COUNT(*) AS count, COALESCE(SUM(amount),0) AS total").
		Group("stage").
		Scan(&stages)

	common.OK(c, gin.H{
		"customerTotal":      customerTotal,
		"myCustomerTotal":    myCustomerTotal,
		"openOppCount":       openOppCount,
		"openOppAmount":      openOppAmount,
		"contractTotal":      contractTotal,
		"contractAmount":     contractAmount,
		"followWeekCount":    followWeekCount,
		"pendingFollowCount": pendingFollowCount,
		"opportunityStages":  stages,
	})
}
