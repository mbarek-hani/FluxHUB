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

	database.Connect()

	username := flag.String("username", "", "Admin username")
	password := flag.String("password", "", "Admin password")
	flag.Parse()

	if *username == "" || *password == "" {
		slog.Error("Usage: go run cmd/admin/main.go -username <username> -password <password>")
		return
	}

	admin := models.User{
		Username: *username,
		Role:     models.RoleAdmin,
	}
	if err := admin.SetPassword(*password); err != nil {
		slog.Error(fmt.Sprintf("Error hashing password: %v", err))
		os.Exit(1)
	}

	if err := database.DB.Create(&admin).Error; err != nil {
		slog.Error(fmt.Sprintf("Error creating admin account (might already exist): %v", err))
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("Admin account '%s' created successfully!", *username))
}
