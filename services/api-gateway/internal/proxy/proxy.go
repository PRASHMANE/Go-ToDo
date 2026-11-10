// Package proxy builds reverse proxies to upstream services.
package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

// New creates a reverse proxy to target (e.g. "http://todo-service:8082").
// The incoming path is kept as-is: /api/v1/todos → target/api/v1/todos.
func New(target string, log *slog.Logger) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, fmt.Errorf("parse upstream url %q: %w", target, err)
	}

	return &httputil.ReverseProxy{
		// Rewrite (Go 1.20+) is the modern, safer hook. It receives the
		// incoming request (In) and the outgoing copy (Out).
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(u)       // scheme + host of upstream, keep the path
			pr.SetXForwarded() // X-Forwarded-For / -Host / -Proto
		},
		Transport: &http.Transport{
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Error("upstream error", "upstream", u.Host, "path", r.URL.Path, "err", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"code":"bad_gateway","message":"upstream service unavailable"}}`))
		},
	}, nil
}

// Handler adapts a ReverseProxy (an http.Handler) into a gin.HandlerFunc.
func Handler(p *httputil.ReverseProxy) gin.HandlerFunc {
	return func(c *gin.Context) {
		p.ServeHTTP(c.Writer, c.Request)
	}
}
