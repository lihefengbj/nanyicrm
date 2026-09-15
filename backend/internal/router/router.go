package router

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
	"github.com/lihefengbj/nanyicrm/backend/internal/modules/crm"
	"github.com/lihefengbj/nanyicrm/backend/internal/modules/system"
)

func New(db *gorm.DB, rdb *redis.Client, cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery(), middleware.CORS())

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	auth := system.NewAuthHandler(db, rdb, cfg)
	v1.POST("/auth/login", auth.Login)
	v1.POST("/auth/refresh", auth.Refresh)

	authed := v1.Group("")
	authed.Use(middleware.JWTAuth(db, cfg.JWT.SigningKey), middleware.OperLog(db))
	{
		authed.POST("/auth/logout", auth.Logout)
		authed.GET("/auth/profile", auth.Profile)

		user := system.NewUserHandler(db)
		authed.GET("/system/user", middleware.RequirePerm(db, "system:user:list"), user.List)
		authed.POST("/system/user", middleware.RequirePerm(db, "system:user:create"), user.Create)
		authed.PUT("/system/user/:id", middleware.RequirePerm(db, "system:user:update"), user.Update)
		authed.DELETE("/system/user/:id", middleware.RequirePerm(db, "system:user:delete"), user.Delete)

		role := system.NewRoleHandler(db)
		authed.GET("/system/role", middleware.RequirePerm(db, "system:role:list"), role.List)
		authed.GET("/system/role/all", middleware.RequirePerm(db, "system:role:list"), role.All)
		authed.POST("/system/role", middleware.RequirePerm(db, "system:role:create"), role.Create)
		authed.PUT("/system/role/:id", middleware.RequirePerm(db, "system:role:update"), role.Update)
		authed.DELETE("/system/role/:id", middleware.RequirePerm(db, "system:role:delete"), role.Delete)

		menu := system.NewMenuHandler(db)
		authed.GET("/system/menu/tree", middleware.RequirePerm(db, "system:menu:list"), menu.Tree)

		dept := system.NewDeptHandler(db)
		authed.GET("/system/dept/tree", middleware.RequirePerm(db, "system:dept:list"), dept.Tree)
		authed.POST("/system/dept", middleware.RequirePerm(db, "system:dept:create"), dept.Create)
		authed.PUT("/system/dept/:id", middleware.RequirePerm(db, "system:dept:update"), dept.Update)
		authed.DELETE("/system/dept/:id", middleware.RequirePerm(db, "system:dept:delete"), dept.Delete)

		tenant := system.NewTenantHandler(db)
		authed.GET("/system/tenant", system.RequirePrivileged(), tenant.List)
		authed.GET("/system/tenant/all", system.RequirePrivileged(), tenant.All)
		authed.POST("/system/tenant", system.RequirePrivileged(), tenant.Create)
		authed.PUT("/system/tenant/:id", system.RequirePrivileged(), tenant.Update)
		authed.DELETE("/system/tenant/:id", system.RequirePrivileged(), tenant.Delete)

		logs := system.NewLogHandler(db)
		authed.GET("/system/log/oper", middleware.RequirePerm(db, "system:log:oper"), logs.OperList)
		authed.GET("/system/log/login", middleware.RequirePerm(db, "system:log:login"), logs.LoginList)

		dict := system.NewDictHandler(db)
		authed.GET("/system/dict", middleware.RequirePerm(db, "system:dict:list"), dict.List)
		authed.POST("/system/dict", middleware.RequirePerm(db, "system:dict:create"), dict.Create)
		authed.PUT("/system/dict/:id", middleware.RequirePerm(db, "system:dict:update"), dict.Update)
		authed.DELETE("/system/dict/:id", middleware.RequirePerm(db, "system:dict:delete"), dict.Delete)
		authed.POST("/system/dict/item", middleware.RequirePerm(db, "system:dict:update"), dict.CreateItem)
		authed.PUT("/system/dict/item/:id", middleware.RequirePerm(db, "system:dict:update"), dict.UpdateItem)
		authed.DELETE("/system/dict/item/:id", middleware.RequirePerm(db, "system:dict:update"), dict.DeleteItem)
		// Dropdown source for any authenticated user; no button perm needed.
		authed.GET("/system/dict/items/:type", dict.Items)

		customer := crm.NewCustomerHandler(db)
		authed.GET("/crm/customer", middleware.RequirePerm(db, "crm:customer:list"), customer.List)
		authed.GET("/crm/customer/all", middleware.RequirePerm(db, "crm:customer:list"), customer.All)
		authed.POST("/crm/customer", middleware.RequirePerm(db, "crm:customer:create"), customer.Create)
		authed.PUT("/crm/customer/:id", middleware.RequirePerm(db, "crm:customer:update"), customer.Update)
		authed.DELETE("/crm/customer/:id", middleware.RequirePerm(db, "crm:customer:delete"), customer.Delete)

		contact := crm.NewContactHandler(db)
		authed.GET("/crm/contact", middleware.RequirePerm(db, "crm:contact:list"), contact.List)
		authed.POST("/crm/contact", middleware.RequirePerm(db, "crm:contact:create"), contact.Create)
		authed.PUT("/crm/contact/:id", middleware.RequirePerm(db, "crm:contact:update"), contact.Update)
		authed.DELETE("/crm/contact/:id", middleware.RequirePerm(db, "crm:contact:delete"), contact.Delete)

		follow := crm.NewFollowUpHandler(db)
		authed.GET("/crm/follow", middleware.RequirePerm(db, "crm:follow:list"), follow.List)
		authed.POST("/crm/follow", middleware.RequirePerm(db, "crm:follow:create"), follow.Create)
		authed.PUT("/crm/follow/:id", middleware.RequirePerm(db, "crm:follow:update"), follow.Update)
		authed.DELETE("/crm/follow/:id", middleware.RequirePerm(db, "crm:follow:delete"), follow.Delete)

		opp := crm.NewOpportunityHandler(db)
		authed.GET("/crm/opportunity", middleware.RequirePerm(db, "crm:opportunity:list"), opp.List)
		authed.GET("/crm/opportunity/all", middleware.RequirePerm(db, "crm:opportunity:list"), opp.All)
		authed.POST("/crm/opportunity", middleware.RequirePerm(db, "crm:opportunity:create"), opp.Create)
		authed.PUT("/crm/opportunity/:id", middleware.RequirePerm(db, "crm:opportunity:update"), opp.Update)
		authed.DELETE("/crm/opportunity/:id", middleware.RequirePerm(db, "crm:opportunity:delete"), opp.Delete)

		contract := crm.NewContractHandler(db)
		authed.GET("/crm/contract", middleware.RequirePerm(db, "crm:contract:list"), contract.List)
		authed.POST("/crm/contract", middleware.RequirePerm(db, "crm:contract:create"), contract.Create)
		authed.PUT("/crm/contract/:id", middleware.RequirePerm(db, "crm:contract:update"), contract.Update)
		authed.DELETE("/crm/contract/:id", middleware.RequirePerm(db, "crm:contract:delete"), contract.Delete)

		dashboard := crm.NewDashboardHandler(db)
		authed.GET("/dashboard/summary", dashboard.Summary)
	}

	return r
}
