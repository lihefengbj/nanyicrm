package router

import (
	"context"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/ai"
	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/modules/crm"
	"github.com/lihefengbj/nanyicrm/backend/internal/modules/system"
)

// registrar wires a route and records it in the API registry at the same
// time, keeping sys_api in sync with the code as the single source of truth.
type registrar struct {
	group *gin.RouterGroup
	db    *gorm.DB
	items *[]system.ApiEntry
}

func (r *registrar) handle(method, path, perms string, handlers ...gin.HandlerFunc) {
	*r.items = append(*r.items, system.ApiEntry{Method: method, Path: "/api/v1" + path, Handler: handlerName(handlers[len(handlers)-1]), Perms: perms})
	r.group.Handle(method, path, handlers...)
}

// perm registers a route guarded by a button-level permission string.
func (r *registrar) perm(method, path, perms string, h gin.HandlerFunc) {
	r.handle(method, path, perms, middleware.RequirePerm(r.db, perms), h)
}

// open registers a route that only requires login (no button-level perm).
func (r *registrar) open(method, path string, h gin.HandlerFunc) {
	r.handle(method, path, "", h)
}

// privileged registers a route restricted to superAdmin / admin.
func (r *registrar) privileged(method, path string, h gin.HandlerFunc) {
	r.handle(method, path, "", system.RequirePrivileged(), h)
}

func (r *registrar) privilegedPerm(method, path, perms string, h gin.HandlerFunc) {
	r.handle(method, path, perms, system.RequirePrivileged(), middleware.RequirePerm(r.db, perms), h)
}

func handlerName(h gin.HandlerFunc) string {
	full := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	if i := strings.LastIndex(full, "/"); i >= 0 {
		full = full[i+1:]
	}
	// e.g. system.(*UserHandler).List-fm -> system.UserHandler.List
	full = strings.TrimSuffix(full, "-fm")
	full = strings.ReplaceAll(full, "(", "")
	full = strings.ReplaceAll(full, ")", "")
	full = strings.ReplaceAll(full, "*", "")
	return full
}

