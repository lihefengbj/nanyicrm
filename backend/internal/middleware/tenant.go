package middleware

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TenantIDKey is the context key holding the current request's tenant scope.
const TenantIDKey = "currentTenantID"

// IsSuper reports whether the current user is the built-in superAdmin (by
// role or by hardcoded username). Super bypasses all permission checks.
func IsSuper(c *gin.Context) bool {
	if v, ok := c.Get(CtxIsSuper); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// IsPrivileged reports whether the current user sees data across tenants:
// superAdmin or admin role holders. Tenant management also requires this.
func IsPrivileged(c *gin.Context) bool {
	if v, ok := c.Get(CtxIsPrivileged); ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// CurrentTenantID returns the caller's own tenant (0 = platform / none).
func CurrentTenantID(c *gin.Context) uint64 {
	if v, ok := c.Get(TenantIDKey); ok {
		if id, ok := v.(uint64); ok {
			return id
		}
	}
	return 0
}

func SetTenantID(c *gin.Context, id uint64) {
	c.Set(TenantIDKey, id)
}

// TenantScope restricts a query to the caller's tenant. Privileged users
// (superAdmin/admin) are NOT filtered; pass table qualifier e.g. "sys_user".
func TenantScope(c *gin.Context, db *gorm.DB, table string) *gorm.DB {
	if IsPrivileged(c) {
		return db
	}
	return db.Where(table+".tenant_id = ?", CurrentTenantID(c))
}
