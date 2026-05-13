package router

import (
	"channel-adapter-gateway/internal/handler"

	"github.com/gin-gonic/gin"
)

func registerAdminRoutes(engine *gin.Engine, dep Dependencies) {
	admin := handler.NewAdminHandler(dep.DB, dep.Admin)
	official := handler.NewOfficialHandler()

	publicAPI := engine.Group("/api")
	publicAPI.POST("/auth/login", admin.Login)

	api := adminGroup(engine, dep)
	api.GET("/me", admin.Me)
	api.GET("/official/endpoints", official.ListEndpoints)
	api.GET("/official/endpoints/:key", official.GetEndpoint)

	api.GET("/providers", admin.ListProviders)
	api.POST("/providers", admin.CreateProvider)
	api.PUT("/providers/:id", admin.UpdateProvider)
	api.DELETE("/providers/:id", admin.DeleteProvider)

	api.GET("/users", admin.ListUsers)
	api.POST("/users", admin.CreateUser)
	api.PUT("/users/:id", admin.UpdateUser)
	api.DELETE("/users/:id", admin.DeleteUser)

	api.GET("/mappings", admin.ListMappings)
	api.POST("/mappings", admin.CreateMapping)
	api.PUT("/mappings/:id", admin.UpdateMapping)
	api.DELETE("/mappings/:id", admin.DeleteMapping)

	api.GET("/request-logs", admin.ListLogs)
}
