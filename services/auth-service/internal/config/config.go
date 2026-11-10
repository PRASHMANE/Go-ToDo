// Package config loads auth-service configuration from the environment.
package config

import (
	"time"

	pkgconfig "github.com/PRASHMANE/Go-ToDo/pkg/config"
)

// Config holds everything the service needs to start.
type Config struct {
	Port        string
	LogLevel    string
	GinMode     string
	DatabaseURL string
	JWTSecret   string
	JWTIssuer   string
	JWTTTL      time.Duration
}

// Load reads env vars. DATABASE_URL and JWT_SECRET are required.
func Load() (Config, error) {
	dbURL, err := pkgconfig.Required("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	secret, err := pkgconfig.Required("JWT_SECRET")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Port:        pkgconfig.String("PORT", "8081"),
		LogLevel:    pkgconfig.String("LOG_LEVEL", "info"),
		GinMode:     pkgconfig.String("GIN_MODE", "debug"),
		DatabaseURL: dbURL,
		JWTSecret:   secret,
		JWTIssuer:   pkgconfig.String("JWT_ISSUER", "go-todo-auth"),
		JWTTTL:      pkgconfig.Duration("JWT_TTL", 24*time.Hour),
	}, nil
}
