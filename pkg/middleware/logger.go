package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger logs one structured line per request AFTER the handler has run.
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next() // run the rest of the chain (handlers) first

		status := c.Writer.Status()
		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"route", c.FullPath(), // "/api/v1/todos/:id" (pattern, good for grouping)
			"status", status,
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
			"request_id", c.GetString(ContextRequestID),
		}
		switch {
		case status >= 500:
			log.Error("request", attrs...)
		case status >= 400:
			log.Warn("request", attrs...)
		default:
			log.Info("request", attrs...)
		}
	}
}
