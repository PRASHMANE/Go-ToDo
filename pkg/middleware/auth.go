package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	"github.com/PRASHMANE/Go-ToDo/pkg/response"
)

// ContextUserEmail holds the email from the token.
const ContextUserEmail = "user_email"

// RequireJWT validates "Authorization: Bearer <token>" and stores the user in the context.
func RequireJWT(tm *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		claims, err := tm.Parse(token)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		c.Set(ContextUserID, claims.Subject)
		c.Set(ContextUserEmail, claims.Email)
		c.Next()
	}
}
