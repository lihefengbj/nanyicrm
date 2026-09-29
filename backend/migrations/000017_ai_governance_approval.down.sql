ALTER TABLE sys_ai_model_config
  DROP COLUMN approval_note,
  DROP COLUMN approved_at,
  DROP COLUMN approved_by,
  DROP COLUMN approval_status;

ALTER TABLE sys_ai_prompt
  DROP COLUMN approval_note,
  DROP COLUMN approved_at,
  DROP COLUMN approved_by,
  DROP COLUMN approval_status;
