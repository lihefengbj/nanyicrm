ALTER TABLE crm_customer_intent_analysis
  ADD COLUMN input_cache_hit_tokens INT NOT NULL DEFAULT 0 AFTER input_tokens,
  ADD COLUMN input_cache_miss_tokens INT NOT NULL DEFAULT 0 AFTER input_cache_hit_tokens;
