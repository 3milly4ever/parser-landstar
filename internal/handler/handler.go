package handler

import (
	"github.com/3milly4ever/lambda-parser-landstar/internal/log"
	"github.com/gofiber/fiber"
)

func MailgunHandler(c *fiber.Ctx) error {
	log.Logger.Info("Mailgun route accessed")

	// Here you can add the logic to handle incoming emails from Mailgun
	// For now, let's just log the request body and respond with an acknowledgment.

	log.Logger.Info("Request Body: ", c.Body())

	return c.SendString("Email received")
}
