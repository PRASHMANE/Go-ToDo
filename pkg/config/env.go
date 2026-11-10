// Package config provides small helpers to read typed environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// String returns the env var value, or fallback if it is unset or empty.
func String(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// Required returns the env var value or an error if it is missing.
// Use it for secrets: the service should refuse to start without them.
func Required(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("required environment variable %s is not set", key)
	}
	return v, nil
}

// Int parses an integer env var, falling back on missing or invalid values.
func Int(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

// Float parses a float env var.
func Float(key string, fallback float64) float64 {
	v, err := strconv.ParseFloat(os.Getenv(key), 64)
	if err != nil {
		return fallback
	}
	return v
}

// Duration parses values like "15m", "24h", "500ms".
func Duration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}

// List splits a comma-separated env var: "a, b,c" → ["a","b","c"].
func List(key string, fallback []string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
