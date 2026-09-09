-- Initial system schema: RBAC + dictionaries + logs.
-- Mirrors internal/model AutoMigrate; keep both in sync when changing tables.

CREATE TABLE IF NOT EXISTS sys_dept (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  parent_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  name VARCHAR(64) NOT NULL,
  leader VARCHAR(64) NULL,
  sort INT NOT NULL DEFAULT 0,
  status TINYINT NOT NULL DEFAULT 1,
  PRIMARY KEY (id),
  KEY idx_sys_dept_parent_id (parent_id),
  KEY idx_sys_dept_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sys_user (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  username VARCHAR(64) NOT NULL,
  pwd_hash VARCHAR(128) NOT NULL,
  nickname VARCHAR(64) NULL,
  email VARCHAR(128) NULL,
  phone VARCHAR(32) NULL,
  dept_id BIGINT UNSIGNED NULL,
  status TINYINT NOT NULL DEFAULT 1,
  remark VARCHAR(255) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY idx_sys_user_username (username),
  KEY idx_sys_user_dept_id (dept_id),
  KEY idx_sys_user_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sys_role (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  name VARCHAR(64) NOT NULL,
  code VARCHAR(64) NOT NULL,
  sort INT NOT NULL DEFAULT 0,
  status TINYINT NOT NULL DEFAULT 1,
  remark VARCHAR(255) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY idx_sys_role_code (code),
  KEY idx_sys_role_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sys_menu (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  parent_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
  title VARCHAR(64) NOT NULL,
  type TINYINT NOT NULL DEFAULT 1,
  path VARCHAR(128) NULL,
  component VARCHAR(128) NULL,
  perms VARCHAR(128) NULL,
  icon VARCHAR(64) NULL,
  sort INT NOT NULL DEFAULT 0,
  visible TINYINT NOT NULL DEFAULT 1,
  status TINYINT NOT NULL DEFAULT 1,
  PRIMARY KEY (id),
  KEY idx_sys_menu_parent_id (parent_id),
  KEY idx_sys_menu_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sys_user_role (
  user_id BIGINT UNSIGNED NOT NULL,
  role_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (user_id, role_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sys_role_menu (
  role_id BIGINT UNSIGNED NOT NULL,
  menu_id BIGINT UNSIGNED NOT NULL,
  PRIMARY KEY (role_id, menu_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sys_dict (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  name VARCHAR(64) NOT NULL,
  type VARCHAR(64) NOT NULL,
  status TINYINT NOT NULL DEFAULT 1,
  remark VARCHAR(255) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY idx_sys_dict_type (type),
  KEY idx_sys_dict_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sys_dict_item (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  created_at DATETIME(3) NULL,
  updated_at DATETIME(3) NULL,
  deleted_at DATETIME(3) NULL,
  dict_id BIGINT UNSIGNED NOT NULL,
  label VARCHAR(64) NOT NULL,
  value VARCHAR(64) NOT NULL,
  sort INT NOT NULL DEFAULT 0,
  status TINYINT NOT NULL DEFAULT 1,
  PRIMARY KEY (id),
  KEY idx_sys_dict_item_dict_id (dict_id),
  KEY idx_sys_dict_item_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sys_oper_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NULL,
  username VARCHAR(64) NULL,
  module VARCHAR(64) NULL,
  action VARCHAR(64) NULL,
  method VARCHAR(16) NULL,
  path VARCHAR(255) NULL,
  ip VARCHAR(64) NULL,
  status INT NULL,
  error_msg VARCHAR(512) NULL,
  cost_millis BIGINT NULL,
  created_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_sys_oper_log_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sys_login_log (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  username VARCHAR(64) NULL,
  ip VARCHAR(64) NULL,
  user_agent VARCHAR(255) NULL,
  success TINYINT(1) NULL,
  message VARCHAR(255) NULL,
  created_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  KEY idx_sys_login_log_username (username)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
