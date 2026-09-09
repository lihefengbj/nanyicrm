package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
)

const (
	CtxUserID   = "currentUserID"
	CtxUsername = "currentUsername"
)

// JWTAuth validates the Bearer access token and stores identity in context.
func JWTAuth(signingKey string) gin.HandlerFunc {
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
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		c.Next()
	}
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
