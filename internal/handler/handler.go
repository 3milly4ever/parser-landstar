package handler

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"mime/quotedprintable"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"github.com/3milly4ever/parser-landstar/internal/parser"
	"github.com/3milly4ever/parser-landstar/internal/sqs"
	config "github.com/3milly4ever/parser-landstar/pkg"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

var requestsData []map[string]string

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

	// Extract the order number from the plain text body
	orderNumber := parser.ExtractOrderNumber(bodyPlain)
	logrus.Infof("Extracted Order Number: %s", orderNumber)

	// Prepare data to send to SQS
	data := map[string]string{
		"extractedOrder": orderNumber,
		"subject":        subject,
	}

	sqsClient, err := sqs.NewSQSClient(
		config.AppConfig.AWSRegion,
		config.AppConfig.SQSQueueURL,
		config.AppConfig.AWSAccessKeyID,
		config.AppConfig.AWSSecretAccessKey,
	)
	if err != nil {
		logrus.Error("Error initializing SQS client: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to initialize SQS client")
	}

	// Send the JSON message to SQS
	if err := sqsClient.SendMessage(data); err != nil {
		logrus.Error("Error sending message to SQS: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to send message to SQS")
	} else {
		logrus.Infof("Message sent to SQS with data: %+v", data)
	}

	logrus.Info("Successfully sent message to SQS")
	return c.JSON(fiber.Map{
		"message":     "Email received and sent to SQS",
		"subject":     subject,
		"orderNumber": orderNumber,
	})
}

func decodeQuotedPrintable(input string) (string, error) {
	reader := quotedprintable.NewReader(bytes.NewReader([]byte(input)))
	decodedBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		logrus.Warn("Error decoding quoted-printable body-html: ", err)
		return input, err
	}
	return string(decodedBytes), nil
}

func stripHTML(input string) string {
	// Remove HTML tags and return plain text
	re := regexp.MustCompile("<[^>]*>")
	return re.ReplaceAllString(input, "")
}

func SaveToJSONFile(data []map[string]string) error {
	// Define the directory to save the file
	dir := "../../storage/emails"
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	// Define the file name
	fileName := filepath.Join(dir, "email_data.json")

	// Open the file for writing (create if not exists, truncate if exists)
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create a new JSON encoder with indentation (pretty print)
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")  // Pretty print with 2 spaces indentation
	encoder.SetEscapeHTML(false) // Disable HTML escaping

	// Encode the data as JSON and write to the file
	if err := encoder.Encode(data); err != nil {
		return err
	}

	return nil
}
