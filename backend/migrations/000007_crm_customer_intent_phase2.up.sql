-- Phase two: human feedback, batch analysis tasks and task items.

ALTER TABLE crm_customer_intent
  ADD COLUMN manual_override BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN manual_intent_level VARCHAR(16) NULL;

CREATE TABLE IF NOT EXISTS crm_customer_intent_feedback (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  customer_id BIGINT UNSIGNED NOT NULL,
  analysis_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  feedback_type VARCHAR(24) NOT NULL,
  accepted BOOLEAN NULL,
  manual_intent_level VARCHAR(16) NULL,
  note VARCHAR(1024) NULL,
  PRIMARY KEY (id),
  KEY idx_intent_feedback_tenant_id (tenant_id),
  KEY idx_intent_feedback_customer_id (customer_id),
  KEY idx_intent_feedback_analysis_id (analysis_id),
  KEY idx_intent_feedback_user_id (user_id),
  KEY idx_intent_feedback_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS crm_customer_intent_task (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_by BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL,
  total_count INT NOT NULL DEFAULT 0,
  pending_count INT NOT NULL DEFAULT 0,
  running_count INT NOT NULL DEFAULT 0,
  success_count INT NOT NULL DEFAULT 0,
  failed_count INT NOT NULL DEFAULT 0,
  canceled_count INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 3,
  error_message VARCHAR(1024) NULL,
  started_at DATETIME(3) NULL,
  finished_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_intent_task_tenant_id (tenant_id),
  KEY idx_intent_task_status (status),
  KEY idx_intent_task_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS crm_customer_intent_task_item (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  task_id BIGINT UNSIGNED NOT NULL,
  tenant_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  customer_id BIGINT UNSIGNED NOT NULL,
  trigger_user_id BIGINT UNSIGNED NOT NULL,
  status VARCHAR(16) NOT NULL,
  attempts INT NOT NULL DEFAULT 0,
  error_message VARCHAR(1024) NULL,
  started_at DATETIME(3) NULL,
  finished_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uk_intent_task_customer (task_id, tenant_id, customer_id),
  KEY idx_intent_task_item_tenant_id (tenant_id),
  KEY idx_intent_task_item_status (status),
  KEY idx_intent_task_item_customer_id (customer_id),
  KEY idx_intent_task_item_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
