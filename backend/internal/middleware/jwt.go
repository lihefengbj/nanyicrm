package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

const (
	CtxUserID       = "currentUserID"
	CtxUsername     = "currentUsername"
	CtxIsSuper      = "currentIsSuper"
	CtxIsPrivileged = "currentIsPrivileged"
)

const (
	// RoleCodeSuperAdmin bypasses every permission check.
	RoleCodeSuperAdmin = "superAdmin"
	// RoleCodeAdmin sees data across tenants but stays subject to menu perms.
	RoleCodeAdmin = "admin"
	// UsernameSuperAdmin is hardcoded: even with no roles it keeps full access.
	UsernameSuperAdmin = "superAdmin"
)

// JWTAuth validates the Bearer access token, resolves the user, their tenant
// and privilege flags, and stores identity in context. Disabled/expired
// tenants are rejected here so a mid-session lockout takes effect immediately.
func JWTAuth(db *gorm.DB, signingKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			common.Abort(c, common.CodeUnauthorized)
			return
		}
		claims, err := common.ParseToken(signingKey, strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			if err == common.ErrTokenExpired {
				common.Abort(c, common.CodeTokenExpired)
			} else {
				common.Abort(c, common.CodeUnauthorized)
			}
			return
		}

		var user model.SysUser
		if err := db.Select("id", "username", "tenant_id", "status").First(&user, claims.UserID).Error; err != nil {
			common.Abort(c, common.CodeUnauthorized)
			return
		}
		if user.Status != 1 {
			common.Abort(c, common.CodeAccountDisabled)
			return
		}
		if user.TenantID != 0 {
			var tenant model.SysTenant
			if err := db.Select("id", "status", "expire_at").First(&tenant, user.TenantID).Error; err != nil {
				common.Abort(c, common.CodeUnauthorized)
				return
			}
			if !tenant.Usable() {
				common.Abort(c, common.CodeTenantDisabled)
				return
			}
		}

		isSuper, isPrivileged := loadPrivilegeFlags(db, user.ID)
		if user.Username == UsernameSuperAdmin {
			// Hardcoded fallback: the built-in superAdmin account can never be
			// locked out, even if its roles are removed.
			isSuper, isPrivileged = true, true
		}

		c.Set(CtxUserID, user.ID)
		c.Set(CtxUsername, user.Username)
		SetTenantID(c, user.TenantID)
		c.Set(CtxIsSuper, isSuper)
		c.Set(CtxIsPrivileged, isPrivileged)
		c.Next()
	}
}

// loadPrivilegeFlags derives (isSuper, isPrivileged) from the user's enabled
// roles: superAdmin -> super; admin -> privileged (cross-tenant data).
func loadPrivilegeFlags(db *gorm.DB, userID uint64) (bool, bool) {
	var codes []string
	db.Model(&model.SysRole{}).
		Joins("JOIN sys_user_role ur ON ur.role_id = sys_role.id").
		Where("ur.user_id = ? AND sys_role.status = 1", userID).
		Pluck("sys_role.code", &codes)
	isSuper, isAdmin := false, false
	for _, code := range codes {
		if code == RoleCodeSuperAdmin {
			isSuper = true
		}
		if code == RoleCodeAdmin {
			isAdmin = true
		}
	}
	return isSuper, isSuper || isAdmin
}

func CurrentUserID(c *gin.Context) uint64 {
	if v, ok := c.Get(CtxUserID); ok {
		if id, ok := v.(uint64); ok {
			return id
		}
	}
	return 0
}

func CurrentUsername(c *gin.Context) string {
	if v, ok := c.Get(CtxUsername); ok {
		if name, ok := v.(string); ok {
			return name
		}
	}
	return ""
}
