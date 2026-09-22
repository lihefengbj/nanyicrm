CREATE TABLE IF NOT EXISTS sys_ai_prompt (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  name VARCHAR(64) NOT NULL,
  content TEXT NOT NULL,
  content_hash VARCHAR(64) NOT NULL,
  version VARCHAR(32) NOT NULL,
  status VARCHAR(16) NOT NULL,
  canary_percent INT NOT NULL DEFAULT 0,
  quality_passed TINYINT(1) NOT NULL DEFAULT 0,
  quality_summary TEXT NULL,
  quality_metrics TEXT NULL,
  quality_checked_at DATETIME(3) NULL,
  activated_at DATETIME(3) NULL,
  retired_at DATETIME(3) NULL,
  created_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  updated_by BIGINT UNSIGNED NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uk_sys_ai_prompt_content_hash (content_hash),
  UNIQUE KEY uk_sys_ai_prompt_version (version),
  KEY idx_sys_ai_prompt_status (status),
  KEY idx_sys_ai_prompt_created_at (created_at),
  KEY idx_sys_ai_prompt_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sys_ai_prompt_change (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  prompt_id BIGINT UNSIGNED NOT NULL,
  from_prompt_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  to_prompt_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  actor_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  action VARCHAR(32) NOT NULL,
  reason VARCHAR(512) NULL,
  from_version VARCHAR(32) NULL,
  to_version VARCHAR(32) NULL,
  quality_summary TEXT NULL,
  created_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_sys_ai_prompt_change_prompt_id (prompt_id),
  KEY idx_sys_ai_prompt_change_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Keep existing deployments on the hard-coded v1 policy functional while
-- moving the policy into the governed store.
INSERT INTO sys_ai_prompt
  (name, content, content_hash, version, status, quality_passed, quality_summary, activated_at, created_at, updated_at)
SELECT
  '意向研判口径 v1',
  '请按客户意向等级定义进行判断：high 表示已有明确采购计划、预算或近期决策动作；medium 表示有真实业务兴趣但采购条件尚未明确；low 表示互动弱或暂无明确需求；unknown 表示输入信息不足。只根据客户上下文判断，抽取明确需求、痛点、预算、采购时间、决策角色、风险和下一步行动；所有未知信息填写“未明确”。',
  SHA2('请按客户意向等级定义进行判断：high 表示已有明确采购计划、预算或近期决策动作；medium 表示有真实业务兴趣但采购条件尚未明确；low 表示互动弱或暂无明确需求；unknown 表示输入信息不足。只根据客户上下文判断，抽取明确需求、痛点、预算、采购时间、决策角色、风险和下一步行动；所有未知信息填写“未明确”。', 256),
  'v1',
  'active',
  1,
  '系统迁移初始化；沿用原有代码提示词口径',
  NOW(3), NOW(3), NOW(3)
FROM DUAL
WHERE NOT EXISTS (SELECT 1 FROM sys_ai_prompt WHERE version = 'v1');

