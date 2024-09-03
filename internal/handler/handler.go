package handler

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	models "github.com/3milly4ever/parser-landstar/internal/model"
	"github.com/3milly4ever/parser-landstar/internal/parser"
	"github.com/PuerkitoBio/goquery"
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

	// Log the received data
	logrus.WithFields(logrus.Fields{
		"subject":    subject,
		"message_id": messageID,
	}).Info("Received email data")

	var (
		orderNumber                                               string
		pickupZip, pickupCity, pickupState, pickupCountry         string
		deliveryZip, deliveryCity, deliveryState, deliveryCountry string
		pickupDateTime, deliveryDateTime                          string
		truckSize, notes                                          string
		length, width, height, weight                             float64
		pieces                                                    int
		stackable, hazardous                                      bool
	)

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
		pickupDateTime = parser.ExtractDateTimeStringFromHTML(doc, "Pick Up")
		deliveryDateTime = parser.ExtractDateTimeStringFromHTML(doc, "Delivery")
		truckSize = parser.ExtractTruckSizeFromHTML(doc)
		notes = parser.ExtractNotesFromHTML(doc)
		length, width, height, weight, pieces, stackable, hazardous = parser.ExtractOrderItemsFromHTML(doc)

	} else if bodyPlain != "" {
		logrus.Warn("No HTML body found, falling back to plain text")

		// Extract data using plain text parsing functions
		orderNumber = parser.ExtractOrderNumber(bodyPlain)
		pickupZip, pickupCity, pickupState, pickupCountry = parser.ExtractLocation(bodyPlain, "Pick Up")
		deliveryZip, deliveryCity, deliveryState, deliveryCountry = parser.ExtractLocation(bodyPlain, "Delivery")
		pickupDateTime = parser.ExtractDateTimeString(bodyPlain, "Pick Up")
		deliveryDateTime = parser.ExtractDateTimeString(bodyPlain, "Delivery")
		truckSize = parser.ExtractTruckSize(bodyPlain)
		notes = parser.ExtractNotes(bodyPlain)
		length, width, height, weight, pieces, stackable, hazardous = parser.ExtractOrderItems(bodyPlain)
	}

	// Create the Order struct
	order := models.Order{
		OrderNumber:        orderNumber,
		PickupLocation:     parser.FormatLocationLabel(pickupZip, pickupCity, pickupState, pickupCountry),
		DeliveryLocation:   parser.FormatLocationLabel(deliveryZip, deliveryCity, deliveryState, deliveryCountry),
		PickupDate:         pickupDateTime,   // Now a string
		DeliveryDate:       deliveryDateTime, // Now a string
		SuggestedTruckSize: truckSize,
		Notes:              notes,
		CreatedAt:          time.Now().Format("2006-01-02 15:04:05"),
		UpdatedAt:          time.Now().Format("2006-01-02 15:04:05"),
		PickupZip:          pickupZip,
		DeliveryZip:        deliveryZip,
	}

	// Create the OrderLocation struct (fields will remain empty if not present in email)
	orderLocation := models.OrderLocation{
		OrderID:             1, // This should be the order ID from the database
		PickupLabel:         parser.FormatLocationLabel(pickupZip, pickupCity, pickupState, pickupCountry),
		PickupCountryCode:   pickupCountry,
		PickupCountryName:   "United States", // Static, or derived based on country code
		PickupStateCode:     pickupState,
		PickupState:         pickupState, // Optional full state name
		PickupCity:          pickupCity,
		PickupPostalCode:    pickupZip,
		DeliveryLabel:       parser.FormatLocationLabel(deliveryZip, deliveryCity, deliveryState, deliveryCountry),
		DeliveryCountryCode: deliveryCountry,
		DeliveryCountryName: "United States", // Static, or derived based on country code
		DeliveryStateCode:   deliveryState,
		DeliveryState:       deliveryState, // Optional full state name
		DeliveryCity:        deliveryCity,
		DeliveryPostalCode:  deliveryZip,
		EstimatedMiles:      435, // Placeholder or parsed from the email
		CreatedAt:           time.Now().Format("2006-01-02 15:04:05"),
		UpdatedAt:           time.Now().Format("2006-01-02 15:04:05"),
	}

	// Create the OrderItem struct
	orderItem := models.OrderItem{
		OrderID:   1, // This should be the order ID from the database
		Length:    length,
		Width:     width,
		Height:    height,
		Weight:    weight,
		Pieces:    pieces,
		Stackable: stackable,
		Hazardous: hazardous,
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Create the OrderEmail struct
	orderEmail := models.OrderEmail{
		ReplyTo:   "jhenry@440transit.com", // Extracted if dynamic
		Subject:   subject,
		MessageID: messageID,
		OrderID:   1, // This should be the order ID from the database
		CreatedAt: time.Now().Format("2006-01-02 15:04:05"),
		UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	// Return the parsed structs as JSON in the response
	return c.JSON(fiber.Map{
		"order":          order,
		"order_location": orderLocation,
		"order_item":     orderItem,
		"order_email":    orderEmail,
	})
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
