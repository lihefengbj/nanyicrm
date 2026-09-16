ALTER TABLE crm_contract
  DROP INDEX uk_contract_tenant_active_code,
  DROP COLUMN active_code,
  ADD KEY idx_crm_contract_code (code);

ALTER TABLE sys_login_log
  DROP INDEX idx_sys_login_log_tenant_id,
  DROP COLUMN tenant_id;

ALTER TABLE sys_oper_log
  DROP INDEX idx_sys_oper_log_tenant_id,
  DROP COLUMN tenant_id;

ALTER TABLE sys_dict
  DROP INDEX uk_dict_tenant_type,
  DROP INDEX idx_sys_dict_tenant_id,
  DROP COLUMN tenant_id,
  ADD UNIQUE KEY idx_sys_dict_type (type);

ALTER TABLE sys_dept
  DROP INDEX idx_sys_dept_tenant_id,
  DROP COLUMN tenant_id;

ALTER TABLE sys_user
  DROP INDEX idx_sys_user_tenant_id,
  DROP COLUMN tenant_id;

DROP TABLE IF EXISTS sys_tenant;
