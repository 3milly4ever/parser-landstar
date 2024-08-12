package handler

import (
	"bytes"
	"html"
	"io/ioutil"
	"mime/quotedprintable"
	"net/url"
	"regexp"
	"strings"

	"github.com/3milly4ever/parser-landstar/internal/parser"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

func MailgunHandler(c *fiber.Ctx) error {
	logrus.Info("Mailgun route accessed")

	// Parse the form data from the body
	formData, err := url.ParseQuery(string(c.Body()))
	if err != nil {
		logrus.Error("Error parsing form data from request body: ", err)
		return c.Status(fiber.StatusBadRequest).SendString("Invalid form data")
	}

	// Log the form data for inspection
	logrus.Info("Parsed Form Data: ", formData)

	// Extract specific fields
	subject := formData.Get("subject")
	bodyPlain := formData.Get("body-plain")
	bodyHTML := formData.Get("body-html")

	// Decode quoted-printable content (if applicable)
	decodedBodyHTML, err := decodeQuotedPrintable(bodyHTML)
	if err != nil {
		logrus.Error("Error decoding quoted-printable body-html: ", err)
	}

	// Convert the HTML body to plain text
	plainTextFromHTML := stripHTML(decodedBodyHTML)

	// Replace \n with actual newlines in the plain text body
	formattedBodyPlain := strings.ReplaceAll(bodyPlain, "\\n", "\n")

	// Extract the order number from the plain text body
	orderNumber := parser.ExtractOrderNumber(formattedBodyPlain)
	logrus.Infof("Extracted Order Number: %s", orderNumber)

	// Format the plain text body for better readability
	formattedBodyPlain = parser.FormatEmailBody(formattedBodyPlain)
	logrus.Infof("Formatted Body (plain text): \n%s", formattedBodyPlain)

	// Log the cleaned-up plain text body derived from HTML
	logrus.Infof("Cleaned Body (from HTML): \n%s", plainTextFromHTML)

	return c.JSON(fiber.Map{
		"message":      "Email received and parsed",
		"subject":      subject,
		"bodyPlain":    formattedBodyPlain, // Send the formatted plain text body
		"bodyHTML":     plainTextFromHTML,  // Cleaned plain text from HTML
		"orderNumber":  orderNumber,        // Include the extracted order number
		"formData":     formData,           // This is optional, just to see all data
		"originalHTML": decodedBodyHTML,    // Optional, to compare original and cleaned
	})
}

// Helper function to decode quoted-printable strings
func decodeQuotedPrintable(input string) (string, error) {
	reader := quotedprintable.NewReader(bytes.NewReader([]byte(input)))
	decodedBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(decodedBytes), nil
}

// Helper function to strip HTML tags from a string
func stripHTML(input string) string {
	// Decode HTML entities
	decoded := html.UnescapeString(input)

	// Remove all HTML tags
	re := regexp.MustCompile(`<.*?>`)
	cleaned := re.ReplaceAllString(decoded, "")

	// Return the cleaned text
	return cleaned
}
