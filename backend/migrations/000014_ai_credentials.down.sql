SET @sql = IF(
  EXISTS(
    SELECT 1 FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_model_config'
      AND index_name = 'idx_sys_ai_model_config_credential_id'
  ),
  'ALTER TABLE sys_ai_model_config DROP INDEX idx_sys_ai_model_config_credential_id',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1 FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_model_config'
      AND column_name = 'credential_id'
  ),
  'ALTER TABLE sys_ai_model_config DROP COLUMN credential_id',
  'SELECT 1'
);
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

DROP TABLE IF EXISTS sys_ai_credential;
