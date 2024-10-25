package main

import (
	"log"

	"github.com/3milly4ever/parser-landstar/internal/handler"
	config "github.com/3milly4ever/parser-landstar/pkg"
	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	// Load configuration
	config.LoadConfig()

	// Initialize the database
	db, err := handler.InitializeDB()
	if err != nil {
		log.Fatalf("Failed to initialize the database: %v", err)
	}

	// Set the DB for the handler package
	handler.SetDB(db)

	lambda.Start(handler.LambdaHandler)

}
