ALTER TABLE crm_customer_intent_analysis
  DROP INDEX idx_intent_analysis_dedup_key,
  DROP COLUMN dedup_key;
