package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/PRASHMANE/Go-ToDo/pkg/response"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      rate.Limit
	burst    int
}

func (l *ipRateLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.rps, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanup forgets IPs not seen for 3 minutes, so memory doesn't grow forever.
func (l *ipRateLimiter) cleanup() {
	for range time.Tick(time.Minute) {
		l.mu.Lock()
		for ip, v := range l.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

// RateLimit allows rps requests/second per client IP with bursts up to burst.
// NOTE: state is per gateway instance. With several replicas you'd move this
// to Redis so all instances share one budget.
func RateLimit(rps float64, burst int) gin.HandlerFunc {
	l := &ipRateLimiter{visitors: make(map[string]*visitor), rps: rate.Limit(rps), burst: burst}
	go l.cleanup()

	return func(c *gin.Context) {
		if !l.get(c.ClientIP()).Allow() {
			c.Header("Retry-After", "1")
			response.Error(c, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		c.Next()
	}
}
