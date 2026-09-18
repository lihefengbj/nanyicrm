DROP TABLE IF EXISTS crm_customer_intent_task_item;
DROP TABLE IF EXISTS crm_customer_intent_task;
DROP TABLE IF EXISTS crm_customer_intent_feedback;

ALTER TABLE crm_customer_intent
  DROP COLUMN manual_override,
  DROP COLUMN manual_intent_level;
