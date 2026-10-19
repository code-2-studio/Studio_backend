package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"time"

	"studio_backend/models"
	"studio_backend/utils"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB *gorm.DB
)

// --------------------
// DATABASE CONNECTION (PostgreSQL)
// --------------------

func buildDSN() (string, string) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL != "" {
		resolved := resolveDatabaseURL(databaseURL)
		return resolved, "PostgreSQL (DATABASE_URL)"
	}

	dbHost := getEnv("DB_HOST", "195.32.64.31")
	dbPort := getEnv("DB_PORT", "8165")
	dbUser := getEnv("DB_USER", "botkeep")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := getEnv("DB_NAME", "app")
	dbSSLMode := getEnv("DB_SSLMODE", "require")
	dbSSLRootCert := os.Getenv("DB_SSLROOTCERT")

	if dbSSLRootCert != "" {
		if _, err := os.Stat(dbSSLRootCert); os.IsNotExist(err) {
			utils.Logger.Warnw("⚠️ SSL root certificate file not found on disk. Falling back to sslmode=require for encrypted TLS connection.", "rootCert", dbSSLRootCert)
			dbSSLRootCert = ""
			if dbSSLMode == "verify-full" || dbSSLMode == "verify-ca" {
				dbSSLMode = "require"
			}
		}
	}

	var dsn string
	if dbSSLRootCert != "" {
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s sslrootcert=%s",
			dbHost, dbUser, dbPassword, dbName, dbPort, dbSSLMode, dbSSLRootCert)
	} else {
		dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			dbHost, dbUser, dbPassword, dbName, dbPort, dbSSLMode)
	}

	return dsn, fmt.Sprintf("%s:%s/%s", dbHost, dbPort, dbName)
}

func resolveDatabaseURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	q := u.Query()
	rootCert := q.Get("sslrootcert")
	if rootCert != "" {
		if _, err := os.Stat(rootCert); os.IsNotExist(err) {
			utils.Logger.Warnw("⚠️ SSL root certificate file not found on disk. Falling back to sslmode=require for encrypted TLS connection.", "rootCert", rootCert)
			q.Del("sslrootcert")
			q.Set("sslmode", "require")
			u.RawQuery = q.Encode()
			return u.String()
		}
	}

	return rawURL
}

func Connect() {
	dsn, target := buildDSN()

	var err error
	maxRetries := 5
	backoff := 2 * time.Second

	for i := 1; i <= maxRetries; i++ {
		utils.Logger.Infow("Connecting to PostgreSQL database", "attempt", i, "target", target)

		DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})

		if err == nil {
			sqlDB, dbErr := DB.DB()
			if dbErr == nil {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				if pingErr := sqlDB.PingContext(ctx); pingErr == nil {
					setupDB(DB)
					autoMigrate(DB)
					utils.Logger.Infow("✅ PostgreSQL database connected successfully", "target", target)
					startDBHealthCheck(sqlDB)
					return
				} else {
					err = pingErr
				}
			} else {
				err = dbErr
			}
		}

		utils.Logger.Errorw("❌ DB connection failed", "attempt", i, "error", err)
		time.Sleep(backoff)
		backoff *= 2
	}

	utils.Logger.Warnw("⚠️ Warning: Could not connect to PostgreSQL database after retries. The app will continue starting.", "error", err)
}

// --------------------
// AUTO MIGRATION
// --------------------

func autoMigrate(database *gorm.DB) {
	err := database.AutoMigrate(
		&models.Permission{},
		&models.Role{},
		&models.RolePermission{},
		&models.User{},
		&models.UserRole{},
	)
	if err != nil {
		utils.Logger.Errorw("❌ Database auto-migration failed", "error", err)
	} else {
		utils.Logger.Infow("✅ Database schema auto-migrated successfully")
	}
}

// --------------------
// CONNECTION POOLING
// --------------------

func setupDB(database *gorm.DB) {
	sqlDB, err := database.DB()
	if err != nil {
		utils.Logger.Errorw("Failed to get sql.DB", "error", err)
		return
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
	sqlDB.SetConnMaxIdleTime(2 * time.Minute)

	stats := sqlDB.Stats()
	utils.Logger.Infow("DB Pool configured", "MaxOpen", stats.MaxOpenConnections)
}

// --------------------
// DB HEALTH CHECK
// --------------------

func startDBHealthCheck(sqlDB *sql.DB) {
	go func() {
		for {
			time.Sleep(30 * time.Second)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := sqlDB.PingContext(ctx)
			cancel()

			if err != nil {
				utils.Logger.Warnw("⚠️ DB health check ping delayed or failed", "error", err)
			}
		}
	}()
}

// --------------------
// CLOSE DB
// --------------------

func CloseDB() {
	if DB == nil {
		return
	}

	sqlDB, err := DB.DB()
	if err != nil {
		utils.Logger.Errorw("Error retrieving sql.DB", "error", err)
		return
	}

	utils.Logger.Infow("Closing DB connection")
	_ = sqlDB.Close()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

