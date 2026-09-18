package database

import (
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
)

func TestTuneMySQLDSNAddsConnectionSafetyDefaults(t *testing.T) {
	got, err := tuneMySQLDSN("user:password@tcp(127.0.0.1:3306)/nanyicrm?charset=utf8mb4&parseTime=True&loc=Local")
	if err != nil {
		t.Fatalf("tuneMySQLDSN() error = %v", err)
	}

	parsed, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("parse tuned DSN: %v", err)
	}
	if parsed.Timeout != 5*time.Second {
		t.Fatalf("Timeout = %v, want 5s", parsed.Timeout)
	}
	if parsed.ReadTimeout != 10*time.Second {
		t.Fatalf("ReadTimeout = %v, want 10s", parsed.ReadTimeout)
	}
	if parsed.WriteTimeout != 10*time.Second {
		t.Fatalf("WriteTimeout = %v, want 10s", parsed.WriteTimeout)
	}
	if !parsed.CheckConnLiveness {
		t.Fatal("CheckConnLiveness = false, want true")
	}
}

func TestTuneMySQLDSNPreservesExplicitTimeouts(t *testing.T) {
	got, err := tuneMySQLDSN("user:password@tcp(127.0.0.1:3306)/nanyicrm?timeout=3s&readTimeout=4s&writeTimeout=5s&checkConnLiveness=false")
	if err != nil {
		t.Fatalf("tuneMySQLDSN() error = %v", err)
	}

	parsed, err := mysql.ParseDSN(got)
	if err != nil {
		t.Fatalf("parse tuned DSN: %v", err)
	}
	if parsed.Timeout != 3*time.Second || parsed.ReadTimeout != 4*time.Second || parsed.WriteTimeout != 5*time.Second {
		t.Fatalf("explicit timeouts were changed: timeout=%v read=%v write=%v",
			parsed.Timeout, parsed.ReadTimeout, parsed.WriteTimeout)
	}
	if !parsed.CheckConnLiveness {
		t.Fatal("CheckConnLiveness = false, want true")
	}
}
