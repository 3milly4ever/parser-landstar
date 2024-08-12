package handler

import (
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

func MailgunHandler(c *fiber.Ctx) error {
	logrus.Info("Mailgun route accessed")

	// Decode URL-encoded body
	decodedBody, err := url.QueryUnescape(string(c.Body()))
	if err != nil {
		logrus.Error("Error decoding request body: ", err)
		return c.Status(fiber.StatusBadRequest).SendString("Invalid request body")
	}

	logrus.Info("Decoded Request Body: ", decodedBody)

	return c.SendString("Email received")
}
