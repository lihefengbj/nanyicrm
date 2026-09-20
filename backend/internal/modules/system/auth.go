package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
	"github.com/lihefengbj/nanyicrm/backend/internal/observability"
)

const refreshTokenKeyPrefix = "auth:refresh:"
const loginAttemptKeyPrefix = "auth:login-attempt:"
const maxLoginAttempts = 10
const accessCookieName = "nanyicrm_access"
const refreshCookieName = "nanyicrm_refresh"

var rotateRefreshTokenScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
	return 0
end
redis.call("DEL", KEYS[1])
redis.call("SET", KEYS[2], ARGV[1], "PX", ARGV[2])
return 1
`)

func refreshTokenKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return refreshTokenKeyPrefix + hex.EncodeToString(sum[:])
}

func loginAttemptKey(ip, username string) string {
	sum := sha256.Sum256([]byte(ip + "\x00" + username))
	return loginAttemptKeyPrefix + hex.EncodeToString(sum[:])
}

func (h *AuthHandler) recordLoginFailure(ctx context.Context, key string) {
	count, err := h.rdb.Incr(ctx, key).Result()
	if err == nil && count == 1 {
		h.rdb.Expire(ctx, key, 10*time.Minute)
	}
}

type AuthHandler struct {
	db      *gorm.DB
	rdb     *redis.Client
	cfg     *config.Config
	alerts  *observability.AlertManager
	metrics *observability.Metrics
}

func NewAuthHandler(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *AuthHandler {
	return &AuthHandler{db: db, rdb: rdb, cfg: cfg}
}

func (h *AuthHandler) SetObservability(metrics *observability.Metrics, alerts *observability.AlertManager) {
	h.metrics = metrics
	h.alerts = alerts
}

func (h *AuthHandler) markLoginFailure(ctx context.Context, key, username, ip, reason string) {
	h.recordLoginFailure(ctx, key)
	if h.alerts != nil {
		h.alerts.LoginFailure(ctx, username, ip, reason)
	} else if h.metrics != nil {
		h.metrics.IncLoginFailure()
	}
}

func (h *AuthHandler) secureCookies() bool {
	return h.cfg != nil && h.cfg.App.Env == "prod"
}

func (h *AuthHandler) setSessionCookies(c *gin.Context, pair *common.TokenPair) {
	if pair == nil {
		return
	}
	secure := h.secureCookies()
	writeSessionCookie(c, accessCookieName, pair.AccessToken, int(h.cfg.JWT.AccessTokenTTL.Seconds()), "/api/v1", secure)
	writeSessionCookie(c, refreshCookieName, pair.RefreshToken, int(h.cfg.JWT.RefreshTokenTTL.Seconds()), "/api/v1/auth", secure)
}

func clearSessionCookies(c *gin.Context, secure bool) {
	writeSessionCookie(c, accessCookieName, "", -1, "/api/v1", secure)
	writeSessionCookie(c, refreshCookieName, "", -1, "/api/v1/auth", secure)
}

func writeSessionCookie(c *gin.Context, name, value string, maxAge int, path string, secure bool) {
	c.Writer.Header().Add("Set-Cookie", (&http.Cookie{
		Name: name, Value: value, MaxAge: maxAge, Path: path,
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
	}).String())
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Pwd      string `json:"pwd" binding:"required"`
}

// Login 账号密码登录
// @Summary  登录，颁发 access/refresh token
// @Tags     认证
// @Param    body  body  LoginRequest  true  "登录请求"
// @Success  200  {object}  common.TokenPair
// @Router   /auth/login [post]
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	attemptKey := loginAttemptKey(loginLog.IP, req.Username)
	if count, err := h.rdb.Get(ctx, attemptKey).Int(); err == nil && count >= maxLoginAttempts {
		loginLog.Success, loginLog.Message = false, "too many attempts"
		h.db.Create(&loginLog)
		if h.alerts != nil {
			h.alerts.LoginFailure(ctx, req.Username, loginLog.IP, "too many attempts")
		} else if h.metrics != nil {
			h.metrics.IncLoginFailure()
		}
		common.FailMsg(c, common.CodeLoginFailed, "登录尝试过多，请稍后再试")
		return
	}

	var user model.SysUser
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		h.markLoginFailure(ctx, attemptKey, req.Username, loginLog.IP, "user not found")
		loginLog.Success, loginLog.Message = false, "user not found"
		h.db.Create(&loginLog)
		common.Fail(c, common.CodeLoginFailed)
		return
	}
	loginLog.TenantID = user.TenantID
	if bcrypt.CompareHashAndPassword([]byte(user.PwdHash), []byte(req.Pwd)) != nil {
		h.markLoginFailure(ctx, attemptKey, req.Username, loginLog.IP, "bad credential")
		loginLog.Success, loginLog.Message = false, "bad credential"
		h.db.Create(&loginLog)
		common.Fail(c, common.CodeLoginFailed)
		return
	}
	if user.Status != 1 {
		h.markLoginFailure(ctx, attemptKey, req.Username, loginLog.IP, "account disabled")
		loginLog.Success, loginLog.Message = false, "account disabled"
		h.db.Create(&loginLog)
		common.Fail(c, common.CodeAccountDisabled)
		return
	}
	// Tenant gate: disabled or expired tenants cannot log in at all.
	if user.TenantID != 0 {
		var tenant model.SysTenant
		if err := h.db.First(&tenant, user.TenantID).Error; err != nil {
			h.markLoginFailure(ctx, attemptKey, req.Username, loginLog.IP, "tenant missing")
			loginLog.Success, loginLog.Message = false, "tenant missing"
			h.db.Create(&loginLog)
			common.Fail(c, common.CodeTenantNotFound)
			return
		}
		if tenant.Status != 1 {
			h.markLoginFailure(ctx, attemptKey, req.Username, loginLog.IP, "tenant disabled")
			loginLog.Success, loginLog.Message = false, "tenant disabled"
			h.db.Create(&loginLog)
			common.Fail(c, common.CodeTenantDisabled)
			return
		}
		if tenant.ExpireAt != nil && tenant.ExpireAt.Before(time.Now()) {
			h.markLoginFailure(ctx, attemptKey, req.Username, loginLog.IP, "tenant expired")
			loginLog.Success, loginLog.Message = false, "tenant expired"
			h.db.Create(&loginLog)
			common.Fail(c, common.CodeTenantExpired)
			return
		}
	}

	pair, err := common.GenerateTokenPair(h.cfg.JWT.SigningKey, h.cfg.JWT.AccessTokenTTL, h.cfg.JWT.RefreshTokenTTL, user.ID, user.Username)
	if err != nil {
		common.Fail(c, common.CodeInternalError)
		return
	}

	if err := h.rdb.Set(ctx, refreshTokenKey(pair.RefreshToken), user.ID, h.cfg.JWT.RefreshTokenTTL).Err(); err != nil {
		common.Fail(c, common.CodeRedisError)
		return
	}

	loginLog.Success, loginLog.Message = true, "login ok"
	h.db.Create(&loginLog)
	h.rdb.Del(ctx, attemptKey)
	if h.metrics != nil {
		h.metrics.IncLoginSuccess()
	}
	h.setSessionCookies(c, pair)
	common.OK(c, gin.H{"authenticated": true, "expiresIn": pair.ExpiresIn})
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// Refresh 刷新令牌
// @Summary  用 refresh token 轮换新的 token 对
// @Tags     认证
// @Param    body  body  RefreshRequest  true  "刷新请求"
// @Success  200  {object}  common.TokenPair
// @Router   /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Empty bodies are valid when the HttpOnly refresh cookie is used.
		req = RefreshRequest{}
	}
	if req.RefreshToken == "" {
		req.RefreshToken, _ = c.Cookie(refreshCookieName)
	}
	if req.RefreshToken == "" {
		common.Fail(c, common.CodeUnauthorized)
		return
	}
	claims, err := common.ParseRefreshToken(h.cfg.JWT.SigningKey, req.RefreshToken)
	if err != nil {
		common.Fail(c, common.CodeUnauthorized)
		return
	}

	var user model.SysUser
	if err := h.db.Select("id", "username", "tenant_id", "status").First(&user, claims.UserID).Error; err != nil {
		common.Fail(c, common.CodeUnauthorized)
		return
	}
	if user.Status != 1 {
		common.Fail(c, common.CodeAccountDisabled)
		return
	}
	if user.TenantID != 0 {
		var tenant model.SysTenant
		if err := h.db.Select("id", "status", "expire_at").First(&tenant, user.TenantID).Error; err != nil {
			common.Fail(c, common.CodeUnauthorized)
			return
		}
		if !tenant.Usable() {
			common.Fail(c, common.CodeUnauthorized)
			return
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	pair, err := common.GenerateTokenPair(h.cfg.JWT.SigningKey, h.cfg.JWT.AccessTokenTTL, h.cfg.JWT.RefreshTokenTTL, user.ID, user.Username)
	if err != nil {
		common.Fail(c, common.CodeInternalError)
		return
	}
	oldKey := refreshTokenKey(req.RefreshToken)
	newKey := refreshTokenKey(pair.RefreshToken)
	result, err := rotateRefreshTokenScript.Run(
		ctx,
		h.rdb,
		[]string{oldKey, newKey},
		strconv.FormatUint(claims.UserID, 10),
		h.cfg.JWT.RefreshTokenTTL.Milliseconds(),
	).Int()
	if err != nil {
		common.Fail(c, common.CodeRedisError)
		return
	}
	if result != 1 {
		common.Fail(c, common.CodeUnauthorized)
		return
	}
	h.setSessionCookies(c, pair)
	common.OK(c, gin.H{"authenticated": true, "expiresIn": pair.ExpiresIn})
}

// Logout 登出
// @Summary  吊销 refresh token
// @Tags     认证
// @Param    body  body  RefreshRequest  false  "要吊销的 refresh token"
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err == nil && req.RefreshToken != "" {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		h.rdb.Del(ctx, refreshTokenKey(req.RefreshToken))
	}
	if req.RefreshToken == "" {
		if cookie, err := c.Cookie(refreshCookieName); err == nil && cookie != "" {
			ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
			defer cancel()
			h.rdb.Del(ctx, refreshTokenKey(cookie))
		}
	}
	clearSessionCookies(c, h.secureCookies())
	common.OK(c, nil)
}

// Profile returns the current user's info, role codes and permission strings.
// @Summary  当前用户信息（含角色与权限标识）
// @Tags     认证
// @Success  200  {object}  map[string]interface{}
// @Security BearerAuth
// @Router   /auth/profile [get]
func (h *AuthHandler) Profile(c *gin.Context) {
	userID := middleware.CurrentUserID(c)
	var user model.SysUser
	if err := h.db.Preload("Dept").Preload("Roles").First(&user, userID).Error; err != nil {
		common.Fail(c, common.CodeUserNotFound)
		return
	}

	roleCodes := make([]string, 0, len(user.Roles))
	roleIDs := make([]uint64, 0, len(user.Roles))
	isSuper := false
	isPrivileged := false
	for _, r := range user.Roles {
		if r.Status != 1 {
			continue
		}
		roleCodes = append(roleCodes, r.Code)
		roleIDs = append(roleIDs, r.ID)
		if r.Code == middleware.RoleCodeSuperAdmin {
			isSuper = true
		}
		if r.Code == middleware.RoleCodeAdmin {
			isPrivileged = true
		}
	}
	if user.Username == middleware.UsernameSuperAdmin {
		isSuper = true
	}
	isPrivileged = isPrivileged || isSuper

	perms := []string{}
	if len(roleIDs) > 0 {
		h.db.Model(&model.SysMenu{}).
			Joins("JOIN sys_role_menu rm ON rm.menu_id = sys_menu.id").
			Where("rm.role_id IN ? AND sys_menu.status = 1 AND sys_menu.perms <> ''", roleIDs).
			Distinct().
			Pluck("sys_menu.perms", &perms)
	}

	// Visible menu tree (dirs + menus, no buttons) for the dynamic sidebar
	// and router. Super admins see every enabled menu; others see only what
	// their roles grant. Hidden (visible=0) menus are included so the
	// frontend can still register their routes.
	var menus []model.SysMenu
	if isSuper {
		h.db.Where("status = 1 AND type IN (1, 2)").Order("sort, id").Find(&menus)
	} else if len(roleIDs) > 0 {
		h.db.Model(&model.SysMenu{}).
			Joins("JOIN sys_role_menu rm ON rm.menu_id = sys_menu.id").
			Where("rm.role_id IN ? AND sys_menu.status = 1 AND sys_menu.type IN (1, 2)", roleIDs).
			Distinct().
			Order("sys_menu.sort, sys_menu.id").
			Find(&menus)
	}

	var tenantInfo gin.H
	if user.TenantID != 0 {
		var tenant model.SysTenant
		if err := h.db.First(&tenant, user.TenantID).Error; err == nil {
			tenantInfo = gin.H{"id": tenant.ID, "code": tenant.Code, "name": tenant.Name}
		}
	}

	common.OK(c, gin.H{
		"id":           user.ID,
		"tenantId":     user.TenantID,
		"tenant":       tenantInfo,
		"isSuper":      isSuper,
		"isPrivileged": isPrivileged,
		"username":     user.Username,
		"nickname":     user.Nickname,
		"email":        user.Email,
		"phone":        user.Phone,
		"dept":         user.Dept,
		"roles":        roleCodes,
		"perms":        perms,
		"menus":        buildMenuTree(menus, 0),
		"isAdmin":      isSuper, // kept for backward compatibility
	})
}
