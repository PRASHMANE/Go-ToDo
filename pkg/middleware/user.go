package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/response"
)

// ContextUserID is the gin.Context key that holds the authenticated user's ID.
const ContextUserID = "user_id"

// UserIDHeader is set by the API gateway after it validates the JWT.
const UserIDHeader = "X-User-ID"

// FixedUser sets a hard-coded user ID. Development only (Phase B2–B4),
// until the gateway exists.
func FixedUser(userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(ContextUserID, userID)
		c.Next()
	}
}

// RequireUserHeader trusts the X-User-ID header set by the gateway (Phase B5).
func RequireUserHeader() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(UserIDHeader)
		if id == "" {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "missing user identity")
			return
		}
		c.Set(ContextUserID, id)
		c.Next()
	}
}
