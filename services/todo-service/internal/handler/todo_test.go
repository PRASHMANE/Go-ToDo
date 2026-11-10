package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/dto"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/handler"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/repository"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/router"
	"github.com/PRASHMANE/Go-ToDo/services/todo-service/internal/service"
)

func newTestServer() *gin.Engine {
	log := slog.New(slog.NewTextHandler(io.Discard, nil)) // silence logs in tests
	svc := service.NewTodoService(repository.NewMemoryTodoRepository(), nil)
	return router.New(log, gin.TestMode, handler.NewTodoHandler(svc, log), nil)
}

func do(t *testing.T, r http.Handler, method, path, body, userID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req.Header.Set("X-User-ID", userID)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

const (
	u1 = "11111111-1111-1111-1111-111111111111"
	u2 = "22222222-2222-2222-2222-222222222222"
)

func TestCreateGetUpdateDelete(t *testing.T) {
	r := newTestServer()

	w := do(t, r, http.MethodPost, "/api/v1/todos", `{"title":"write tests","priority":"high"}`, u1)
	require.Equal(t, http.StatusCreated, w.Code)

	var created dto.TodoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "write tests", created.Title)

	w = do(t, r, http.MethodGet, "/api/v1/todos/"+created.ID, "", u1)
	assert.Equal(t, http.StatusOK, w.Code)

	w = do(t, r, http.MethodGet, "/api/v1/todos/"+created.ID, "", u2) // other user
	assert.Equal(t, http.StatusNotFound, w.Code)

	w = do(t, r, http.MethodPatch, "/api/v1/todos/"+created.ID, `{"completed":true}`, u1)
	require.Equal(t, http.StatusOK, w.Code)
	var updated dto.TodoResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.True(t, updated.Completed)

	w = do(t, r, http.MethodDelete, "/api/v1/todos/"+created.ID, "", u1)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestListReturnsEmptyArrayNotNull(t *testing.T) {
	w := do(t, newTestServer(), http.MethodGet, "/api/v1/todos", "", u1)
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"items":[],"page":1,"page_size":20,"total":0}`, w.Body.String())
}

func TestValidationAndAuthErrors(t *testing.T) {
	r := newTestServer()
	tests := []struct {
		name, method, path, body, user string
		wantStatus                     int
		wantCode                       string
		wantMessage                    string
	}{
		{"missing user header", "GET", "/api/v1/todos", "", "", 401, "unauthorized", "missing user identity"},
		{"empty title", "POST", "/api/v1/todos", `{"title":""}`, u1, 400, "invalid_request", "title is required"},
		{"bad priority", "POST", "/api/v1/todos", `{"title":"x","priority":"urgent"}`, u1, 400, "invalid_request", "priority must be one of: low medium high"},
		{"malformed json", "POST", "/api/v1/todos", `{bad`, u1, 400, "invalid_request", "malformed request body or query"},
		{"bad page size", "GET", "/api/v1/todos?page_size=1000", "", u1, 400, "invalid_request", "page_size must be at most 100"},
		{"non-uuid id", "GET", "/api/v1/todos/abc", "", u1, 404, "not_found", "todo not found"},
		{"unknown id", "DELETE", "/api/v1/todos/" + u1, "", u1, 404, "not_found", "todo not found"},
		{"whitespace title", "POST", "/api/v1/todos", `{"title":"   "}`, u1, 400, "validation_error", "validation failed: title is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := do(t, r, tc.method, tc.path, tc.body, tc.user)
			assert.Equal(t, tc.wantStatus, w.Code)
			var body struct {
				Error struct{ Code, Message string }
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Error.Code)
			assert.Equal(t, tc.wantMessage, body.Error.Message)
		})
	}
}
