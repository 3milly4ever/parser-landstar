package routes

import (
	"github.com/3milly4ever/parser-landstar/internal/handler"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

func Setup(app *fiber.App) {
	// Root route
	app.Get("/", func(c *fiber.Ctx) error {
		logrus.Info("Root route accessed")
		return c.SendString("Welcome to the Fiber server!")
	})

	// Mailgun route
	app.Post("/mailgun", handler.MailgunHandler)
}
