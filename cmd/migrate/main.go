package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Info("No .env file found, proceeding with system environment variables")
	}

	fresh := flag.Bool("fresh", false, "Drop all tables and re-migrate")
	flag.Parse()

	if !*fresh {
		slog.Info("Running normal migration... (Use -fresh to drop all tables first)")
		database.Connect()
		err := database.DB.AutoMigrate(
			&models.User{},
			&models.Plugin{},
			&models.Version{},
			&models.Session{},
		)
		if err != nil {
			slog.Error(fmt.Sprintf("Failed to migrate tables: %v", err))
			os.Exit(1)
		}
		slog.Info("Migration complete!")
		return
	}

	slog.Info("Dropping all tables...")
	database.Connect()

	err := database.DB.Migrator().DropTable(
		&models.User{},
		&models.Plugin{},
		&models.Version{},
		&models.Session{},
	)

	if err != nil {
		slog.Error(fmt.Sprintf("Failed to drop tables: %v", err))
		os.Exit(1)
	}

	slog.Info("Tables dropped successfully.")

	slog.Info("Running fresh migration...")
	err = database.DB.AutoMigrate(
		&models.User{},
		&models.Plugin{},
		&models.Version{},
		&models.Session{},
	)

	if err != nil {
		slog.Error(fmt.Sprintf("Failed to migrate tables: %v", err))
		os.Exit(1)
	}

	slog.Info("Fresh migration complete!")
}
