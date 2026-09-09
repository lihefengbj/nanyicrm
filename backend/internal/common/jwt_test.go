package common

import (
	"testing"
	"time"
)

func TestGenerateAndParseToken(t *testing.T) {
	key := "test-signing-key"
	pair, err := GenerateTokenPair(key, time.Hour, 24*time.Hour, 42, "tester")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("tokens must not be empty")
	}
	if pair.ExpiresIn != 3600 {
		t.Fatalf("expiresIn = %d, want 3600", pair.ExpiresIn)
	}

	claims, err := ParseToken(key, pair.AccessToken)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.UserID != 42 || claims.Username != "tester" {
		t.Fatalf("claims = %+v", claims)
	}
}

func TestParseTokenWrongKey(t *testing.T) {
	pair, err := GenerateTokenPair("key-a", time.Hour, time.Hour, 1, "u")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := ParseToken("key-b", pair.AccessToken); err != ErrTokenInvalid {
		t.Fatalf("err = %v, want ErrTokenInvalid", err)
	}
}

func TestParseTokenExpired(t *testing.T) {
	key := "test-signing-key"
	pair, err := GenerateTokenPair(key, -time.Hour, time.Hour, 1, "u")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := ParseToken(key, pair.AccessToken); err != ErrTokenExpired {
		t.Fatalf("err = %v, want ErrTokenExpired", err)
	}
}
