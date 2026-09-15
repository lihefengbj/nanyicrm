-- API registry: endpoints synced from the Gin route table at boot.
-- Mirrors internal/model AutoMigrate; keep both in sync when changing tables.

CREATE TABLE IF NOT EXISTS sys_api (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  method VARCHAR(8) NOT NULL,
  path VARCHAR(255) NOT NULL,
  handler VARCHAR(128) NULL,
  title VARCHAR(64) NULL,
  module VARCHAR(32) NULL,
  perms VARCHAR(128) NULL,
  status TINYINT NOT NULL DEFAULT 1,
  PRIMARY KEY (id),
  UNIQUE KEY uk_api_method_path (method, path),
  KEY idx_sys_api_module (module),
  KEY idx_sys_api_deleted_at (deleted_at)
);
