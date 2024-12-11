package main

import (
	"log"

	"github.com/3milly4ever/parser-landstar/internal/handler"
	config "github.com/3milly4ever/parser-landstar/pkg"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	// Load configuration (assumes internal error handling within LoadConfig)
	config.LoadConfig()

	// Initialize the database
	db, err := handler.InitializeDB()
	if err != nil {
		log.Fatalf("Failed to initialize the database: %v", err)
	}

	// Properly close the database connection on shutdown
	sqlDB, err := db.DB() // Retrieve the underlying *sql.DB
	if err != nil {
		log.Fatalf("Failed to get underlying database connection: %v", err)
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}()

	// Set the DB for the handler package
	handler.SetDB(db)

	// Start the Lambda handler
	lambda.Start(handler.LambdaHandler)
}
