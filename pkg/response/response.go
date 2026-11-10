// Package response defines the JSON error envelope shared by all services.
package response

import "github.com/gin-gonic/gin"

// ErrorBody is the top-level error JSON: {"error": {...}}.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries a machine-readable code and a human message.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error writes the error JSON and ABORTS the middleware chain.
// The caller must still `return` afterwards.
func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorBody{Error: ErrorDetail{Code: code, Message: message}})
}
