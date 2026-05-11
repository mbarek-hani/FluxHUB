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
	
	// Connect to database
	database.Connect()

	username := flag.String("username", "", "Admin username")
	password := flag.String("password", "", "Admin password")
	flag.Parse()

	if *username == "" || *password == "" {
		fmt.Println("Usage: go run cmd/admin/main.go -username <username> -password <password>")
		return
	}

	admin := models.Admin{
		Username: *username,
	}
	if err := admin.SetPassword(*password); err != nil {
		log.Fatalf("Error hashing password: %v", err)
	}

	if err := database.DB.Create(&admin).Error; err != nil {
		log.Fatalf("Error creating admin account (might already exist): %v", err)
	}

	fmt.Printf("✅ Admin account '%s' created successfully!\n", *username)
}
