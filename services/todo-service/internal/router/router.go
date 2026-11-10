// Package router builds the HTTP routing table for todo-service.
package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/handler"
)

// DemoUserID is used until the API gateway exists (Phase B5).
const DemoUserID = "00000000-0000-0000-0000-000000000001"

// New creates the Gin engine with global middleware and routes.
func New(log *slog.Logger, ginMode string, todos *handler.TodoHandler) *gin.Engine {
	gin.SetMode(ginMode)

	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.Logger(log))

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	v1 := r.Group("/api/v1", middleware.FixedUser(DemoUserID)) // B5: RequireUserHeader()
	todos.RegisterRoutes(v1)

	return r
}
