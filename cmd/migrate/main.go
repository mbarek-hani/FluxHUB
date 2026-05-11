package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/mbarek-hani/FluxHUB/database"
	"github.com/mbarek-hani/FluxHUB/models"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, proceeding with system environment variables")
	}

	fresh := flag.Bool("fresh", false, "Drop all tables and re-migrate")
	flag.Parse()

	if !*fresh {
		fmt.Println("Running normal migration... (Use -fresh to drop all tables first)")
		database.Connect()
		fmt.Println("✅ Migration complete!")
		return
	}

	fmt.Println("⚠️  Dropping all tables...")
	// Connect to the DB to get the GORM instance
	database.Connect() 
	
	// Drop the tables
	err := database.DB.Migrator().DropTable(
		&models.Developer{},
		&models.Plugin{},
		&models.Version{},
		&models.Admin{},
	)

	if err != nil {
		log.Fatalf("Failed to drop tables: %v", err)
	}

	fmt.Println("✅ Tables dropped successfully.")
	
	fmt.Println("🚀 Running fresh migration...")
	// Re-run AutoMigrate
	err = database.DB.AutoMigrate(
		&models.Developer{},
		&models.Plugin{},
		&models.Version{},
		&models.Admin{},
	)

	if err != nil {
		log.Fatalf("Failed to migrate tables: %v", err)
	}

	fmt.Println("✅ Fresh migration complete!")
}
