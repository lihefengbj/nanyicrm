SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'crm_customer_intent_analysis' AND column_name = 'billing_period'),
  'SELECT 1',
  'ALTER TABLE crm_customer_intent_analysis ADD COLUMN billing_period VARCHAR(16) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
