package handler

import (
	"net/http"

	"channel-adapter-gateway/internal/official"

	"github.com/gin-gonic/gin"
)

type OfficialHandler struct{}

func NewOfficialHandler() *OfficialHandler {
	return &OfficialHandler{}
}

func (h *OfficialHandler) ListEndpoints(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": official.AllEndpoints()})
}

func (h *OfficialHandler) GetEndpoint(c *gin.Context) {
	endpoint, ok := official.FindEndpoint(c.Param("key"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "official endpoint not found"})
		return
	}
	c.JSON(http.StatusOK, endpoint)
}
