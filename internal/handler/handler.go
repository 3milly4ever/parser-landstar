package handler

import (
	"bytes"
	"encoding/json"
	"html"
	"io"
	"mime/quotedprintable"
	"net/url"
	"os"
	"path/filepath"
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

	// Prepare data to save to file including the cleaned body from HTML
	data := map[string]string{
		"extractedOrder": orderNumber,
		"formattedBody":  formattedBodyPlain,
		"cleanedBody":    plainTextFromHTML,
		"subject":        subject,
	}

	// Save data to JSON file
	if err := saveToJSONFile(data); err != nil {
		logrus.Error("Error saving data to JSON file: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to save data")
	}

	return c.JSON(fiber.Map{
		"message":     "Email received, parsed, and saved",
		"subject":     subject,
		"orderNumber": orderNumber,
		"bodyPlain":   formattedBodyPlain, // Send the formatted plain text body
		"cleanedBody": plainTextFromHTML,  // Send the cleaned body
	})
}

// Improved decodeQuotedPrintable function with larger buffer
func decodeQuotedPrintable(input string) (string, error) {
	reader := quotedprintable.NewReader(bytes.NewReader([]byte(input)))
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Helper function to strip HTML tags from a string and handle newlines
func stripHTML(input string) string {
	// Decode HTML entities
	decoded := html.UnescapeString(input)

	// Remove all HTML tags
	re := regexp.MustCompile(`<.*?>`)
	cleaned := re.ReplaceAllString(decoded, "")

	// Replace multiple newlines with a single newline for readability
	cleaned = regexp.MustCompile(`\n+`).ReplaceAllString(cleaned, "\n")

	// Trim leading and trailing whitespace
	return strings.TrimSpace(cleaned)
}

func saveToJSONFile(data map[string]string) error {
	// Define the directory to save the file
	dir := "../../storage/emails"
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	// Define the file name (you could use a timestamp or a unique identifier)
	fileName := filepath.Join(dir, "email_data.json")

	// Open the file for writing (create if not exists, truncate if exists)
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Encode the data as JSON and write to the file
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ") // Pretty print with indentation
	if err := encoder.Encode(data); err != nil {
		return err
	}

	return nil
}
