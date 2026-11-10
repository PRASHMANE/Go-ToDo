// Package router builds the HTTP routing table for todo-service.
package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/health"
	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/handler"
)

// New creates the Gin engine with global middleware and routes.
// deps are checked by /readyz (nil map = nothing to check).
func New(log *slog.Logger, ginMode string, todos *handler.TodoHandler, deps map[string]health.Pinger) *gin.Engine {
	gin.SetMode(ginMode)

	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(log), middleware.Metrics("todo-service"))

	health.Register(r, deps) // /healthz, /readyz, /metrics

	// The gateway validated the JWT and set X-User-ID; we trust it (internal network only).
	v1 := r.Group("/api/v1", middleware.RequireUserHeader())
	todos.RegisterRoutes(v1)

	return r
}
