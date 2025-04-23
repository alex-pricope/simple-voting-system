package transport

import (
	"github.com/alex-pricope/simple-voting-system/logging"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"net/http"
	"os"
)

func NewRouter(ginMode string) *gin.Engine {
	gin.SetMode(ginMode)
	engine := gin.New()
	engine.Use(CORSMiddleware())

	//Bypass swagger for non-local
	if os.Getenv("APP_ENV") == "local" {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	engine.NoRoute(NoRouteHandler())

	return engine
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-admin-token")

		if c.Request.Method == "OPTIONS" {
			logging.Log.Infof("OPTIONS request received:%s", c.Request.URL.Path)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func NoRouteHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		logging.Log.Infof("No routed request received for:%s", c.Request.URL.Path)
		c.JSON(http.StatusNotFound, gin.H{"code": "PAGE_NOT_FOUND", "message": "Page not found"})
	}
}

func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("x-admin-token")
		expected := os.Getenv("ADMIN_TOKEN")

		if token == "" || token != expected {
			logging.Log.Warnf("ADMIN: Unauthorized access attempt to %s", c.Request.URL.Path)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
