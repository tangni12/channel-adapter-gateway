package middleware

import (
	"net/http"

	"channel-adapter-gateway/internal/service"

	"github.com/gin-gonic/gin"
)

func AdminAuth(auth *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := service.BearerToken(c.GetHeader("Authorization"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "please login"})
			return
		}
		claims, err := auth.ParseToken(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "invalid login token"})
			return
		}
		c.Set("claims", claims)
		c.Next()
	}
}