// New builds the Gin engine and returns it together with the API registry
// collected during wiring. main.go passes the registry to system.SyncApis
// once the database is ready.
func New(db *gorm.DB, rdb *redis.Client, cfg *config.Config) (*gin.Engine, []system.ApiEntry, *crm.IntentQueue) {
	r := gin.New()
	if err := r.SetTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		panic("invalid trusted proxies: " + err.Error())
	}
	r.Use(gin.Recovery(), middleware.CORS(cfg.Server.AllowedOrigins))

	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		sqlDB, err := db.DB()
		if err != nil || sqlDB.PingContext(ctx) != nil || rdb.Ping(ctx).Err() != nil {
			c.JSON(503, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Swagger UI is available outside production only.
	if cfg.App.Env != "prod" {
		r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	registry := make([]system.ApiEntry, 0, 64)

	v1 := r.Group("/api/v1")
	pub := &registrar{group: v1, db: db, items: &registry}

	auth := system.NewAuthHandler(db, rdb, cfg)
	pub.open("POST", "/auth/login", auth.Login)
	pub.open("POST", "/auth/refresh", auth.Refresh)

	authed := v1.Group("")
	authed.Use(middleware.JWTAuth(db, cfg.JWT.SigningKey), middleware.OperLog(db))
	a := &registrar{group: authed, db: db, items: &registry}

	a.open("POST", "/auth/logout", auth.Logout)
	a.open("GET", "/auth/profile", auth.Profile)

	user := system.NewUserHandler(db)
	a.perm("GET", "/system/user", "system:user:list", user.List)
	a.perm("POST", "/system/user", "system:user:create", user.Create)
	a.perm("PUT", "/system/user/:id", "system:user:update", user.Update)
	a.perm("DELETE", "/system/user/:id", "system:user:delete", user.Delete)

	role := system.NewRoleHandler(db)
	a.privilegedPerm("GET", "/system/role", "system:role:list", role.List)
	a.privilegedPerm("GET", "/system/role/all", "system:role:list", role.All)
	a.privilegedPerm("POST", "/system/role", "system:role:create", role.Create)
	a.privilegedPerm("PUT", "/system/role/:id", "system:role:update", role.Update)
	a.privilegedPerm("DELETE", "/system/role/:id", "system:role:delete", role.Delete)

	menu := system.NewMenuHandler(db)
	a.privilegedPerm("GET", "/system/menu/tree", "system:menu:list", menu.Tree)
	a.privilegedPerm("POST", "/system/menu", "system:menu:create", menu.Create)
	a.privilegedPerm("PUT", "/system/menu/:id", "system:menu:update", menu.Update)
	a.privilegedPerm("DELETE", "/system/menu/:id", "system:menu:delete", menu.Delete)

	dept := system.NewDeptHandler(db)
	a.perm("GET", "/system/dept/tree", "system:dept:list", dept.Tree)
	a.perm("POST", "/system/dept", "system:dept:create", dept.Create)
	a.perm("PUT", "/system/dept/:id", "system:dept:update", dept.Update)
	a.perm("DELETE", "/system/dept/:id", "system:dept:delete", dept.Delete)

	tenant := system.NewTenantHandler(db)
	a.privileged("GET", "/system/tenant", tenant.List)
	a.privileged("GET", "/system/tenant/all", tenant.All)
	a.privileged("POST", "/system/tenant", tenant.Create)
	a.privileged("PUT", "/system/tenant/:id", tenant.Update)
	a.privileged("DELETE", "/system/tenant/:id", tenant.Delete)

	logs := system.NewLogHandler(db)
	a.perm("GET", "/system/log/oper", "system:log:oper", logs.OperList)
	a.perm("GET", "/system/log/login", "system:log:login", logs.LoginList)

	dict := system.NewDictHandler(db)
	a.perm("GET", "/system/dict", "system:dict:list", dict.List)
	a.perm("POST", "/system/dict", "system:dict:create", dict.Create)
	a.perm("PUT", "/system/dict/:id", "system:dict:update", dict.Update)
	a.perm("DELETE", "/system/dict/:id", "system:dict:delete", dict.Delete)
	a.perm("POST", "/system/dict/item", "system:dict:update", dict.CreateItem)
	a.perm("PUT", "/system/dict/item/:id", "system:dict:update", dict.UpdateItem)
	a.perm("DELETE", "/system/dict/item/:id", "system:dict:update", dict.DeleteItem)
	// Dropdown source for any authenticated user; no button perm needed.
	a.open("GET", "/system/dict/items/:type", dict.Items)

	api := system.NewApiHandler(db, &registry)
	a.privilegedPerm("GET", "/system/api", "system:api:list", api.List)
	a.privilegedPerm("GET", "/system/api/all", "system:api:list", api.All)
	a.privilegedPerm("PUT", "/system/api/:id", "system:api:update", api.UpdateTitle)
	a.privilegedPerm("POST", "/system/api/sync", "system:api:update", api.Sync)

	customer := crm.NewCustomerHandler(db)
	a.perm("GET", "/crm/customer", "crm:customer:list", customer.List)
	a.perm("GET", "/crm/customer/all", "crm:customer:list", customer.All)
	a.perm("POST", "/crm/customer", "crm:customer:create", customer.Create)

	contact := crm.NewContactHandler(db)
	a.perm("GET", "/crm/contact", "crm:contact:list", contact.List)
	a.perm("POST", "/crm/contact", "crm:contact:create", contact.Create)
	a.perm("PUT", "/crm/contact/:id", "crm:contact:update", contact.Update)
	a.perm("DELETE", "/crm/contact/:id", "crm:contact:delete", contact.Delete)

	var intentProvider ai.Provider
	if cfg.LLM.Enabled {
		intentProvider = ai.NewProvider(cfg.LLM)
	}
	intent := crm.NewIntentHandler(db, cfg.LLM.Enabled, intentProvider)
	var intentQueue *crm.IntentQueue
	var enqueueIntent crm.IntentEnqueuer
	if cfg.LLM.Enabled {
		intentQueue = crm.NewIntentQueue(rdb, intent)
		enqueueIntent = intentQueue.Enqueue
		intent.SetTaskEnqueuer(intentQueue.EnqueueTask)
	}

	follow := crm.NewFollowUpHandler(db, enqueueIntent)
	a.perm("GET", "/crm/follow", "crm:follow:list", follow.List)
	a.perm("POST", "/crm/follow", "crm:follow:create", follow.Create)
	a.perm("PUT", "/crm/follow/:id", "crm:follow:update", follow.Update)
	a.perm("DELETE", "/crm/follow/:id", "crm:follow:delete", follow.Delete)

	opp := crm.NewOpportunityHandler(db)
	a.perm("GET", "/crm/opportunity", "crm:opportunity:list", opp.List)
	a.perm("GET", "/crm/opportunity/all", "crm:opportunity:list", opp.All)
	a.perm("POST", "/crm/opportunity", "crm:opportunity:create", opp.Create)
	a.perm("PUT", "/crm/opportunity/:id", "crm:opportunity:update", opp.Update)
	a.perm("DELETE", "/crm/opportunity/:id", "crm:opportunity:delete", opp.Delete)

	contract := crm.NewContractHandler(db)
	a.perm("GET", "/crm/contract", "crm:contract:list", contract.List)
	a.perm("POST", "/crm/contract", "crm:contract:create", contract.Create)
	a.perm("PUT", "/crm/contract/:id", "crm:contract:update", contract.Update)
	a.perm("DELETE", "/crm/contract/:id", "crm:contract:delete", contract.Delete)

	a.perm("POST", "/crm/customer/intent/batch", "crm:intent:batch", intent.Batch)
	a.perm("GET", "/crm/customer/intent/tasks/:id", "crm:intent:batch", intent.Task)
	a.perm("POST", "/crm/customer/intent/tasks/:id/cancel", "crm:intent:batch", intent.CancelTask)
	a.perm("GET", "/crm/customer/:id/intent", "crm:intent:list", intent.Current)
	a.perm("GET", "/crm/customer/:id/intent/history", "crm:intent:history", intent.History)
	a.perm("GET", "/crm/customer/:id/intent/compare", "crm:intent:list", intent.Compare)
	a.perm("POST", "/crm/customer/:id/intent/analyze", "crm:intent:analyze", intent.Analyze)
	a.perm("POST", "/crm/customer/:id/intent/retry", "crm:intent:analyze", intent.Retry)
	a.handle("GET", "/crm/customer/:id/intent/feedback", "crm:intent:feedback|crm:intent:feedback:list",
		middleware.RequireAnyPerm(db, "crm:intent:feedback", "crm:intent:feedback:list"),
		intent.FeedbackHistory)
	a.perm("POST", "/crm/customer/:id/intent/feedback", "crm:intent:feedback", intent.Feedback)
	a.perm("GET", "/crm/intent/workbench", "crm:intent:workbench", intent.Workbench)
	a.perm("GET", "/crm/intent/metrics", "crm:intent:metrics", intent.Metrics)
	a.privilegedPerm("POST", "/crm/intent/config/test", "crm:intent:config", intent.ConfigTest)
	a.perm("PUT", "/crm/customer/:id", "crm:customer:update", customer.Update)
	a.perm("DELETE", "/crm/customer/:id", "crm:customer:delete", customer.Delete)

	dashboard := crm.NewDashboardHandler(db)
	a.open("GET", "/dashboard/summary", dashboard.Summary)

	return r, registry, intentQueue
}
