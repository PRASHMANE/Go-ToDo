package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PRASHMANE/Go-ToDo/pkg/response"
)

type req struct {
	Email    string `json:"email" binding:"required,email"`
	PageSize int    `json:"page_size" binding:"min=1,max=100"`
	Name     string `json:"name" binding:"max=3"`
	Kind     string `json:"kind" binding:"omitempty,oneof=a b"`
}

func TestBindErrorMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := map[string]string{
		`{"email":"","page_size":1}`:                        "email is required",
		`{"email":"nope","page_size":1}`:                    "email must be a valid email address",
		`{"email":"a@b.co","page_size":500}`:                "page_size must be at most 100",
		`{"email":"a@b.co","page_size":1,"name":"toolong"}`: "name must be at most 3 characters",
		`{"email":"a@b.co","page_size":1,"kind":"z"}`:       "kind must be one of: a b",
		`{"email":"","page_size":0}`:                        "email is required; page_size must be at least 1",
		`{oops`:                                             "malformed request body or query",
	}
	for body, want := range tests {
		t.Run(want, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")

			var r req
			err := c.ShouldBindJSON(&r)
			require.Error(t, err)
			response.BindError(c, err)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			var got response.ErrorBody
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			assert.Equal(t, "invalid_request", got.Error.Code)
			assert.Equal(t, want, got.Error.Message)
		})
	}
}
