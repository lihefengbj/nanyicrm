-- Bring the versioned schema in line with the multi-tenant GORM models.
-- Conditional statements keep this migration compatible with databases that
-- were previously updated by GORM AutoMigrate.

CREATE TABLE IF NOT EXISTS sys_tenant (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  code VARCHAR(64) NOT NULL,
  name VARCHAR(64) NOT NULL,
  contact VARCHAR(64) NULL,
  phone VARCHAR(32) NULL,
  expire_at DATETIME(3) NULL,
  status TINYINT NOT NULL DEFAULT 1,
  remark VARCHAR(255) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY idx_sys_tenant_code (code),
  KEY idx_sys_tenant_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'sys_user' AND column_name = 'tenant_id'),
  'SELECT 1',
  'ALTER TABLE sys_user ADD COLUMN tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER deleted_at, ADD KEY idx_sys_user_tenant_id (tenant_id)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'sys_dept' AND column_name = 'tenant_id'),
  'SELECT 1',
  'ALTER TABLE sys_dept ADD COLUMN tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER deleted_at, ADD KEY idx_sys_dept_tenant_id (tenant_id)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'sys_dict' AND column_name = 'tenant_id'),
  'SELECT 1',
  'ALTER TABLE sys_dict ADD COLUMN tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER deleted_at, ADD KEY idx_sys_dict_tenant_id (tenant_id)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'sys_dict' AND index_name = 'idx_sys_dict_type'),
  'ALTER TABLE sys_dict DROP INDEX idx_sys_dict_type',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'sys_dict' AND index_name = 'uk_dict_tenant_type'),
  'SELECT 1',
  'ALTER TABLE sys_dict ADD UNIQUE KEY uk_dict_tenant_type (tenant_id, type)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'sys_oper_log' AND column_name = 'tenant_id'),
  'SELECT 1',
  'ALTER TABLE sys_oper_log ADD COLUMN tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER id, ADD KEY idx_sys_oper_log_tenant_id (tenant_id)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'sys_login_log' AND column_name = 'tenant_id'),
  'SELECT 1',
  'ALTER TABLE sys_login_log ADD COLUMN tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER id, ADD KEY idx_sys_login_log_tenant_id (tenant_id)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'crm_contract' AND index_name = 'idx_crm_contract_code'),
  'ALTER TABLE crm_contract DROP INDEX idx_crm_contract_code',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'crm_contract' AND index_name = 'uk_contract_tenant_code'),
  'ALTER TABLE crm_contract DROP INDEX uk_contract_tenant_code',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_contract' AND column_name = 'active_code'),
  'SELECT 1',
  'ALTER TABLE crm_contract ADD COLUMN active_code VARCHAR(64) GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL AND code <> '''' THEN code ELSE NULL END) STORED'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'crm_contract' AND index_name = 'uk_contract_tenant_active_code'),
  'SELECT 1',
  'ALTER TABLE crm_contract ADD UNIQUE KEY uk_contract_tenant_active_code (tenant_id, active_code)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
