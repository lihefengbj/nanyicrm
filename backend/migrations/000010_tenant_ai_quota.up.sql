-- Tenant-level AI intent quota overrides on sys_tenant.
-- NULL inherits the platform default (llm.quota in config.yaml); 0 means "no limit".
-- Conditional statements keep this migration compatible with databases that
-- were previously updated by GORM AutoMigrate.

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'sys_tenant' AND column_name = 'ai_daily_calls'),
  'SELECT 1',
  'ALTER TABLE sys_tenant ADD COLUMN ai_daily_calls BIGINT NULL COMMENT ''每日AI意向分析次数上限，NULL 继承平台默认，0 不限制'' AFTER remark'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'sys_tenant' AND column_name = 'ai_daily_tokens'),
  'SELECT 1',
  'ALTER TABLE sys_tenant ADD COLUMN ai_daily_tokens BIGINT NULL COMMENT ''单日AI分析Token用量上限，NULL 继承平台默认，0 不限制'' AFTER ai_daily_calls'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'sys_tenant' AND column_name = 'ai_concurrency'),
  'SELECT 1',
  'ALTER TABLE sys_tenant ADD COLUMN ai_concurrency INT NULL COMMENT ''AI意向分析并发上限，NULL 继承平台默认，0 不限制'' AFTER ai_daily_tokens'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
