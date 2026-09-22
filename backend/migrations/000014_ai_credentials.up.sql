CREATE TABLE IF NOT EXISTS sys_ai_credential (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  name VARCHAR(64) NOT NULL,
  provider VARCHAR(64) NOT NULL,
  encrypted_api_key TEXT NOT NULL,
  key_last4 VARCHAR(8) NOT NULL,
  status TINYINT NOT NULL DEFAULT 1,
  PRIMARY KEY (id),
  KEY idx_sys_ai_credential_provider (provider),
  KEY idx_sys_ai_credential_status (status),
  KEY idx_sys_ai_credential_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

SET @sql = IF(
  EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_model_config'
      AND column_name = 'credential_id'
  ),
  'SELECT 1',
  'ALTER TABLE sys_ai_model_config ADD COLUMN credential_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER model'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_model_config'
      AND index_name = 'idx_sys_ai_model_config_credential_id'
  ),
  'SELECT 1',
  'CREATE INDEX idx_sys_ai_model_config_credential_id ON sys_ai_model_config (credential_id)'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
