// Package handler exposes todo use cases over HTTP.
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/pkg/response"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/dto"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/model"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/service"
)

// TodoHandler holds HTTP handlers for /todos.
type TodoHandler struct {
	svc *service.TodoService
	log *slog.Logger
}

// NewTodoHandler builds a TodoHandler.
func NewTodoHandler(svc *service.TodoService, log *slog.Logger) *TodoHandler {
	return &TodoHandler{svc: svc, log: log}
}

// RegisterRoutes mounts the todo routes on a router group.
func (h *TodoHandler) RegisterRoutes(rg *gin.RouterGroup) {
	todos := rg.Group("/todos")
	todos.GET("", h.list)
	todos.POST("", h.create)
	todos.GET("/:id", h.get)
	todos.PATCH("/:id", h.update)
	todos.DELETE("/:id", h.delete)
}

func (h *TodoHandler) create(c *gin.Context) {
	var req dto.CreateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	todo, err := h.svc.Create(c.Request.Context(), userID(c), service.CreateTodoInput{
		Title:       req.Title,
		Description: req.Description,
		Priority:    model.Priority(req.Priority),
		DueDate:     req.DueDate,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.FromModel(todo))
}

func (h *TodoHandler) list(c *gin.Context) {
	var q dto.ListTodosQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	res, err := h.svc.List(c.Request.Context(), userID(c), service.ListTodosInput{
		Status:   q.Status,
		Priority: model.Priority(q.Priority),
		Search:   q.Q,
		Page:     q.Page,
		PageSize: q.PageSize,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	items := make([]dto.TodoResponse, 0, len(res.Items)) // non-nil → JSON [] not null
	for i := range res.Items {
		items = append(items, dto.FromModel(&res.Items[i]))
	}
	c.JSON(http.StatusOK, dto.ListTodosResponse{
		Items: items, Page: q.Page, PageSize: q.PageSize, Total: res.Total,
	})
}

func (h *TodoHandler) get(c *gin.Context) {
	id, ok := todoID(c)
	if !ok {
		return
	}
	todo, err := h.svc.Get(c.Request.Context(), userID(c), id)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromModel(todo))
}

func (h *TodoHandler) update(c *gin.Context) {
	id, ok := todoID(c)
	if !ok {
		return
	}
	var req dto.UpdateTodoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	in := service.UpdateTodoInput{
		Title:        req.Title,
		Description:  req.Description,
		Completed:    req.Completed,
		DueDate:      req.DueDate,
		ClearDueDate: req.ClearDueDate,
	}
	if req.Priority != nil {
		p := model.Priority(*req.Priority)
		in.Priority = &p
	}
	todo, err := h.svc.Update(c.Request.Context(), userID(c), id, in)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromModel(todo))
}

func (h *TodoHandler) delete(c *gin.Context) {
	id, ok := todoID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), userID(c), id); err != nil {
		h.handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleError maps domain errors to HTTP responses in ONE place.
func (h *TodoHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		response.Error(c, http.StatusNotFound, "not_found", "todo not found")
	case errors.Is(err, model.ErrValidation):
		response.Error(c, http.StatusBadRequest, "validation_error", err.Error())
	default:
		// Log the real error, but never leak internals to the client.
		h.log.Error("internal error", "err", err, "request_id", c.GetString(middleware.ContextRequestID))
		response.Error(c, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}

func userID(c *gin.Context) string { return c.GetString(middleware.ContextUserID) }

// todoID reads and validates the :id path param (must be a UUID).
func todoID(c *gin.Context) (string, bool) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		response.Error(c, http.StatusNotFound, "not_found", "todo not found")
		return "", false
	}
	return id, true
}
