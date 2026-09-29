package controller

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "fizzbuzz-api/docs" // registers the OpenAPI spec served by Swagger UI
)

// NewRouter wires the routes and middlewares onto a fresh gin engine.
func NewRouter(uc UseCase, log *slog.Logger) *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies(nil) // do not trust X-Forwarded-For by default
	r.HandleMethodNotAllowed = true
	r.Use(gin.Recovery(), requestLogger(log))

	ctl := &Controller{uc: uc, log: log}

	r.GET("/healthz", ctl.health)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1")
	v1.POST("/fizzbuzz", ctl.fizzBuzz)
	v1.GET("/stats", ctl.stats)
	v1.GET("/history", ctl.history)
	v1.GET("/history/export", ctl.exportHistory)

	return r
}

// requestLogger emits one structured log line per request.
func requestLogger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration", time.Since(start),
			"client_ip", c.ClientIP(),
		)
	}
}
