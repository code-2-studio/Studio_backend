package utils

import (
	"os"
)

// GetEnv helper to fetch environment variable with fallback
func GetEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}
