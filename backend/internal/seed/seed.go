package seed

import (
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

// Run inserts the built-in admin account, admin role, base menus and a root
// department when they do not exist yet. Safe to call on every boot.
func Run(db *gorm.DB) error {
	if err := seedDept(db); err != nil {
		return err
	}
	if err := seedMenus(db); err != nil {
		return err
	}
	if err := seedAdminRole(db); err != nil {
		return err
	}
	if err := seedAdminUser(db); err != nil {
		return err
	}
	return nil
}

func seedDept(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.SysDept{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	dept := model.SysDept{Name: "南壹科技", Leader: "admin", Sort: 0, Status: 1}
	return db.Create(&dept).Error
}

func seedMenus(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.SysMenu{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	system := model.SysMenu{Title: "系统管理", Type: 1, Path: "/system", Icon: "setting", Sort: 100, Visible: 1, Status: 1}
	if err := db.Create(&system).Error; err != nil {
		return err
	}
	menus := []model.SysMenu{
		{ParentID: system.ID, Title: "用户管理", Type: 2, Path: "user", Component: "system/user/index", Perms: "system:user:list", Sort: 1, Visible: 1, Status: 1},
		{ParentID: system.ID, Title: "角色管理", Type: 2, Path: "role", Component: "system/role/index", Perms: "system:role:list", Sort: 2, Visible: 1, Status: 1},
		{ParentID: system.ID, Title: "部门管理", Type: 2, Path: "dept", Component: "system/dept/index", Perms: "system:dept:list", Sort: 3, Visible: 1, Status: 1},
		{ParentID: system.ID, Title: "菜单管理", Type: 2, Path: "menu", Component: "system/menu/index", Perms: "system:menu:list", Sort: 4, Visible: 1, Status: 1},
		{ParentID: system.ID, Title: "操作日志", Type: 2, Path: "operlog", Component: "system/log/oper", Perms: "system:log:oper", Sort: 5, Visible: 1, Status: 1},
		{ParentID: system.ID, Title: "登录日志", Type: 2, Path: "loginlog", Component: "system/log/login", Perms: "system:log:login", Sort: 6, Visible: 1, Status: 1},
	}
	for i := range menus {
		if err := db.Create(&menus[i]).Error; err != nil {
			return err
		}
	}

	// Button-level permissions under each page.
	buttons := []model.SysMenu{}
	for _, m := range menus {
		base := m.Perms
		prefix := base[:len(base)-len(":list")]
		if m.Perms == "system:log:oper" || m.Perms == "system:log:login" {
			continue // logs are read-only pages
		}
		buttons = append(buttons,
			model.SysMenu{ParentID: m.ID, Title: "新增", Type: 3, Perms: prefix + ":create", Sort: 1, Visible: 1, Status: 1},
			model.SysMenu{ParentID: m.ID, Title: "编辑", Type: 3, Perms: prefix + ":update", Sort: 2, Visible: 1, Status: 1},
			model.SysMenu{ParentID: m.ID, Title: "删除", Type: 3, Perms: prefix + ":delete", Sort: 3, Visible: 1, Status: 1},
		)
	}
	for i := range buttons {
		if err := db.Create(&buttons[i]).Error; err != nil {
			return err
		}
	}
	log.Println("seed: base menus created")
	return nil
}

func seedAdminRole(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.SysRole{}).Where("code = ?", "admin").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	role := model.SysRole{Name: "超级管理员", Code: "admin", Sort: 0, Status: 1, Remark: "内置角色，拥有全部权限"}
	if err := db.Create(&role).Error; err != nil {
		return err
	}
	// Grant every menu to the admin role.
	var menus []model.SysMenu
	if err := db.Find(&menus).Error; err != nil {
		return err
	}
	links := make([]model.SysRoleMenu, 0, len(menus))
	for _, m := range menus {
		links = append(links, model.SysRoleMenu{RoleID: role.ID, MenuID: m.ID})
	}
	if len(links) > 0 {
		return db.Create(&links).Error
	}
	return nil
}

func seedAdminUser(db *gorm.DB) error {
	var count int64
	if err := db.Model(&model.SysUser{}).Where("username = ?", "admin").Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	var dept model.SysDept
	if err := db.First(&dept).Error; err != nil {
		return err
	}
	user := model.SysUser{
		Username: "admin",
		PwdHash:  string(hash),
		Nickname: "系统管理员",
		DeptID:   &dept.ID,
		Status:   1,
		Remark:   "内置管理员，默认密码 admin123，请登录后立即修改",
	}
	if err := db.Create(&user).Error; err != nil {
		return err
	}
	var role model.SysRole
	if err := db.Where("code = ?", "admin").First(&role).Error; err != nil {
		return err
	}
	if err := db.Create(&model.SysUserRole{UserID: user.ID, RoleID: role.ID}).Error; err != nil {
		return err
	}
	log.Println("seed: admin user created (admin / admin123)")
	return nil
}
