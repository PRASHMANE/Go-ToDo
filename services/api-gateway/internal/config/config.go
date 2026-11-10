// Package config loads api-gateway configuration from the environment.
package config

import pkgconfig "github.com/PRASHMANE/Go-ToDo/pkg/config"

// Config holds everything the gateway needs to start.
type Config struct {
	Port           string
	LogLevel       string
	GinMode        string
	AuthServiceURL string
	TodoServiceURL string
	JWTSecret      string
	JWTIssuer      string
	CORSOrigins    []string
	TrustedProxies []string
	RateLimitRPS   float64
	RateLimitBurst int
}

// Load reads env vars. JWT_SECRET is required.
func Load() (Config, error) {
	secret, err := pkgconfig.Required("JWT_SECRET")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Port:           pkgconfig.String("PORT", "8080"),
		LogLevel:       pkgconfig.String("LOG_LEVEL", "info"),
		GinMode:        pkgconfig.String("GIN_MODE", "debug"),
		AuthServiceURL: pkgconfig.String("AUTH_SERVICE_URL", "http://localhost:8081"),
		TodoServiceURL: pkgconfig.String("TODO_SERVICE_URL", "http://localhost:8082"),
		JWTSecret:      secret,
		JWTIssuer:      pkgconfig.String("JWT_ISSUER", "go-todo-auth"),
		CORSOrigins:    pkgconfig.List("CORS_ORIGINS", []string{"http://localhost:5173"}),
		TrustedProxies: pkgconfig.List("TRUSTED_PROXIES", nil),
		RateLimitRPS:   pkgconfig.Float("RATE_LIMIT_RPS", 10),
		RateLimitBurst: pkgconfig.Int("RATE_LIMIT_BURST", 20),
	}, nil
}
