ALTER TABLE crm_customer_intent_analysis
  ADD COLUMN billing_period VARCHAR(16) NULL AFTER total_tokens;
