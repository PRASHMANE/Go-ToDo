// Package middleware holds gateway-only middleware.
package middleware

import (
	"github.com/gin-gonic/gin"

	pkgmw "github.com/PRASHMANE/Go-ToDo/pkg/middleware"
)

// StripIdentityHeaders removes identity headers a client might try to forge.
// MUST run on every request, before anything else reads them.
func StripIdentityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Del(pkgmw.UserIDHeader)
		c.Next()
	}
}

// ForwardUserID copies the verified user ID (set by RequireJWT) into the
// X-User-ID header that upstream services trust.
func ForwardUserID() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Set(pkgmw.UserIDHeader, c.GetString(pkgmw.ContextUserID))
		c.Next()
	}
}
