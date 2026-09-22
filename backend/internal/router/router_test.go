package router

import (
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
)

func TestNewRegistersCustomerIntentRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		Server: config.ServerConfig{TrustedProxies: []string{"127.0.0.1"}},
	}
	engine, registry, _, _ := New(nil, nil, cfg)
	if engine == nil {
		t.Fatal("expected gin engine")
	}
	paths := map[string]bool{}
	for _, item := range registry {
		paths[item.Method+" "+item.Path] = true
	}
	for _, route := range []string{
		"POST /api/v1/crm/customer/intent/batch",
		"GET /api/v1/crm/customer/intent/tasks/:id",
		"POST /api/v1/crm/customer/intent/tasks/:id/cancel",
		"GET /api/v1/crm/customer/:id/intent/feedback",
		"POST /api/v1/crm/customer/:id/intent/feedback",
		"GET /api/v1/crm/intent/workbench",
		"GET /api/v1/crm/intent/metrics",
		"POST /api/v1/crm/intent/config/test",
		"GET /api/v1/crm/intent/model-credential",
		"POST /api/v1/crm/intent/model-credential",
		"PUT /api/v1/crm/intent/model-credential/:id/rotate",
		"GET /api/v1/crm/intent/model-config",
		"PUT /api/v1/crm/intent/model-config/:id/credential",
		"POST /api/v1/crm/intent/model-config/:id/quality-gate",
		"POST /api/v1/crm/intent/model-config/:id/canary",
		"POST /api/v1/crm/intent/model-config/:id/activate",
		"POST /api/v1/crm/intent/model-config/rollback",
		"GET /api/v1/crm/intent/model-config/changes",
	} {
		if !paths[route] {
			t.Fatalf("route %s was not registered", route)
		}
	}
}
