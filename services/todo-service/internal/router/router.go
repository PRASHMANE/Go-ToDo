// Package router builds the HTTP routing table for todo-service.
package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/handler"
)

// New creates the Gin engine with global middleware and routes.
func New(log *slog.Logger, ginMode string, todos *handler.TodoHandler) *gin.Engine {
	gin.SetMode(ginMode)

	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(log))

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// The gateway validated the JWT and set X-User-ID; we trust it (internal network only).
	v1 := r.Group("/api/v1", middleware.RequireUserHeader())
	todos.RegisterRoutes(v1)

	return r
}
