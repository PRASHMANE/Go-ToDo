// Package router builds the HTTP routing table for todo-service.
package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
)

// New creates the Gin engine with global middleware and routes.
func New(log *slog.Logger, ginMode string) *gin.Engine {
	gin.SetMode(ginMode)

	r := gin.New()
	r.Use(
		gin.Recovery(),         // turn panics into 500s instead of crashing
		middleware.RequestID(), // must run before Logger so the ID gets logged
		middleware.Logger(log),
	)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}
