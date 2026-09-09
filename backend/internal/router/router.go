package router

import (
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/lihefengbj/nanyicrm/backend/internal/config"
	"github.com/lihefengbj/nanyicrm/backend/internal/middleware"
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
	authed.Use(middleware.JWTAuth(cfg.JWT.SigningKey), middleware.OperLog(db))
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

		logs := system.NewLogHandler(db)
		authed.GET("/system/log/oper", middleware.RequirePerm(db, "system:log:oper"), logs.OperList)
		authed.GET("/system/log/login", middleware.RequirePerm(db, "system:log:login"), logs.LoginList)
	}

	return r
}
