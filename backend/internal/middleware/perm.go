package middleware

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

// RequirePerm checks that the current user holds the given permission string
// (e.g. "system:user:list") through any of their enabled roles. The super
// admin bypasses the check entirely.
func RequirePerm(db *gorm.DB, perm string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := CurrentUserID(c)
		if userID == 0 {
			common.Abort(c, common.CodeUnauthorized)
			return
		}
		// Super admin bypasses everything, even with zero roles assigned
		// (hardcoded username fallback is folded into IsSuper).
		if IsSuper(c) {
			c.Next()
			return
		}

		var roles []model.SysRole
		if err := db.Joins("JOIN sys_user_role ur ON ur.role_id = sys_role.id").
			Where("ur.user_id = ? AND sys_role.status = 1", userID).
			Find(&roles).Error; err != nil {
			common.Abort(c, common.CodeDBError)
			return
		}
		if len(roles) == 0 {
			common.Abort(c, common.CodeForbidden)
			return
		}
		roleIDs := make([]uint64, 0, len(roles))
		for _, r := range roles {
			roleIDs = append(roleIDs, r.ID)
		}

		var count int64
		if err := db.Model(&model.SysMenu{}).
			Joins("JOIN sys_role_menu rm ON rm.menu_id = sys_menu.id").
			Where("rm.role_id IN ? AND sys_menu.perms = ? AND sys_menu.status = 1", roleIDs, perm).
			Count(&count).Error; err != nil {
			common.Abort(c, common.CodeDBError)
			return
		}
		if count == 0 {
			common.Abort(c, common.CodeForbidden)
			return
		}
		c.Next()
	}
}
