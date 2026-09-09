package system

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
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

	query := h.db.Model(&model.SysUser{})
	if username != "" {
		query = query.Where("username LIKE ?", "%"+username+"%")
	}
	if status != "" {
		if s, err := strconv.Atoi(status); err == nil {
			query = query.Where("status = ?", s)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.Fail(c, common.CodeDBError)
		return
	}
	var users []model.SysUser
	if err := query.Preload("Dept").Preload("Roles").
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
	Status   int8     `json:"status"`
	Remark   string   `json:"remark" binding:"max=255"`
	RoleIDs  []uint64 `json:"roleIds"`
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

func (h *UserHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}
	var user model.SysUser
	if err := h.db.First(&user, id).Error; err != nil {
		common.Fail(c, common.CodeUserNotFound)
		return
	}

	var req UserSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Fail(c, common.CodeParamInvalid)
		return
	}

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
	var user model.SysUser
	if err := h.db.First(&user, id).Error; err != nil {
		common.Fail(c, common.CodeUserNotFound)
		return
	}
	if user.Username == "admin" {
		common.FailMsg(c, common.CodeParamInvalid, "内置管理员不可删除")
		return
	}
	err = h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", user.ID).Delete(&model.SysUserRole{}).Error; err != nil {
			return err
		}
		// Hard delete: the username unique index would otherwise keep
		// blocking re-creation after a soft delete.
		return tx.Unscoped().Delete(&user).Error
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
