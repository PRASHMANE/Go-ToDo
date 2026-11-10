// Package router maps public paths to upstream services.
package router

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	pkgmw "github.com/PRASHMANE/Go-ToDo/pkg/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/config"
	gwmw "github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/middleware"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/proxy"
)

// Upstreams groups the reverse proxies.
type Upstreams struct {
	Auth *httputil.ReverseProxy
	Todo *httputil.ReverseProxy
}

// New builds the gateway's gin.Engine: global middleware chain + routing table.
func New(cfg config.Config, log *slog.Logger, tm *auth.TokenManager, up Upstreams) (*gin.Engine, error) {
	gin.SetMode(cfg.GinMode)
	r := gin.New()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil { // nil = trust nobody
		return nil, err
	}

	// ORDER MATTERS: this is the chain every request goes through.
	r.Use(
		gin.Recovery(),
		pkgmw.RequestID(),
		pkgmw.Logger(log),
		cors.New(cors.Config{
			AllowOrigins:  cfg.CORSOrigins,
			AllowMethods:  []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
			AllowHeaders:  []string{"Authorization", "Content-Type", pkgmw.RequestIDHeader},
			ExposeHeaders: []string{pkgmw.RequestIDHeader},
			MaxAge:        12 * time.Hour,
		}),
		gwmw.RateLimit(cfg.RateLimitRPS, cfg.RateLimitBurst),
		gwmw.StripIdentityHeaders(),
	)

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	authProxy := proxy.Handler(up.Auth)
	todoProxy := proxy.Handler(up.Todo)
	requireAuth := []gin.HandlerFunc{pkgmw.RequireJWT(tm), gwmw.ForwardUserID()}

	v1 := r.Group("/api/v1")

	// public
	v1.POST("/auth/register", authProxy)
	v1.POST("/auth/login", authProxy)

	// protected
	v1.GET("/auth/me", append(requireAuth, authProxy)...)

	todos := v1.Group("/todos", requireAuth...)
	todos.Any("", todoProxy)       // /api/v1/todos
	todos.Any("/*path", todoProxy) // /api/v1/todos/<anything>

	return r, nil
}
