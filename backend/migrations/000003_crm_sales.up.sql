-- M3 sales domain: opportunities and contracts.
-- Mirrors internal/model AutoMigrate; keep both in sync when changing tables.

CREATE TABLE IF NOT EXISTS crm_opportunity (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  customer_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(128) NOT NULL,
  stage TINYINT NOT NULL DEFAULT 1,
  amount DECIMAL(12,2) NOT NULL DEFAULT 0,
  expect_date DATETIME(3) NULL,
  owner_id BIGINT UNSIGNED NULL,
  remark VARCHAR(255) NULL,
  PRIMARY KEY (id),
  KEY idx_crm_opportunity_tenant_id (tenant_id),
  KEY idx_crm_opportunity_customer_id (customer_id),
  KEY idx_crm_opportunity_stage (stage),
  KEY idx_crm_opportunity_owner_id (owner_id),
  KEY idx_crm_opportunity_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS crm_contract (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  code VARCHAR(64) NULL,
  name VARCHAR(128) NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  opportunity_id BIGINT UNSIGNED NULL,
  amount DECIMAL(12,2) NOT NULL DEFAULT 0,
  sign_date DATETIME(3) NULL,
  start_date DATETIME(3) NULL,
  end_date DATETIME(3) NULL,
  status TINYINT NOT NULL DEFAULT 1,
  owner_id BIGINT UNSIGNED NULL,
  remark VARCHAR(255) NULL,
  PRIMARY KEY (id),
  KEY idx_crm_contract_tenant_id (tenant_id),
  KEY idx_crm_contract_code (code),
  KEY idx_crm_contract_customer_id (customer_id),
  KEY idx_crm_contract_opportunity_id (opportunity_id),
  KEY idx_crm_contract_owner_id (owner_id),
  KEY idx_crm_contract_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
