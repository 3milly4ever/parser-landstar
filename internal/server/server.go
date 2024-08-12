package server

import (
	"github.com/3milly4ever/parser-landstar/internal/log"
	"github.com/3milly4ever/parser-landstar/internal/routes"
	"github.com/gofiber/fiber/v2"
)

func SetupAndRun() {
	// Initialize the logger
	log.InitLogger()

	// Create a new Fiber app
	app := fiber.New()

	// Set up routes
	routes.Setup(app)

	// Start the server on the specified IP and port
	log.Logger.Info("Starting server on 127.0.0.1:54321")
	if err := app.Listen("127.0.0.1:54321"); err != nil {
		log.Logger.Fatal(err)
	}
}
