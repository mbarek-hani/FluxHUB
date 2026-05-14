package database

import (
	"fmt"
	"log/slog"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect() {
	var err error
	var dialector gorm.Dialector

	dbDriver := getEnv("DB_DRIVER", "sqlite")

	switch dbDriver {
	case "postgres":
		dsn := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
			getEnv("DB_HOST", "localhost"),
			getEnv("DB_USER", "postgres"),
			getEnv("DB_PASSWORD", ""),
			getEnv("DB_NAME", "flux_hub"),
			getEnv("DB_PORT", "5432"),
		)
		dialector = postgres.Open(dsn)
	default:
		dbPath := getEnv("SQLITE_PATH", "./fluxHUB.db")
		dialector = sqlite.Open(dbPath)
	}

	DB, err = gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		slog.Error("DB connection failed", "error", err)
		os.Exit(1)
	}

	slog.Info("DB connected", "driver", dbDriver)
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
