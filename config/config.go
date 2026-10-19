package config

import (
	"os"
	"studio_backend/utils"
)

type Config struct {
	Env            string
	Port           string
	DatabaseURL    string
	DBName         string
	DBUser         string
	DBPassword     string
	DBPort         string
	DBHost         string
	DBSSLMode      string
	DBSSLRootCert  string
	UserContextKey string
	JWTSecret      string
}

func Load() *Config {
	cfg := &Config{
		Env:            getEnv("ENV", "development"),
		Port:           getEnv("APP_PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", ""),
		DBName:         getEnv("DB_NAME", "app"),
		DBUser:         getEnv("DB_USER", "botkeep"),
		DBPassword:     getEnv("DB_PASSWORD", ""),
		DBPort:         getEnv("DB_PORT", "8165"),
		DBHost:         getEnv("DB_HOST", "195.32.64.31"),
		DBSSLMode:      getEnv("DB_SSLMODE", "require"),
		DBSSLRootCert:  getEnv("DB_SSLROOTCERT", "botkeep-ca.pem"),
		UserContextKey: getEnv("USER_CONTEXT_KEY", "studio_crm"),
		JWTSecret:      getEnv("JWT_SECRET_KEY", ""),
	}

	if cfg.JWTSecret == "" {
		utils.Logger.Warn("Warning: JWT_SECRET_KEY is not set in environment")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
