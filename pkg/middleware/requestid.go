// Package middleware contains Gin middleware shared by all services.
package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDHeader is propagated across every service hop.
const RequestIDHeader = "X-Request-ID"

// ContextRequestID is the gin.Context key holding the request ID.
const ContextRequestID = "request_id"

// RequestID reuses an incoming X-Request-ID or generates a new one,
// stores it in the context, and echoes it in the response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(RequestIDHeader)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(ContextRequestID, id)
		c.Request.Header.Set(RequestIDHeader, id) // so a reverse proxy forwards it
		c.Header(RequestIDHeader, id)             // so the client sees it
		c.Next()
	}
}
