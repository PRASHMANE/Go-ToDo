// Package handler exposes the activity feed over HTTP.
package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/pkg/response"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/dto"
	"github.com/PRASHMANE/Go-ToDo/services/activity-service/internal/service"
)

// ActivityHandler holds HTTP handlers for /activity.
type ActivityHandler struct {
	svc *service.ActivityService
	log *slog.Logger
}

// NewActivityHandler builds an ActivityHandler.
func NewActivityHandler(svc *service.ActivityService, log *slog.Logger) *ActivityHandler {
	return &ActivityHandler{svc: svc, log: log}
}

// RegisterRoutes mounts the activity routes.
func (h *ActivityHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/activity", h.list)
}

func (h *ActivityHandler) list(c *gin.Context) {
	var q dto.ListActivityQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.BindError(c, err)
		return
	}
	items, err := h.svc.List(c.Request.Context(), c.GetString(middleware.ContextUserID), q.Limit)
	if err != nil {
		h.log.Error("internal error", "err", err, "request_id", c.GetString(middleware.ContextRequestID))
		response.Error(c, http.StatusInternalServerError, "internal_error", "something went wrong")
		return
	}
	out := make([]dto.ActivityResponse, 0, len(items))
	for i := range items {
		out = append(out, dto.FromModel(&items[i]))
	}
	c.JSON(http.StatusOK, dto.ListActivityResponse{Items: out})
}
