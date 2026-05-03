package database

import (
	"fmt"
	"log"
	"os"

	"github.com/flux/marketplace/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// Connect initialise la connexion à la base de données
func Connect() {
	var err error
	var dialector gorm.Dialector

	// Choix du driver selon la variable d'environnement
	dbDriver := os.Getenv("DB_DRIVER")
	if dbDriver == "" {
		dbDriver = "sqlite"
	}

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

	default: // sqlite
		dbPath := getEnv("SQLITE_PATH", "./FluxHUB.db")
		dialector = sqlite.Open(dbPath)
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	DB, err = gorm.Open(dialector, gormConfig)
	if err != nil {
		log.Fatalf("Impossible de se connecter à la base de données: %v", err)
	}

	log.Printf("Base de données connectée (%s)", dbDriver)

	// Auto-migration des schémas
	if err := autoMigrate(); err != nil {
		log.Fatalf("Erreur de migration: %v", err)
	}
}

// autoMigrate exécute les migrations automatiques
func autoMigrate() error {
	return DB.AutoMigrate(
		&models.Plugin{},
		&models.Version{},
	)
}

// getEnv retourne la valeur d'une variable d'environnement ou une valeur par défaut
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
