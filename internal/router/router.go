package router

import (
	"net/http"

	"channel-adapter-gateway/internal/config"
	"channel-adapter-gateway/internal/middleware"
	"channel-adapter-gateway/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Dependencies struct {
	Config *config.Config
	DB     *gorm.DB
	Auth   *service.AuthService
	Admin  *service.AdminService
	Proxy  *service.ProxyService
}

func New(dep Dependencies) *gin.Engine {
	engine := gin.Default()
	engine.MaxMultipartMemory = 128 << 20

	// 路由按职责拆分：官方协议入口、后台管理接口、前端静态资源分别注册。
	registerHealthRoutes(engine)
	registerOpenAIRoutes(engine, dep.Proxy)
	registerAdminRoutes(engine, dep)
	registerWebRoutes(engine)
	return engine
}

func registerHealthRoutes(engine *gin.Engine) {
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

func registerWebRoutes(engine *gin.Engine) {
	engine.Static("/assets", "./web/dist/assets")
	engine.GET("/", func(c *gin.Context) {
		c.File("./web/dist/index.html")
	})
}

func adminGroup(engine *gin.Engine, dep Dependencies) *gin.RouterGroup {
	api := engine.Group("/api")
	api.Use(middleware.AdminAuth(dep.Auth))
	return api
}
