package security

import "testing"

func TestEncryptDecryptSecret(t *testing.T) {
	encoded, err := EncryptSecret("master-key", "sk-test-secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encoded == "sk-test-secret" {
		t.Fatal("secret was stored in plaintext")
	}
	decoded, err := DecryptSecret("master-key", encoded)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decoded != "sk-test-secret" {
		t.Fatalf("decoded secret = %q", decoded)
	}
	if _, err := DecryptSecret("wrong-key", encoded); err == nil {
		t.Fatal("decrypt with wrong key unexpectedly succeeded")
	}
}

func TestKeyLast4(t *testing.T) {
	if got := KeyLast4("abcd1234"); got != "1234" {
		t.Fatalf("KeyLast4 = %q", got)
	}
	if got := KeyLast4("abc"); got != "abc" {
		t.Fatalf("short KeyLast4 = %q", got)
	}
}
