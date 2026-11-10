package router_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/PRASHMANE/Go-ToDo/pkg/auth"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/config"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/proxy"
	"github.com/PRASHMANE/Go-ToDo/services/api-gateway/internal/router"
)

const secret = "test-secret-test-secret-test-secret"

// echoUpstream is a fake service that reports what the gateway forwarded.
func echoUpstream(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"path":       r.URL.Path,
			"user_id":    r.Header.Get("X-User-ID"),
			"request_id": r.Header.Get("X-Request-ID"),
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newGateway starts the real gateway router on a real (local) HTTP server.
// A real server is needed because httputil.ReverseProxy relies on features
// (like CloseNotify) that httptest.ResponseRecorder doesn't implement.
func newGateway(t *testing.T, upstreamURL string, rps float64, burst int) string {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	p, err := proxy.New(upstreamURL, log)
	require.NoError(t, err)
	cfg := config.Config{GinMode: gin.TestMode, JWTIssuer: "iss", CORSOrigins: []string{"http://localhost:5173"},
		RateLimitRPS: rps, RateLimitBurst: burst}
	r, err := router.New(cfg, log, auth.NewTokenManager(secret, "iss", time.Hour), router.Upstreams{Auth: p, Todo: p, Activity: p})
	require.NoError(t, err)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv.URL
}

func token(t *testing.T, userID string) string {
	tok, _, err := auth.NewTokenManager(secret, "iss", time.Hour).Generate(userID, "u@x.com")
	require.NoError(t, err)
	return tok
}

type result struct {
	Code int
	Body []byte
}

func call(t *testing.T, base, method, path string, headers map[string]string) result {
	t.Helper()
	req, err := http.NewRequest(method, base+path, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return result{Code: resp.StatusCode, Body: body}
}

func TestProtectedRouteForwardsVerifiedUser(t *testing.T) {
	gw := newGateway(t, echoUpstream(t).URL, 100, 100)
	w := call(t, gw, http.MethodGet, "/api/v1/todos/abc", map[string]string{
		"Authorization": "Bearer " + token(t, "user-42"),
		"X-User-ID":     "attacker", // forged header must be replaced
		"X-Request-ID":  "rid-1",
	})
	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]string
	require.NoError(t, json.Unmarshal(w.Body, &got))
	assert.Equal(t, "user-42", got["user_id"])
	assert.Equal(t, "/api/v1/todos/abc", got["path"])
	assert.Equal(t, "rid-1", got["request_id"])
}

func TestPublicRouteStripsForgedUserHeader(t *testing.T) {
	gw := newGateway(t, echoUpstream(t).URL, 100, 100)
	w := call(t, gw, http.MethodPost, "/api/v1/auth/login", map[string]string{"X-User-ID": "attacker"})
	require.Equal(t, http.StatusOK, w.Code)
	var got map[string]string
	require.NoError(t, json.Unmarshal(w.Body, &got))
	assert.Empty(t, got["user_id"])
}

func TestRejectsMissingOrBadToken(t *testing.T) {
	gw := newGateway(t, echoUpstream(t).URL, 100, 100)
	assert.Equal(t, http.StatusUnauthorized, call(t, gw, "GET", "/api/v1/todos", nil).Code)
	assert.Equal(t, http.StatusUnauthorized,
		call(t, gw, "GET", "/api/v1/todos", map[string]string{"Authorization": "Bearer nope"}).Code)
	assert.Equal(t, http.StatusUnauthorized, call(t, gw, "GET", "/api/v1/auth/me", nil).Code)
	assert.Equal(t, http.StatusUnauthorized, call(t, gw, "GET", "/api/v1/activity", nil).Code)
}

func TestRateLimit(t *testing.T) {
	gw := newGateway(t, echoUpstream(t).URL, 1, 3)
	codes := map[int]int{}
	for i := 0; i < 6; i++ {
		codes[call(t, gw, http.MethodPost, "/api/v1/auth/login", nil).Code]++
	}
	assert.Equal(t, 3, codes[http.StatusOK])
	assert.Equal(t, 3, codes[http.StatusTooManyRequests])
}

func TestUpstreamDownGives502(t *testing.T) {
	gw := newGateway(t, "http://127.0.0.1:1", 100, 100) // nothing listens on port 1
	w := call(t, gw, http.MethodPost, "/api/v1/auth/login", nil)
	assert.Equal(t, http.StatusBadGateway, w.Code)
	assert.JSONEq(t, `{"error":{"code":"bad_gateway","message":"upstream service unavailable"}}`, string(w.Body))
}
