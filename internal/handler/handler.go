package handler

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/3milly4ever/parser-landstar/internal/parser"
	config "github.com/3milly4ever/parser-landstar/pkg"
	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

//var requestsData []map[string]string

func MailgunHandler(c *fiber.Ctx) error {
	logrus.Info("Mailgun route accessed")

	// Parse the form data from the body
	formData, err := url.ParseQuery(string(c.Body()))
	if err != nil {
		logrus.Error("Error parsing form data from request body: ", err)
		return c.Status(fiber.StatusBadRequest).SendString("Invalid form data")
	}

	// Check if the form data is empty
	if len(formData) == 0 {
		logrus.Warn("No data received from Mailgun")
		return c.Status(fiber.StatusBadRequest).SendString("No data received")
	}

	// Extract specific fields
	subject := formData.Get("subject")
	bodyHTML := formData.Get("body-html")
	bodyPlain := formData.Get("body-plain")
	messageID := formData.Get("Message-Id") // Extract the Message-ID field
	replyTo := formData.Get("reply-to")     // Extract the Reply-To field

	// Log the received data
	logrus.WithFields(logrus.Fields{
		"subject":    subject,
		"message_id": messageID,
		"body_plain": bodyPlain,
	}).Info("Received email data")

	var (
		orderNumber                                               string
		pickupZip, pickupCity, pickupState, pickupCountry         string
		deliveryZip, deliveryCity, deliveryState, deliveryCountry string
		pickupDateTime, deliveryDateTime                          time.Time
		truckSize, notes                                          string
		length, width, height, weight                             float64
		pieces                                                    int
		stackable, hazardous                                      bool
		estimatedMiles                                            float64
	)

	layout := "2006-01-02 15:04:05" // Layout for MySQL datetime format

	// Check if the HTML body exists and parse it
	if bodyHTML != "" {
		logrus.Info("Parsing HTML body")

		// Load HTML body into goquery for parsing
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(bodyHTML))
		if err != nil {
			logrus.Error("Error parsing HTML: ", err)
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to parse HTML")
		}

		// Extract data using HTML parsing functions
		orderNumber = parser.ExtractOrderNumberFromHTML(doc)
		pickupZip, pickupCity, pickupState, pickupCountry = parser.ExtractLocationFromHTML(doc, "Pick Up")
		deliveryZip, deliveryCity, deliveryState, deliveryCountry = parser.ExtractLocationFromHTML(doc, "Delivery")
		pickupDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeStringFromHTML(doc, "Pick Up")))
		deliveryDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeStringFromHTML(doc, "Delivery")))
		truckSize = parser.ExtractTruckSizeFromHTML(doc)
		notes = parser.ExtractNotesFromHTML(doc)
		length, width, height, weight, pieces, stackable, hazardous = parser.ExtractOrderItemsFromHTML(doc)
		estimatedMiles = parser.ExtractDistanceFromHTML(doc) // Extract the distance in miles

	} else if bodyPlain != "" {
		logrus.Warn("No HTML body found, falling back to plain text")

		// Extract data using plain text parsing functions
		orderNumber = parser.ExtractOrderNumber(bodyPlain)
		pickupZip, pickupCity, pickupState, pickupCountry = parser.ExtractLocation(bodyPlain, "Pick Up")
		deliveryZip, deliveryCity, deliveryState, deliveryCountry = parser.ExtractLocation(bodyPlain, "Delivery")
		pickupDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeString(bodyPlain, "Pick Up")))
		deliveryDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeString(bodyPlain, "Delivery")))
		truckSize = parser.ExtractTruckSize(bodyPlain)
		notes = parser.ExtractNotes(bodyPlain)
		length, width, height, weight, pieces, stackable, hazardous = parser.ExtractOrderItems(bodyPlain)
		estimatedMiles = parser.ExtractDistance(bodyPlain) // Extract the distance in miles
	}

	// Prepare data to be sent to SQS
	data := map[string]interface{}{
		"orderNumber":         orderNumber,
		"pickupLocation":      parser.FormatLocationLabel(pickupZip, pickupCity, pickupState, pickupCountry),
		"deliveryLocation":    parser.FormatLocationLabel(deliveryZip, deliveryCity, deliveryState, deliveryCountry),
		"pickupDate":          pickupDateTime,
		"deliveryDate":        deliveryDateTime,
		"suggestedTruckSize":  truckSize,
		"notes":               notes,
		"pickupZip":           pickupZip,
		"deliveryZip":         deliveryZip,
		"pickupLabel":         parser.FormatLocationLabel(pickupZip, pickupCity, pickupState, pickupCountry),
		"pickupCountryCode":   pickupCountry,
		"pickupCountryName":   "United States",
		"pickupStateCode":     pickupState,
		"pickupState":         pickupState,
		"pickupCity":          pickupCity,
		"pickupPostalCode":    pickupZip,
		"deliveryLabel":       parser.FormatLocationLabel(deliveryZip, deliveryCity, deliveryState, deliveryCountry),
		"deliveryCountryCode": deliveryCountry,
		"deliveryCountryName": "United States",
		"deliveryStateCode":   deliveryState,
		"deliveryState":       deliveryState,
		"deliveryCity":        deliveryCity,
		"deliveryPostalCode":  deliveryZip,
		"estimatedMiles":      estimatedMiles,
		"length":              length,
		"width":               width,
		"height":              height,
		"weight":              weight,
		"pieces":              pieces,
		"stackable":           stackable,
		"hazardous":           hazardous,
		"replyTo":             replyTo,
		"subject":             subject,
		"messageID":           messageID,
		"createdAt":           time.Now(),
		"updatedAt":           time.Now(),
	}

	// Marshal the data to JSON
	messageBody, err := json.Marshal(data)
	if err != nil {
		logrus.Error("Error marshaling data to JSON: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to prepare message")
	}

	// Send the message to SQS
	sqsClient := sqs.New(session.Must(session.NewSession()))
	_, err = sqsClient.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(config.AppConfig.SQSQueueURL),
		MessageBody: aws.String(string(messageBody)),
	})
	if err != nil {
		logrus.Error("Failed to send message to SQS: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to send message")
	}

	logrus.Info("Message successfully sent to SQS")
	return c.SendString("Email data parsed and sent to SQS successfully")
}

// func decodeQuotedPrintable(input string) (string, error) {
// 	reader := quotedprintable.NewReader(bytes.NewReader([]byte(input)))
// 	decodedBytes, err := ioutil.ReadAll(reader)
// 	if err != nil {
// 		logrus.Warn("Error decoding quoted-printable body-html: ", err)
// 		return input, err
// 	}
// 	return string(decodedBytes), nil
// }

// func stripHTML(input string) string {
// 	// Remove HTML tags and return plain text
// 	re := regexp.MustCompile("<[^>]*>")
// 	return re.ReplaceAllString(input, "")
// }

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
