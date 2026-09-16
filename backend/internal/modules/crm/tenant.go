package crm

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

func resolveBusinessTenant(c *gin.Context, db *gorm.DB, requested uint64, existing uint64) (uint64, bool) {
	if !middleware.IsPrivileged(c) {
		return middleware.CurrentTenantID(c), true
	}
	tenantID := requested
	if tenantID == 0 && existing != 0 {
		tenantID = existing
	}
	if tenantID == 0 {
		return 0, true
	}
	var count int64
	if err := db.Model(&model.SysTenant{}).Where("id = ? AND status = 1", tenantID).Count(&count).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return 0, false
	}
	if count == 0 {
		common.Fail(c, common.CodeTenantNotFound)
		return 0, false
	}
	return tenantID, true
}

func resolveOwnerForTenant(c *gin.Context, db *gorm.DB, requested *uint64, tenantID uint64) (*uint64, bool) {
	ownerID := requested
	if ownerID == nil || *ownerID == 0 {
		if middleware.CurrentTenantID(c) != tenantID {
			return nil, true
		}
		uid := middleware.CurrentUserID(c)
		ownerID = &uid
	}
	var owner model.SysUser
	if err := db.Select("id", "tenant_id", "status").First(&owner, *ownerID).Error; err != nil || owner.Status != 1 {
		common.FailMsg(c, common.CodeParamInvalid, "归属人不存在或已停用")
		return nil, false
	}
	if owner.TenantID != tenantID {
		common.FailMsg(c, common.CodeParamInvalid, "归属人不属于业务数据所在租户")
		return nil, false
	}
	return ownerID, true
}
