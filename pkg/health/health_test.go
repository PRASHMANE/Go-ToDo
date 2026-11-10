package health_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/PRASHMANE/Go-ToDo/pkg/health"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func get(r http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	up := gin.New()
	health.Register(up, map[string]health.Pinger{"db": fakePinger{}, "optional": nil})
	assert.Equal(t, http.StatusOK, get(up, "/healthz").Code)
	assert.Equal(t, http.StatusOK, get(up, "/readyz").Code)
	m := get(up, "/metrics")
	assert.Equal(t, http.StatusOK, m.Code)
	assert.True(t, strings.Contains(m.Body.String(), "go_goroutines"))

	down := gin.New()
	health.Register(down, map[string]health.Pinger{"db": fakePinger{err: errors.New("connection refused")}})
	assert.Equal(t, http.StatusOK, get(down, "/healthz").Code, "liveness must not depend on the DB")
	w := get(down, "/readyz")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "connection refused")
}
