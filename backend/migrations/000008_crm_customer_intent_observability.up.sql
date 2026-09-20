SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'actual_model'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN actual_model VARCHAR(128) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'model_config_version'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN model_config_version VARCHAR(64) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'adapter_version'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN adapter_version VARCHAR(64) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'input_tokens'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN input_tokens INT NOT NULL DEFAULT 0'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'output_tokens'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN output_tokens INT NOT NULL DEFAULT 0'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'total_tokens'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN total_tokens INT NOT NULL DEFAULT 0'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'provider_request_id'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN provider_request_id VARCHAR(128) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'error_type'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN error_type VARCHAR(32) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'input_hash'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN input_hash VARCHAR(64) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND index_name = 'idx_intent_analysis_model_config_version'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD KEY idx_intent_analysis_model_config_version (model_config_version)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND index_name = 'idx_intent_analysis_error_type'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD KEY idx_intent_analysis_error_type (error_type)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND index_name = 'idx_intent_analysis_input_hash'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD KEY idx_intent_analysis_input_hash (input_hash)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
