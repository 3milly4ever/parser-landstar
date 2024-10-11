package handler

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	models "github.com/3milly4ever/parser-landstar/internal/model"
	"github.com/3milly4ever/parser-landstar/internal/parser"
	config "github.com/3milly4ever/parser-landstar/pkg"
	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Ensure you have access to the 'db' instance
var db *gorm.DB

func SetDB(database *gorm.DB) {
	db = database
}

// InitializeDB initializes the DB connection for use in the handler
func InitializeDB() (*gorm.DB, error) {
	dsn := config.AppConfig.MySQLDSN
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logrus.Error("Failed to connect to the database: ", err)
		return nil, err
	}
	return database, nil
}

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

	// Check for the "Landstar" keyword in the link "www.LandstarCarriers.com/Loads"
	if strings.Contains(bodyHTML, "www.LandstarCarriers.com/Loads") || strings.Contains(bodyPlain, "www.LandstarCarriers.com/Loads") {
		// Print out the request details, including bodyHTML, bodyPlain, and other fields
		logrus.WithFields(logrus.Fields{
			"subject":    subject,
			"message_id": messageID,
			"body_html":  bodyHTML,
			"body_plain": bodyPlain,
		}).Info("Keyword 'Landstar' found in the email. Request details printed.")

		// Return a response indicating that the request was printed
		return c.SendString("Keyword 'Landstar' found. Request details printed.")
	}

	// Create a new parser_log record with necessary initial values
	parserLog := &models.ParserLog{
		ParserID:   4,          // Set ParserID to 4 as per your logic
		ParserType: "mail",     // Set the parser type
		BodyHtml:   bodyHTML,   // Set the extracted HTML body
		BodyPlain:  bodyPlain,  // Set the extracted plain text body
		CreatedAt:  time.Now(), // Set the current time for creation
		UpdatedAt:  time.Now(), // Set the updated time
	}

	// Save to database
	if err := db.Create(parserLog).Error; err != nil {
		logrus.Error("Failed to create parser log record: ", err)
		// Update parser_log with error_type and error_text
		parserLog.ErrorType = "ParseError"
		parserLog.ErrorText = err.Error()
		parserLog.UpdatedAt = time.Now()
		db.Save(parserLog)

		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create parser log record")
	}

	// Log the received data for debugging
	logrus.WithFields(logrus.Fields{
		"subject":    subject,
		"message_id": messageID,
		"body_plain": bodyPlain,
		"body_html":  bodyHTML,
	}).Info("Received email data")

	// Extract reply-to from plain text body
	var replyTo string
	if bodyPlain != "" {
		logrus.Info("Parsing plain text body for 'replyTo'")
		replyTo = parser.ExtractReplyTo(bodyPlain) // Extract ReplyTo from plain text body
		logrus.WithField("replyTo", replyTo).Info("Extracted 'replyTo' from plain text body")
	}

	// Fallback to form 'reply-to' if not found in plain text
	if replyTo == "" {
		logrus.Warn("No 'replyTo' field found in the plain text body, falling back to form data")
		replyTo = formData.Get("reply-to")
		logrus.WithField("replyTo", replyTo).Info("Extracted 'replyTo' from form data")
	}

	// Check if HTML body exists and parse it if available
	var (
		orderNumber                                                                  string
		pickupZip, pickupCity, pickupState, pickupCountry, pickupStateCode           string
		deliveryZip, deliveryCity, deliveryState, deliveryCountry, deliveryStateCode string
		pickupCountryCode, deliveryCountryCode                                       string
		pickupDateTime, deliveryDateTime                                             time.Time
		truckSize, notes                                                             string
		originalTruckSize                                                            string
		length, width, height, weight                                                float64
		pieces                                                                       int
		stackable, hazardous                                                         bool
		estimatedMiles                                                               int
		truckTypeID                                                                  int
	)

	layout := "2006-01-02 15:04:05" // Layout for MySQL datetime format

	// First try extracting from HTML body
	var htmlParsed bool
	if bodyHTML != "" {
		logrus.Info("Parsing HTML body")
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(bodyHTML))
		if err != nil {
			logrus.Error("Error parsing HTML: ", err)
		} else {
			orderNumber = parser.ExtractOrderNumberFromHTML(doc)
			pickupZip, pickupCity, pickupState, pickupStateCode, pickupCountry = parser.ExtractLocationFromHTML(doc, "Pick Up")
			deliveryZip, deliveryCity, deliveryState, deliveryStateCode, deliveryCountry = parser.ExtractLocationFromHTML(doc, "Delivery")
			pickupDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeStringFromHTML(doc, "Pick Up")))
			deliveryDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeStringFromHTML(doc, "Delivery")))
			truckSize = parser.ExtractTruckSizeFromHTML(doc)
			notes = parser.ExtractNotesFromHTML(doc)
			estimatedMiles = parser.ExtractDistanceFromHTML(doc)
			originalTruckSize = parser.ExtractTruckClassFromHTML(doc)
			// **Use the ExtractOrderItemsFromHTML function to extract order items**
			length, width, height, weight, pieces, stackable, hazardous = parser.ExtractOrderItemsFromHTML(doc)

			// Extract additional fields
			pickupCountryCode = "US"
			deliveryCountryCode = "US"

			// Check if extraction was successful
			htmlParsed = pickupCity != "" && deliveryCity != ""
		}
	}

	// Fallback to plain text extraction if HTML parsing failed or key fields are missing
	if !htmlParsed && bodyPlain != "" {
		logrus.Warn("HTML parsing failed or incomplete, falling back to plain text body")

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

		// Extract additional fields
		pickupCountryCode = "US"
		deliveryCountryCode = "US"
	}

	// Define truck size mappings (case-insensitive)
	truckSizeMap := map[string]int{
		"small straight":  1,
		"large straight":  2,
		"sprinter":        3,
		"tractor trailer": 4,
	}

	// Convert truck size to lowercase and match it with the corresponding TruckTypeID
	lowerTruckSize := strings.ToLower(truckSize)
	if id, exists := truckSizeMap[lowerTruckSize]; exists {
		truckTypeID = id
	} else {
		truckTypeID = 4 // Default if no match found
	}

	// Prepare data to be sent to SQS
	data := map[string]interface{}{
		"orderNumber":         orderNumber,
		"pickupLocation":      pickupZip + ", " + pickupCity + ", " + pickupState + ", " + pickupCountry,
		"deliveryLocation":    deliveryZip + ", " + deliveryCity + ", " + deliveryState + ", " + deliveryCountry,
		"pickupDate":          pickupDateTime,
		"deliveryDate":        deliveryDateTime,
		"suggestedTruckSize":  truckSize,
		"truckTypeID":         truckTypeID, // Include the TruckTypeID
		"originalTruckSize":   originalTruckSize,
		"notes":               notes,
		"pickupZip":           pickupZip,
		"deliveryZip":         deliveryZip,
		"pickupCity":          pickupCity,
		"pickupState":         pickupState,
		"pickupStateCode":     pickupStateCode,
		"pickupCountry":       pickupCountry,
		"pickupCountryCode":   pickupCountryCode, // Include pickup country code
		"pickupCountryName":   pickupCountry,
		"deliveryCountryName": deliveryCountry,
		"deliveryCity":        deliveryCity,
		"deliveryState":       deliveryState,
		"deliveryStateCode":   deliveryStateCode,
		"deliveryCountry":     deliveryCountry,
		"deliveryCountryCode": deliveryCountryCode, // Include delivery country code
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
		"bodyHTML":            bodyHTML,  // Include the HTML body
		"bodyPlain":           bodyPlain, // Include the plain text body
		"messageID":           messageID,
		"parserLogID":         parserLog.ID, // Include the parser_log ID
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
