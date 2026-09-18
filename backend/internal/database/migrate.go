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
		&model.SysTenant{},
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
		&model.SysApi{},
		&model.CrmCustomer{},
		&model.CrmContact{},
		&model.CrmFollowUp{},
		&model.CrmOpportunity{},
		&model.CrmContract{},
		&model.CrmCustomerIntent{},
		&model.CrmCustomerIntentAnalysis{},
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	if db.Migrator().HasIndex(&model.SysDict{}, "idx_sys_dict_type") {
		if err := db.Migrator().DropIndex(&model.SysDict{}, "idx_sys_dict_type"); err != nil {
			return fmt.Errorf("drop legacy dict index: %w", err)
		}
	}
	if !db.Migrator().HasIndex(&model.SysDict{}, "uk_dict_tenant_type") {
		if err := db.Migrator().CreateIndex(&model.SysDict{}, "uk_dict_tenant_type"); err != nil {
			return fmt.Errorf("create dict tenant index: %w", err)
		}
	}
	if db.Migrator().HasIndex(&model.CrmContract{}, "uk_contract_tenant_code") {
		if err := db.Migrator().DropIndex(&model.CrmContract{}, "uk_contract_tenant_code"); err != nil {
			return fmt.Errorf("drop legacy contract unique index: %w", err)
		}
	}
	if !db.Migrator().HasColumn(&model.CrmContract{}, "active_code") {
		if err := db.Exec(`
			ALTER TABLE crm_contract
			ADD COLUMN active_code VARCHAR(64)
			GENERATED ALWAYS AS (
				CASE
					WHEN deleted_at IS NULL AND code <> '' THEN code
					ELSE NULL
				END
			) STORED
		`).Error; err != nil {
			return fmt.Errorf("add active contract code: %w", err)
		}
	}
	if !db.Migrator().HasIndex(&model.CrmContract{}, "uk_contract_tenant_active_code") {
		if err := db.Exec(`
			CREATE UNIQUE INDEX uk_contract_tenant_active_code
			ON crm_contract (tenant_id, active_code)
		`).Error; err != nil {
			return fmt.Errorf("create active contract code index: %w", err)
		}
	}
	return nil
}
