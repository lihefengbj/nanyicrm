SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'crm_customer_intent_analysis'
      AND column_name = 'dedup_key'
  ),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN dedup_key VARCHAR(64) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

UPDATE crm_customer_intent_analysis
SET dedup_key = SHA2(
  CONCAT(
    COALESCE(input_hash, ''),
    CHAR(0),
    COALESCE(prompt_version, ''),
    CHAR(0),
    COALESCE(model_config_version, '')
  ),
  256
)
WHERE dedup_key IS NULL;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
      AND table_name = 'crm_customer_intent_analysis'
      AND index_name = 'idx_intent_analysis_dedup_key'
  ),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD KEY idx_intent_analysis_dedup_key (dedup_key)'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
