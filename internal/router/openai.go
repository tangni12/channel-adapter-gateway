package router

import (
	"channel-adapter-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

func registerOpenAIRoutes(engine *gin.Engine, proxy *service.ProxyService) {
	engine.GET("/v1/models", proxy.ListModels)
	engine.POST("/v1/images/generations", proxy.OpenAIImageGeneration)
	engine.POST("/v1/images/edits", proxy.OpenAIImageEdit)
}
