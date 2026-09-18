ALTER TABLE crm_customer_intent
  ADD COLUMN analysis_id BIGINT UNSIGNED NOT NULL DEFAULT 0 AFTER customer_id,
  ADD KEY idx_customer_intent_analysis_id (analysis_id);

UPDATE crm_customer_intent AS ci
SET ci.analysis_id = COALESCE((
  SELECT MAX(ia.id)
  FROM crm_customer_intent_analysis AS ia
  WHERE ia.tenant_id = ci.tenant_id
    AND ia.customer_id = ci.customer_id
    AND ia.status = 'success'
    AND ia.analyzed_at = ci.analyzed_at
    AND ia.deleted_at IS NULL
), 0)
WHERE ci.analysis_id = 0;
