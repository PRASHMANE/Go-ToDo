// Package handler exposes auth over HTTP.
package handler

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/pkg/response"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/dto"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/model"
	"github.com/PRASHMANE/Go-ToDo/services/auth-service/internal/service"
)

// AuthHandler holds HTTP handlers for /auth.
type AuthHandler struct {
	svc *service.AuthService
	log *slog.Logger
}

// NewAuthHandler builds an AuthHandler.
func NewAuthHandler(svc *service.AuthService, log *slog.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, log: log}
}

// RegisterRoutes mounts /auth routes. requireAuth protects /me.
func (h *AuthHandler) RegisterRoutes(rg *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	g := rg.Group("/auth")
	g.POST("/register", h.register)
	g.POST("/login", h.login)
	g.GET("/me", requireAuth, h.me)
}

func (h *AuthHandler) register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	u, err := h.svc.Register(c.Request.Context(), req.Email, req.Name, req.Password)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, dto.FromUser(u))
}

func (h *AuthHandler) login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	res, err := h.svc.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.LoginResponse{
		AccessToken: res.AccessToken,
		TokenType:   "Bearer",
		ExpiresAt:   res.ExpiresAt.UTC(),
		User:        dto.FromUser(res.User),
	})
}

func (h *AuthHandler) me(c *gin.Context) {
	u, err := h.svc.Me(c.Request.Context(), c.GetString(middleware.ContextUserID))
	if err != nil {
		h.handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, dto.FromUser(u))
}

func (h *AuthHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrEmailTaken):
		response.Error(c, http.StatusConflict, "email_taken", err.Error())
	case errors.Is(err, model.ErrInvalidCredentials):
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", err.Error())
	case errors.Is(err, model.ErrUserNotFound):
		response.Error(c, http.StatusNotFound, "not_found", "user not found")
	default:
		h.log.Error("internal error", "err", err, "request_id", c.GetString(middleware.ContextRequestID))
		response.Error(c, http.StatusInternalServerError, "internal_error", "something went wrong")
	}
}
