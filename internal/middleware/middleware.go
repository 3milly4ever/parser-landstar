package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/sirupsen/logrus"
)

var lastRequestTime time.Time
var firstRequest bool = true

func RequestThrottle() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if firstRequest {
			firstRequest = false
			lastRequestTime = time.Now()
			return c.Next()
		}

		// Check if 5 minutes have passed since the last request
		if time.Since(lastRequestTime) < 5*time.Minute {
			logrus.Warn("Request received before the 5-minute wait period")
			return c.Status(fiber.StatusTooManyRequests).SendString("Please wait 5 minutes between requests.")
		}

		// Update the last request time
		lastRequestTime = time.Now()

		// Continue to the next handler
		return c.Next()
	}
}

func CORS() fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins: "*", // Allow all origins, customize as needed
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH",
	})
}
