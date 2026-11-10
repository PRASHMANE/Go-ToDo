// Package router builds the HTTP routing table for auth-service.
package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/handler"
)

// New creates the Gin engine with global middleware and routes.
func New(log *slog.Logger, ginMode string, tm *auth.TokenManager, h *handler.AuthHandler) *gin.Engine {
	gin.SetMode(ginMode)
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(log))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	v1 := r.Group("/api/v1")
	h.RegisterRoutes(v1, middleware.RequireJWT(tm))
	return r
}
