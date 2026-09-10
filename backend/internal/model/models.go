package model

import (
	"time"

	"gorm.io/gorm"
)

// Base holds columns shared by every business table.
type Base struct {
	ID        uint64         `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// SysTenant is a SaaS tenant. tenant_id = 0 on other tables means
// "platform-level" data owned by the super admin (who has no tenant).
type SysTenant struct {
	Base
	Code     string     `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Name     string     `gorm:"size:64;not null" json:"name"`
	Contact  string     `gorm:"size:64" json:"contact"`
	Phone    string     `gorm:"size:32" json:"phone"`
	ExpireAt *time.Time `json:"expireAt"`                // nil = never expires
	Status   int8       `gorm:"default:1" json:"status"` // 1 enabled, 0 disabled
	Remark   string     `gorm:"size:255" json:"remark"`
}

func (SysTenant) TableName() string { return "sys_tenant" }

// Usable reports whether tenant users may log in right now.
func (t *SysTenant) Usable() bool {
	if t.Status != 1 {
		return false
	}
	if t.ExpireAt != nil && t.ExpireAt.Before(time.Now()) {
		return false
	}
	return true
}

type SysUser struct {
	Base
	TenantID uint64     `gorm:"index;default:0" json:"tenantId"` // 0 = platform super admin
	Username string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PwdHash  string     `gorm:"size:128;not null" json:"-"`
	Nickname string     `gorm:"size:64" json:"nickname"`
	Email    string     `gorm:"size:128" json:"email"`
	Phone    string     `gorm:"size:32" json:"phone"`
	DeptID   *uint64    `gorm:"index" json:"deptId"`
	Status   int8       `gorm:"default:1" json:"status"` // 1 enabled, 0 disabled
	Remark   string     `gorm:"size:255" json:"remark"`
	Dept     *SysDept   `gorm:"foreignKey:DeptID" json:"dept,omitempty"`
	Tenant   *SysTenant `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Roles    []SysRole  `gorm:"many2many:sys_user_role;joinForeignKey:UserID;joinReferences:RoleID" json:"roles,omitempty"`
}

func (SysUser) TableName() string { return "sys_user" }

type SysDept struct {
	Base
	TenantID uint64    `gorm:"index;default:0" json:"tenantId"`
	ParentID uint64    `gorm:"index;default:0" json:"parentId"`
	Name     string    `gorm:"size:64;not null" json:"name"`
	Leader   string    `gorm:"size:64" json:"leader"`
	Sort     int       `gorm:"default:0" json:"sort"`
	Status   int8      `gorm:"default:1" json:"status"`
	Children []SysDept `gorm:"-" json:"children,omitempty"`
}

func (SysDept) TableName() string { return "sys_dept" }

// SysRole is global: all tenants share one set of roles, only the platform
// side manages them. Code is globally unique.
type SysRole struct {
	Base
	Name   string    `gorm:"size:64;not null" json:"name"`
	Code   string    `gorm:"size:64;uniqueIndex;not null" json:"code"`
	Sort   int       `gorm:"default:0" json:"sort"`
	Status int8      `gorm:"default:1" json:"status"`
	Remark string    `gorm:"size:255" json:"remark"`
	Menus  []SysMenu `gorm:"many2many:sys_role_menu;joinForeignKey:RoleID;joinReferences:MenuID" json:"menus,omitempty"`
}

func (SysRole) TableName() string { return "sys_role" }

// SysMenu is a menu, a directory, or a button-level permission.
type SysMenu struct {
	Base
	ParentID  uint64    `gorm:"index;default:0" json:"parentId"`
	Title     string    `gorm:"size:64;not null" json:"title"`
	Type      int8      `gorm:"default:1" json:"type"` // 1 dir, 2 menu, 3 button
	Path      string    `gorm:"size:128" json:"path"`
	Component string    `gorm:"size:128" json:"component"`
	Perms     string    `gorm:"size:128" json:"perms"` // e.g. system:user:list
	Icon      string    `gorm:"size:64" json:"icon"`
	Sort      int       `gorm:"default:0" json:"sort"`
	Visible   int8      `gorm:"default:1" json:"visible"`
	Status    int8      `gorm:"default:1" json:"status"`
	Children  []SysMenu `gorm:"-" json:"children,omitempty"`
}

func (SysMenu) TableName() string { return "sys_menu" }

type SysUserRole struct {
	UserID uint64 `gorm:"primaryKey"`
	RoleID uint64 `gorm:"primaryKey"`
}

func (SysUserRole) TableName() string { return "sys_user_role" }

type SysRoleMenu struct {
	RoleID uint64 `gorm:"primaryKey"`
	MenuID uint64 `gorm:"primaryKey"`
}

func (SysRoleMenu) TableName() string { return "sys_role_menu" }

type SysDict struct {
	Base
	TenantID uint64        `gorm:"uniqueIndex:uk_dict_tenant_type;index;default:0" json:"tenantId"`
	Name     string        `gorm:"size:64;not null" json:"name"`
	Type     string        `gorm:"size:64;uniqueIndex:uk_dict_tenant_type;not null" json:"type"`
	Status   int8          `gorm:"default:1" json:"status"`
	Remark   string        `gorm:"size:255" json:"remark"`
	Items    []SysDictItem `gorm:"foreignKey:DictID" json:"items,omitempty"`
}

func (SysDict) TableName() string { return "sys_dict" }

type SysDictItem struct {
	Base
	DictID uint64 `gorm:"index;not null" json:"dictId"`
	Label  string `gorm:"size:64;not null" json:"label"`
	Value  string `gorm:"size:64;not null" json:"value"`
	Sort   int    `gorm:"default:0" json:"sort"`
	Status int8   `gorm:"default:1" json:"status"`
}

func (SysDictItem) TableName() string { return "sys_dict_item" }

type SysOperLog struct {
	ID         uint64    `gorm:"primaryKey" json:"id"`
	TenantID   uint64    `gorm:"index;default:0" json:"tenantId"`
	UserID     uint64    `gorm:"index" json:"userId"`
	Username   string    `gorm:"size:64" json:"username"`
	Module     string    `gorm:"size:64" json:"module"`
	Action     string    `gorm:"size:64" json:"action"`
	Method     string    `gorm:"size:16" json:"method"`
	Path       string    `gorm:"size:255" json:"path"`
	IP         string    `gorm:"size:64" json:"ip"`
	Status     int       `json:"status"` // business code from response
	ErrorMsg   string    `gorm:"size:512" json:"errorMsg"`
	CostMillis int64     `json:"costMillis"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (SysOperLog) TableName() string { return "sys_oper_log" }

type SysLoginLog struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	TenantID  uint64    `gorm:"index;default:0" json:"tenantId"`
	Username  string    `gorm:"size:64;index" json:"username"`
	IP        string    `gorm:"size:64" json:"ip"`
	UserAgent string    `gorm:"size:255" json:"userAgent"`
	Success   bool      `json:"success"`
	Message   string    `gorm:"size:255" json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

func (SysLoginLog) TableName() string { return "sys_login_log" }
