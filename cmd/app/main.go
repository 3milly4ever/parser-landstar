package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/3milly4ever/parser-landstar/internal/handler"
	"github.com/3milly4ever/parser-landstar/internal/metrics"
	"github.com/3milly4ever/parser-landstar/internal/server"
	"github.com/3milly4ever/parser-landstar/internal/worker"
	config "github.com/3milly4ever/parser-landstar/pkg"
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

	// Set up and run the Fiber server in a separate goroutine
	go server.SetupAndRun()

	// Initialize the SQS worker
	sqsWorker, err := worker.NewSQSWorker(config.AppConfig.SQSQueueURL, config.AppConfig.AWSRegion)
	if err != nil {
		log.Fatalf("Failed to initialize SQS worker: %v", err)
	}

	// Start the SQS worker in a separate goroutine
	go sqsWorker.Start()

	metrics.InitializePrometheus()

	// Wait for a signal to gracefully shut down the server and worker
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan

	log.Printf("Received signal %s. Shutting down...", sig)
	// Optionally, add any cleanup logic here
}
