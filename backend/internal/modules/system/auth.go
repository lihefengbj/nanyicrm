package system

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

const refreshTokenKeyPrefix = "auth:refresh:"

type AuthHandler struct {
	db  *gorm.DB
	rdb *redis.Client
	cfg *config.Config
}

func NewAuthHandler(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, rdb: rdb, cfg: cfg}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Pwd      string `json:"pwd" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}

	loginLog := model.SysLoginLog{
		Username:  req.Username,
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}

	var user model.SysUser
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		loginLog.Success, loginLog.Message = false, "user not found"
		h.db.Create(&loginLog)
		common.Fail(c, common.CodeLoginFailed)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PwdHash), []byte(req.Pwd)) != nil {
		loginLog.Success, loginLog.Message = false, "bad credential"
		h.db.Create(&loginLog)
		common.Fail(c, common.CodeLoginFailed)
		return
	}
	if user.Status != 1 {
		loginLog.Success, loginLog.Message = false, "account disabled"
		h.db.Create(&loginLog)
		common.Fail(c, common.CodeAccountDisabled)
		return
	}

	pair, err := common.GenerateTokenPair(h.cfg.JWT.SigningKey, h.cfg.JWT.AccessTokenTTL, h.cfg.JWT.RefreshTokenTTL, user.ID, user.Username)
	if err != nil {
		common.Fail(c, common.CodeInternalError)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	if err := h.rdb.Set(ctx, refreshTokenKeyPrefix+pair.RefreshToken, user.ID, h.cfg.JWT.RefreshTokenTTL).Err(); err != nil {
		common.Fail(c, common.CodeRedisError)
		return
	}

	loginLog.Success, loginLog.Message = true, "login ok"
	h.db.Create(&loginLog)
	common.OK(c, pair)
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	claims, err := common.ParseToken(h.cfg.JWT.SigningKey, req.RefreshToken)
	if err != nil {
		common.Fail(c, common.CodeUnauthorized)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	key := refreshTokenKeyPrefix + req.RefreshToken
	if err := h.rdb.Get(ctx, key).Err(); err != nil {
		// Missing or revoked refresh token.
		common.Fail(c, common.CodeUnauthorized)
		return
	}

	pair, err := common.GenerateTokenPair(h.cfg.JWT.SigningKey, h.cfg.JWT.AccessTokenTTL, h.cfg.JWT.RefreshTokenTTL, claims.UserID, claims.Username)
	if err != nil {
		common.Fail(c, common.CodeInternalError)
		return
	}
	// Rotate: revoke the old refresh token, store the new one.
	h.rdb.Del(ctx, key)
	if err := h.rdb.Set(ctx, refreshTokenKeyPrefix+pair.RefreshToken, claims.UserID, h.cfg.JWT.RefreshTokenTTL).Err(); err != nil {
		common.Fail(c, common.CodeRedisError)
		return
	}
	common.OK(c, pair)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		h.rdb.Del(ctx, refreshTokenKeyPrefix+req.RefreshToken)
	}
	common.OK(c, nil)
}

// Profile returns the current user's info, role codes and permission strings.
func (h *AuthHandler) Profile(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	var user model.SysUser
	if err := h.db.Preload("Dept").Preload("Roles").First(&user, userID).Error; err != nil {
		common.Fail(c, common.CodeUserNotFound)
		return
	}

	roleCodes := make([]string, 0, len(user.Roles))
	roleIDs := make([]uint64, 0, len(user.Roles))
	isAdmin := false
	for _, r := range user.Roles {
		if r.Status != 1 {
			continue
		}
		roleCodes = append(roleCodes, r.Code)
		roleIDs = append(roleIDs, r.ID)
		if r.Code == roleCodeAdmin {
			isAdmin = true
		}
	}

	var perms []string
	if len(roleIDs) > 0 {
		h.db.Model(&model.SysMenu{}).
			Joins("JOIN sys_role_menu rm ON rm.menu_id = sys_menu.id").
			Where("rm.role_id IN ? AND sys_menu.status = 1 AND sys_menu.perms <> ''", roleIDs).
			Distinct().
			Pluck("sys_menu.perms", &perms)
	}

	common.OK(c, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"nickname": user.Nickname,
		"email":    user.Email,
		"phone":    user.Phone,
		"dept":     user.Dept,
		"roles":    roleCodes,
		"perms":    perms,
		"isAdmin":  isAdmin,
	})
}
