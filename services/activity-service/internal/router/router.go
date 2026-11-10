// Package router builds the HTTP routing table for activity-service.
package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/handler"
)

// New creates the Gin engine with global middleware and routes.
func New(log *slog.Logger, ginMode string, h *handler.ActivityHandler) *gin.Engine {
	gin.SetMode(ginMode)
	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(log))
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	v1 := r.Group("/api/v1", middleware.RequireUserHeader())
	h.RegisterRoutes(v1)
	return r
}
