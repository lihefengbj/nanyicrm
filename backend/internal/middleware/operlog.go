package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

// OperLog records mutating requests (POST/PUT/DELETE) into sys_oper_log.
func OperLog(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		method := c.Request.Method
		if method == "GET" || method == "OPTIONS" {
			return
		}
		module, action := splitModuleAction(c.FullPath())
		entry := model.SysOperLog{
			UserID:     CurrentUserID(c),
			Username:   CurrentUsername(c),
			Module:     module,
			Action:     action,
			Method:     method,
			Path:       c.FullPath(),
			IP:         c.ClientIP(),
			Status:     c.Writer.Status(),
			CostMillis: time.Since(start).Milliseconds(),
		}
		if errs := c.Errors.String(); errs != "" {
			entry.ErrorMsg = truncate(errs, 500)
		}
		// Best-effort: log write failures must not break the request.
		_ = db.Create(&entry).Error
	}
}

func splitModuleAction(path string) (string, string) {
	// /api/v1/system/user -> module=system, action=user
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 4 {
		return parts[2], parts[3]
	}
	if len(parts) >= 1 {
		return parts[len(parts)-1], ""
	}
	return "", ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
