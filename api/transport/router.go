package transport

import (
	"github.com/gin-gonic/gin"
	"github.com/swaggo/files"
	"github.com/swaggo/gin-swagger"
	"net/http"
	"os"
)

type Router struct {
	*gin.Engine
}

func NewRouter(ginMode string) *Router {
	gin.SetMode(ginMode)
	g := gin.New()

	//Everything else is 404
	g.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
	})

	if os.Getenv("APP_ENV") == "local" {
		g.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	return &Router{g}
}

func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("x-admin-token")
		if token != os.Getenv("ADMIN_TOKEN") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
