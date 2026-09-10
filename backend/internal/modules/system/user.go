package system

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

type UserHandler struct {
	db *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

func (h *UserHandler) List(c *gin.Context) {
	page := common.ParsePageQuery(c)
	username := strings.TrimSpace(c.Query("username"))
	status := strings.TrimSpace(c.Query("status"))

	query := middleware.TenantScope(c, h.db.Model(&model.SysUser{}), "sys_user")
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if status != "" {
		if s, err := strconv.Atoi(status); err == nil {
			query = query.Where("status = ?", s)
		}
	}
	// The super admin may narrow the list to one tenant explicitly.
	if middleware.IsPrivileged(c) {
		if tid, err := strconv.ParseUint(c.Query("tenantId"), 10, 64); err == nil && tid > 0 {
			query = query.Where("sys_user.tenant_id = ?", tid)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var users []model.SysUser
	if err := query.Preload("Dept").Preload("Tenant").Preload("Roles").
		Order("sys_user.id DESC").
		Offset(page.Offset()).Limit(page.PageSize).
		Find(&users).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OKPage(c, users, total, page.PageNum, page.PageSize)
}

type UserSaveRequest struct {
	Username string   `json:"username" binding:"required,min=2,max=64"`
	Pwd      string   `json:"pwd"` // required on create, optional on update
	Nickname string   `json:"nickname" binding:"max=64"`
	Email    string   `json:"email" binding:"omitempty,email"`
	Phone    string   `json:"phone" binding:"max=32"`
	DeptID   *uint64  `json:"deptId"`
	TenantID uint64   `json:"tenantId"` // super admin only; ignored for tenant users
	Status   int8     `json:"status"`
	Remark   string   `json:"remark" binding:"max=255"`
	RoleIDs  []uint64 `json:"roleIds"`
}

// resolveTenant decides which tenant a saved user belongs to and validates
// that the referenced roles/dept live in the same tenant.
func (h *UserHandler) resolveTenant(c *gin.Context, req *UserSaveRequest, userID uint64) (uint64, bool) {
	tenantID := middleware.CurrentTenantID(c)
	if middleware.IsPrivileged(c) {
		tenantID = req.TenantID
		if tenantID == 0 {
			// tenant_id = 0 stays valid only for existing platform users.
			if userID == 0 {
				common.FailMsg(c, common.CodeParamInvalid, "请为用户选择所属租户")
				return 0, false
			}
			var cur model.SysUser
			if err := h.db.Select("tenant_id").First(&cur, userID).Error; err != nil || cur.TenantID != 0 {
				common.FailMsg(c, common.CodeParamInvalid, "请为用户选择所属租户")
				return 0, false
			}
			return 0, true
		}
		var exists int64
		h.db.Model(&model.SysTenant{}).Where("id = ?", tenantID).Count(&exists)
		if exists == 0 {
			common.Fail(c, common.CodeTenantNotFound)
			return 0, false
		}
	}

	if req.DeptID != nil {
		var dept model.SysDept
		if err := h.db.First(&dept, *req.DeptID).Error; err != nil || dept.TenantID != tenantID {
			common.FailMsg(c, common.CodeParamInvalid, "部门不属于该租户")
			return 0, false
		}
	}
	return tenantID, true
}

func (h *UserHandler) Create(c *gin.Context) {
	var req UserSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	if len(req.Pwd) < 6 {
		common.FailMsg(c, common.CodeParamInvalid, "密码长度至少 6 位")
		return
	}
	tenantID, ok := h.resolveTenant(c, &req, 0)
	if !ok {
		return
	}

	var exists int64
	h.db.Model(&model.SysUser{}).Where("username = ?", req.Username).Count(&exists)
	if exists > 0 {
		common.Fail(c, common.CodeUserExists)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Pwd), bcrypt.DefaultCost)
	if err != nil {
		common.Fail(c, common.CodeInternalError)
		return
	}
	user := model.SysUser{
		TenantID: tenantID,
		Username: req.Username,
		PwdHash:  string(hash),
		Nickname: req.Nickname,
		Email:    req.Email,
		Phone:    req.Phone,
		DeptID:   req.DeptID,
		Status:   req.Status,
		Remark:   req.Remark,
	}
	if user.Status == 0 {
		user.Status = 1
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		return replaceUserRoles(tx, user.ID, req.RoleIDs)
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, gin.H{"id": user.ID})
}

// findInTenant loads a user scoped to the caller's tenant (super = any).
func (h *UserHandler) findInTenant(c *gin.Context, id uint64) (*model.SysUser, bool) {
	var user model.SysUser
	query := h.db
	if !middleware.IsPrivileged(c) {
		query = query.Where("tenant_id = ?", middleware.CurrentTenantID(c))
	}
	if err := query.First(&user, id).Error; err != nil {
		common.Fail(c, common.CodeUserNotFound)
		return nil, false
	}
	return &user, true
}

func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	user, ok := h.findInTenant(c, id)
	if !ok {
		return
	}

	var req UserSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	// Privileged users may re-assign the tenant; tenant users keep their own.
	if !middleware.IsPrivileged(c) {
		req.TenantID = user.TenantID
	}
	tenantID, ok := h.resolveTenant(c, &req, user.ID)
	if !ok {
		return
	}
	user.TenantID = tenantID

	user.Nickname = req.Nickname
	user.Email = req.Email
	user.Phone = req.Phone
	user.DeptID = req.DeptID
	user.Remark = req.Remark
	if req.Status == 0 || req.Status == 1 {
		user.Status = req.Status
	}
	if req.Pwd != "" {
		if len(req.Pwd) < 6 {
			common.FailMsg(c, common.CodeParamInvalid, "密码长度至少 6 位")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Pwd), bcrypt.DefaultCost)
		if err != nil {
			common.Fail(c, common.CodeInternalError)
			return
		}
		user.PwdHash = string(hash)
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&user).Error; err != nil {
			return err
		}
		return replaceUserRoles(tx, user.ID, req.RoleIDs)
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

func (h *UserHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	user, ok := h.findInTenant(c, id)
	if !ok {
		return
	}
	if (user.Username == "admin" || user.Username == "superAdmin") && user.TenantID == 0 {
		common.FailMsg(c, common.CodeParamInvalid, "内置账号不可删除")
		return
	}
	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", user.ID).Delete(&model.SysUserRole{}).Error; err != nil {
			return err
		}
		// Hard delete: the username unique index would otherwise keep
		// blocking re-creation after a soft delete.
		return tx.Unscoped().Delete(user).Error
	})
	if err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	common.OK(c, nil)
}

func replaceUserRoles(tx *gorm.DB, userID uint64, roleIDs []uint64) error {
	if err := tx.Where("user_id = ?", userID).Delete(&model.SysUserRole{}).Error; err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}
	links := make([]model.SysUserRole, 0, len(roleIDs))
	for _, rid := range roleIDs {
		links = append(links, model.SysUserRole{UserID: userID, RoleID: rid})
	}
	return tx.Create(&links).Error
}
