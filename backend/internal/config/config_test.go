package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExpandsEnvPlaceholders(t *testing.T) {
	t.Setenv("TEST_MYSQL_PWD", "s3cret")
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "server:\n  port: \"19090\"\nmysql:\n  - name: default\n    dsn: \"root:${TEST_MYSQL_PWD}@tcp(localhost:3306)/nanyicrm\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)

	cfg := Load()
	if cfg.Server.Port != "19090" {
		t.Fatalf("port = %q, want 19090", cfg.Server.Port)
	}
	dsn := cfg.DefaultMySQL().DSN
	if want := "root:s3cret@tcp(localhost:3306)/nanyicrm"; dsn[:len(want)] != want {
		t.Fatalf("dsn = %q, want prefix %q", dsn, want)
	}
}

func TestLoadFallsBackToDefaults(t *testing.T) {
	t.Setenv("CONFIG_PATH", filepath.Join(t.TempDir(), "missing.yaml"))
	cfg := Load()
	if cfg.Server.Port != "8080" {
		t.Fatalf("port = %q, want default 8080", cfg.Server.Port)
	}
	if len(cfg.MySQL) != 1 {
		t.Fatalf("mysql entries = %d, want 1 default", len(cfg.MySQL))
	}
}

func TestLoadKeepsZeroLogRetention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("log:\n  retain_days: 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	if got := Load().Log.RetainDays; got != 0 {
		t.Fatalf("retain days = %d, want 0", got)
	}
}

func TestProductionRequiresStrongSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "app:\n  env: prod\njwt:\n  signing_key: short\nbootstrap:\n  admin_password: short\n  super_admin_password: short\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_PATH", path)
	defer func() {
		if recover() == nil {
			t.Fatal("Load did not reject weak production secrets")
		}
	}()
	Load()
}
