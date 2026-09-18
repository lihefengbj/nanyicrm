ALTER TABLE crm_customer_intent_analysis
  ADD COLUMN actual_model VARCHAR(128) NULL AFTER model,
  ADD COLUMN model_config_version VARCHAR(64) NULL AFTER actual_model,
  ADD COLUMN adapter_version VARCHAR(64) NULL AFTER model_config_version,
  ADD COLUMN input_tokens INT NOT NULL DEFAULT 0 AFTER prompt_version,
  ADD COLUMN output_tokens INT NOT NULL DEFAULT 0 AFTER input_tokens,
  ADD COLUMN total_tokens INT NOT NULL DEFAULT 0 AFTER output_tokens,
  ADD COLUMN provider_request_id VARCHAR(128) NULL AFTER total_tokens,
  ADD COLUMN error_type VARCHAR(32) NULL AFTER provider_request_id,
  ADD COLUMN input_hash VARCHAR(64) NULL AFTER error_type,
  ADD KEY idx_intent_analysis_model_config_version (model_config_version),
  ADD KEY idx_intent_analysis_error_type (error_type),
  ADD KEY idx_intent_analysis_input_hash (input_hash);
