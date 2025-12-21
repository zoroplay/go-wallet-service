package main

import (
	"log"

	"wallet-service/internal/database"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found in current directory, trying parent")
		if err := godotenv.Load("../.env"); err != nil {
			log.Println("No .env file found, using system environment variables")
		}
	}

	// Initialize Database
	database.Connect()

	// Run Migrations
	log.Println("Running database migrations...")
	database.Migrate()

	log.Println("Migrations completed successfully!")
}
