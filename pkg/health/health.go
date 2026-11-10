// Package health provides liveness, readiness and metrics endpoints.
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Pinger is satisfied by *pgxpool.Pool (and anything else with Ping).
type Pinger interface {
	Ping(ctx context.Context) error
}

// Register mounts:
//   - GET /healthz  liveness:  "the process is alive" (never touches dependencies)
//   - GET /readyz   readiness: "my dependencies are reachable, send me traffic"
//   - GET /metrics  Prometheus scrape endpoint
//
// deps maps a dependency name (e.g. "db") to its Pinger; nil entries are skipped.
func Register(r *gin.Engine, deps map[string]Pinger) {
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		checks := gin.H{}
		ready := true
		for name, p := range deps {
			if p == nil {
				continue
			}
			if err := p.Ping(ctx); err != nil {
				checks[name] = err.Error()
				ready = false
			} else {
				checks[name] = "ok"
			}
		}
		if !ready {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "checks": checks})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready", "checks": checks})
	})

	r.GET("/metrics", gin.WrapH(promhttp.Handler())) // gin.WrapH adapts an http.Handler
}
