-- M2 CRM customer domain: customers, contacts, follow-up records.
-- Mirrors internal/model AutoMigrate; keep both in sync when changing tables.

CREATE TABLE IF NOT EXISTS crm_customer (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  name VARCHAR(128) NOT NULL,
  phone VARCHAR(32) NULL,
  source VARCHAR(32) NULL,
  industry VARCHAR(64) NULL,
  level VARCHAR(8) NULL,
  status TINYINT NOT NULL DEFAULT 1,
  owner_id BIGINT UNSIGNED NULL,
  address VARCHAR(255) NULL,
  remark VARCHAR(255) NULL,
  PRIMARY KEY (id),
  KEY idx_crm_customer_tenant_id (tenant_id),
  KEY idx_crm_customer_name (name),
  KEY idx_crm_customer_owner_id (owner_id),
  KEY idx_crm_customer_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS crm_contact (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  customer_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(64) NOT NULL,
  phone VARCHAR(32) NULL,
  email VARCHAR(128) NULL,
  position VARCHAR(64) NULL,
  is_primary TINYINT NOT NULL DEFAULT 0,
  remark VARCHAR(255) NULL,
  PRIMARY KEY (id),
  KEY idx_crm_contact_tenant_id (tenant_id),
  KEY idx_crm_contact_customer_id (customer_id),
  KEY idx_crm_contact_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS crm_follow_up (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  customer_id BIGINT UNSIGNED NOT NULL,
  contact_id BIGINT UNSIGNED NULL,
  type TINYINT NOT NULL DEFAULT 1,
  content VARCHAR(1024) NOT NULL,
  next_at DATETIME(3) NULL,
  creator_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  creator VARCHAR(64) NULL,
  PRIMARY KEY (id),
  KEY idx_crm_follow_up_tenant_id (tenant_id),
  KEY idx_crm_follow_up_customer_id (customer_id),
  KEY idx_crm_follow_up_contact_id (contact_id),
  KEY idx_crm_follow_up_creator_id (creator_id),
  KEY idx_crm_follow_up_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
