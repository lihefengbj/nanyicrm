ALTER TABLE crm_customer_intent
  DROP INDEX idx_customer_intent_analysis_id,
  DROP COLUMN analysis_id;
