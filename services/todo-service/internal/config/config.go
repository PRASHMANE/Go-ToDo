// Package config loads todo-service configuration from the environment.
package config

import pkgconfig "github.com/PRASHMANE/Go-ToDo/pkg/config"

// Config holds everything the service needs to start.
type Config struct {
	Port     string
	LogLevel string
	GinMode  string
}

// Load reads env vars, applying local-development defaults.
func Load() (Config, error) {
	return Config{
		Port:     pkgconfig.String("PORT", "8082"),
		LogLevel: pkgconfig.String("LOG_LEVEL", "info"),
		GinMode:  pkgconfig.String("GIN_MODE", "debug"),
	}, nil
}
