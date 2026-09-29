SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_model_config'
      AND column_name = 'approval_status'
  ),
  'SELECT 1',
  'ALTER TABLE sys_ai_model_config ADD COLUMN approval_status VARCHAR(16) NOT NULL DEFAULT ''approved'''
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_model_config'
      AND column_name = 'approved_by'
  ),
  'SELECT 1',
  'ALTER TABLE sys_ai_model_config ADD COLUMN approved_by BIGINT UNSIGNED NOT NULL DEFAULT 0'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_model_config'
      AND column_name = 'approved_at'
  ),
  'SELECT 1',
  'ALTER TABLE sys_ai_model_config ADD COLUMN approved_at DATETIME(3) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_model_config'
      AND column_name = 'approval_note'
  ),
  'SELECT 1',
  'ALTER TABLE sys_ai_model_config ADD COLUMN approval_note VARCHAR(512) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_prompt'
      AND column_name = 'approval_status'
  ),
  'SELECT 1',
  'ALTER TABLE sys_ai_prompt ADD COLUMN approval_status VARCHAR(16) NOT NULL DEFAULT ''approved'''
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_prompt'
      AND column_name = 'approved_by'
  ),
  'SELECT 1',
  'ALTER TABLE sys_ai_prompt ADD COLUMN approved_by BIGINT UNSIGNED NOT NULL DEFAULT 0'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_prompt'
      AND column_name = 'approved_at'
  ),
  'SELECT 1',
  'ALTER TABLE sys_ai_prompt ADD COLUMN approved_at DATETIME(3) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

SET @sql = IF(
  EXISTS(
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
      AND table_name = 'sys_ai_prompt'
      AND column_name = 'approval_note'
  ),
  'SELECT 1',
  'ALTER TABLE sys_ai_prompt ADD COLUMN approval_note VARCHAR(512) NULL'
);
PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

-- Existing active/deployment bootstrap rows remain compatible. Historical
-- non-active candidates must not bypass the new human approval gate.
UPDATE sys_ai_model_config
SET approval_status = CASE WHEN status = 'active' THEN 'approved' ELSE 'pending' END,
    approved_at = CASE WHEN status = 'active' AND approved_at IS NULL THEN CURRENT_TIMESTAMP(3) ELSE approved_at END
;

UPDATE sys_ai_prompt
SET approval_status = CASE WHEN status = 'active' THEN 'approved' ELSE 'pending' END,
    approved_at = CASE WHEN status = 'active' AND approved_at IS NULL THEN CURRENT_TIMESTAMP(3) ELSE approved_at END
;
