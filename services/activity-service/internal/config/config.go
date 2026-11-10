// Package config loads activity-service configuration from the environment.
package config

import pkgconfig "github.com/PRASHMANE/Go-ToDo/pkg/config"

// Config holds everything the service needs to start.
type Config struct {
	Port        string
	LogLevel    string
	GinMode     string
	DatabaseURL string
	NATSURL     string
}

// Load reads env vars. DATABASE_URL is required.
func Load() (Config, error) {
	dbURL, err := pkgconfig.Required("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Port:        pkgconfig.String("PORT", "8083"),
		LogLevel:    pkgconfig.String("LOG_LEVEL", "info"),
		GinMode:     pkgconfig.String("GIN_MODE", "debug"),
		DatabaseURL: dbURL,
		NATSURL:     pkgconfig.String("NATS_URL", "nats://localhost:4222"),
	}, nil
}
