package system

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/lihefengbj/nanyicrm/backend/internal/common"
	"github.com/lihefengbj/nanyicrm/backend/internal/config"
)

func TestSessionCookiesAreHttpOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &AuthHandler{cfg: &config.Config{
		App: config.AppConfig{Env: "prod"},
		JWT: config.JWTConfig{AccessTokenTTL: time.Hour, RefreshTokenTTL: 24 * time.Hour},
	}}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	handler.setSessionCookies(ctx, &common.TokenPair{
		AccessToken:  "access",
		RefreshToken: "refresh",
	})
	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookies = %d, want 2", len(cookies))
	}
	for _, cookie := range cookies {
		if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite == 0 {
			t.Fatalf("cookie %q missing HttpOnly/Secure: %+v", cookie.Name, cookie)
		}
	}
}

func TestSessionCookiesCanBeCleared(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	clearSessionCookies(ctx, true)
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.MaxAge >= 0 {
			t.Fatalf("cookie %q was not expired: %+v", cookie.Name, cookie)
		}
	}
}
