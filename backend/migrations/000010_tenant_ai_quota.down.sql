SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'sys_tenant' AND column_name = 'ai_daily_calls'),
  'ALTER TABLE sys_tenant DROP COLUMN ai_daily_calls',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'sys_tenant' AND column_name = 'ai_daily_tokens'),
  'ALTER TABLE sys_tenant DROP COLUMN ai_daily_tokens',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'sys_tenant' AND column_name = 'ai_concurrency'),
  'ALTER TABLE sys_tenant DROP COLUMN ai_concurrency',
  'SELECT 1'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
