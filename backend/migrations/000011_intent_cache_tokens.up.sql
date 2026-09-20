SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'input_cache_hit_tokens'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN input_cache_hit_tokens INT NOT NULL DEFAULT 0'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'input_cache_miss_tokens'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN input_cache_miss_tokens INT NOT NULL DEFAULT 0'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
