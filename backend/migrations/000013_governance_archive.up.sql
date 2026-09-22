CREATE TABLE IF NOT EXISTS sys_ai_model_config (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  name VARCHAR(64) NOT NULL,
  provider VARCHAR(64) NOT NULL,
  base_url VARCHAR(255) NOT NULL,
  model VARCHAR(128) NOT NULL,
  credential_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  config_version VARCHAR(64) NOT NULL,
  prompt_version VARCHAR(32) NOT NULL,
  response_format VARCHAR(32) NULL,
  thinking_mode VARCHAR(16) NULL,
  max_tokens INT NOT NULL DEFAULT 1200,
  temperature FLOAT NOT NULL DEFAULT 0.2,
  status VARCHAR(16) NOT NULL,
  canary_percent INT NOT NULL DEFAULT 0,
  quality_passed TINYINT(1) NOT NULL DEFAULT 0,
  quality_summary TEXT NULL,
  quality_metrics TEXT NULL,
  quality_checked_at DATETIME(3) NULL,
  activated_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_sys_ai_model_config_version (config_version),
  KEY idx_sys_ai_model_config_status (status),
  KEY idx_sys_ai_model_config_credential_id (credential_id),
  KEY idx_sys_ai_model_config_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sys_ai_model_change (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  config_id BIGINT UNSIGNED NOT NULL,
  from_config_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  to_config_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  actor_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  action VARCHAR(32) NOT NULL,
  reason VARCHAR(512) NULL,
  quality_summary TEXT NULL,
  created_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_sys_ai_model_change_config_id (config_id),
  KEY idx_sys_ai_model_change_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS crm_customer_intent_analysis_archive (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  original_analysis_id BIGINT UNSIGNED NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL,
  customer_id BIGINT UNSIGNED NOT NULL,
  payload LONGTEXT NOT NULL,
  created_at DATETIME(3) NULL,
  archived_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_intent_analysis_archive_original (original_analysis_id),
  KEY idx_intent_analysis_archive_tenant_customer (tenant_id, customer_id),
  KEY idx_intent_analysis_archive_archived_at (archived_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
