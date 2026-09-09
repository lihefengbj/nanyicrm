package database

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/model"
)

// AutoMigrate keeps the schema in sync for development. SQL migration files
// under backend/migrations mirror these definitions for golang-migrate in
// production deployments.
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.SysUser{},
		&model.SysDept{},
		&model.SysRole{},
		&model.SysMenu{},
		&model.SysUserRole{},
		&model.SysRoleMenu{},
		&model.SysDict{},
		&model.SysDictItem{},
		&model.SysOperLog{},
		&model.SysLoginLog{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	return nil
}
