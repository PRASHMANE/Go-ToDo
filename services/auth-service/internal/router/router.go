// Package router builds the HTTP routing table for auth-service.
package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	"github.com/PRASHMANE/Go-ToDo/pkg/health"
	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/handler"
)

// New creates the Gin engine with global middleware and routes.
// deps are checked by /readyz.
func New(log *slog.Logger, ginMode string, tm *auth.TokenManager, h *handler.AuthHandler, deps map[string]health.Pinger) *gin.Engine {
	gin.SetMode(ginMode)
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(log), middleware.Metrics("auth-service"))
	health.Register(r, deps) // /healthz, /readyz, /metrics

	v1 := r.Group("/api/v1")
	h.RegisterRoutes(v1, middleware.RequireJWT(tm))
	return r
}
