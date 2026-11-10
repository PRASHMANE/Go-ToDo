// Package router builds the HTTP routing table for activity-service.
package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/health"
	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/handler"
)

// New creates the Gin engine with global middleware and routes.
// deps are checked by /readyz.
func New(log *slog.Logger, ginMode string, h *handler.ActivityHandler, deps map[string]health.Pinger) *gin.Engine {
	gin.SetMode(ginMode)
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(log), middleware.Metrics("activity-service"))
	health.Register(r, deps) // /healthz, /readyz, /metrics

	v1 := r.Group("/api/v1", middleware.RequireUserHeader())
	h.RegisterRoutes(v1)
	return r
}
