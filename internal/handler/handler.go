package handler

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	models "github.com/3milly4ever/parser-landstar/internal/model"
	"github.com/3milly4ever/parser-landstar/internal/parser"
	config "github.com/3milly4ever/parser-landstar/pkg"
	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
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

// func MailgunHandler(c *fiber.Ctx) error {
// 	logrus.Info("Mailgun route accessed")

// 	// Parse the form data from the body
// 	formData, err := url.ParseQuery(string(c.Body()))
// 	if err != nil {
// 		logrus.Error("Error parsing form data from request body: ", err)
// 		return c.Status(fiber.StatusBadRequest).SendString("Invalid form data")
// 	}

// 	// Check if the form data is empty
// 	if len(formData) == 0 {
// 		logrus.Warn("No data received from Mailgun")
// 		return c.Status(fiber.StatusBadRequest).SendString("No data received")
// 	}

// 	// Extract specific fields
// 	subject := formData.Get("subject")
// 	bodyHTML := formData.Get("body-html")
// 	bodyPlain := formData.Get("body-plain")
// 	messageID := formData.Get("Message-Id") // Extract the Message-ID field

// 	// Create a new parser_log record with necessary initial values
// 	parserLog := &models.ParserLog{
// 		ParserID:   4,          // Set ParserID to 4 as per your logic
// 		ParserType: "mail",     // Set the parser type
// 		BodyHtml:   bodyHTML,   // Set the extracted HTML body
// 		BodyPlain:  bodyPlain,  // Set the extracted plain text body
// 		CreatedAt:  time.Now(), // Set the current time for creation
// 		UpdatedAt:  time.Now(), // Set the updated time
// 	}

// 	// Save to database
// 	if err := db.Create(parserLog).Error; err != nil {
// 		logrus.Error("Failed to create parser log record: ", err)
// 		// Update parser_log with error_type and error_text
// 		parserLog.ErrorType = "ParseError"
// 		parserLog.ErrorText = err.Error()
// 		parserLog.UpdatedAt = time.Now()
// 		db.Save(parserLog)

// 		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create parser log record")
// 	}

// 	// Check for the "Landstar" keyword in the link "www.LandstarCarriers.com/Loads"
// 	if strings.Contains(bodyHTML, "www.LandstarCarriers.com/Loads") || strings.Contains(bodyPlain, "www.LandstarCarriers.com/Loads") {
// 		// Print out the request details, including bodyHTML, bodyPlain, and other fields
// 		logrus.WithFields(logrus.Fields{
// 			"subject":    subject,
// 			"message_id": messageID,
// 			"body_html":  bodyHTML,
// 			"body_plain": bodyPlain,
// 		}).Info("Keyword 'Landstar' found in the email. Request details printed.")

// 		// Initialize the Landstar parser
// 		landstarParser := &parser.LandstarParser{}

// 		// Parse the email content
// 		parserResult, err := landstarParser.Parse(bodyHTML, bodyPlain)
// 		if err != nil {
// 			logrus.Error("Failed to parse Landstar email: ", err)
// 			return c.Status(fiber.StatusInternalServerError).SendString("Failed to parse email")
// 		}

// 		// **Handle nil parserResult when the truck size is ignored**
// 		if parserResult == nil {
// 			logrus.Warn("ParserResult is nil due to ignored truck size. Deleting parser log and skipping processing.")

// 			// Delete the parserLog record
// 			if err := db.Delete(&parserLog).Error; err != nil {
// 				logrus.Error("Failed to delete parser log record: ", err)
// 				return c.Status(fiber.StatusInternalServerError).SendString("Failed to delete parser log record")
// 			}

// 			return c.SendString("Email ignored due to truck size and parser log deleted")
// 		}

// 		// Extract reply-to from plain text body
// 		var replyTo string
// 		if bodyPlain != "" {
// 			replyTo = parser.ExtractReplyTo(bodyPlain)
// 		}

// 		// Prepare data to be sent to SQS
// 		data := map[string]interface{}{
// 			"orderNumber":         parserResult.Order.OrderNumber,
// 			"pickupLocation":      parserResult.OrderLocation.PickupCity + ", " + parserResult.OrderLocation.PickupState + ", " + parserResult.OrderLocation.PickupCountryName,
// 			"deliveryLocation":    parserResult.OrderLocation.DeliveryCity + ", " + parserResult.OrderLocation.DeliveryState + ", " + parserResult.OrderLocation.DeliveryCountryName,
// 			"pickupDate":          parserResult.Order.PickupDate,
// 			"truckTypeID":         parserResult.Order.TruckTypeID,
// 			"deliveryDate":        parserResult.Order.DeliveryDate,
// 			"suggestedTruckSize":  parserResult.Order.SuggestedTruckSize,
// 			"originalTruckSize":   parserResult.Order.OriginalTruckSize,
// 			"notes":               parserResult.Order.Notes,
// 			"pickupZip":           parserResult.PickupZip,
// 			"deliveryZip":         parserResult.DeliveryZip,
// 			"pickupCity":          parserResult.OrderLocation.PickupCity,
// 			"pickupState":         parserResult.OrderLocation.PickupState,
// 			"pickupStateCode":     parserResult.OrderLocation.PickupStateCode,
// 			"pickupCountry":       parserResult.OrderLocation.PickupCountryCode,
// 			"pickupCountryCode":   parserResult.OrderLocation.PickupCountryCode,
// 			"pickupCountryName":   parserResult.OrderLocation.PickupCountryName,
// 			"deliveryCountryName": parserResult.OrderLocation.DeliveryCountryName,
// 			"deliveryCity":        parserResult.OrderLocation.DeliveryCity,
// 			"deliveryState":       parserResult.OrderLocation.DeliveryState,
// 			"deliveryStateCode":   parserResult.OrderLocation.DeliveryStateCode,
// 			"deliveryCountry":     parserResult.OrderLocation.DeliveryCountryCode,
// 			"deliveryCountryCode": parserResult.OrderLocation.DeliveryCountryCode,
// 			"estimatedMiles":      parserResult.Order.EstimatedMiles,
// 			"length":              parserResult.OrderItem.Length,
// 			"width":               parserResult.OrderItem.Width,
// 			"height":              parserResult.OrderItem.Height,
// 			"weight":              parserResult.OrderItem.Weight,
// 			"pieces":              parserResult.OrderItem.Pieces,
// 			"stackable":           parserResult.OrderItem.Stackable,
// 			"hazardous":           parserResult.OrderItem.Hazardous,
// 			"orderTypeID":         5,
// 			"replyTo":             replyTo,
// 			"subject":             subject,
// 			"bodyHTML":            bodyHTML,
// 			"parserLogID":         parserLog.ID, // Include the parser_log ID
// 			"bodyPlain":           bodyPlain,
// 			"messageID":           messageID,
// 			"createdAt":           time.Now(),
// 			"updatedAt":           time.Now(),
// 		}

// 		// Print each field individually, excluding bodyHTML and bodyPlain
// 		logrus.Info("Parsed data from Landstar email:")
// 		for key, value := range data {
// 			if key == "bodyHTML" || key == "bodyPlain" {
// 				continue // Skip printing bodyHTML and bodyPlain
// 			}
// 			logrus.Infof("%s: %v", key, value)
// 		}

// 		// Alternatively, you can marshal the data to JSON and print it
// 		messageBody, err := json.MarshalIndent(data, "", "  ")
// 		if err != nil {
// 			logrus.Error("Error marshaling data to JSON: ", err)
// 			return c.Status(fiber.StatusInternalServerError).SendString("Failed to prepare message")
// 		}

// 		//	logrus.Info("Parsed Data:\n", string(messageBody))
// 		// Send the message to SQS
// 		sqsClient := sqs.New(session.Must(session.NewSession()))
// 		_, err = sqsClient.SendMessage(&sqs.SendMessageInput{
// 			QueueUrl:    aws.String(config.AppConfig.SQSQueueURL),
// 			MessageBody: aws.String(string(messageBody)),
// 		})
// 		if err != nil {
// 			logrus.Error("Failed to send message to SQS: ", err)
// 			return c.Status(fiber.StatusInternalServerError).SendString("Failed to send message")
// 		}
// 		logrus.Info("Landstar email data parsed and sent to SQS successfully")
// 		// Return a response indicating that the data has been printed
// 		return c.SendString("Landstar email data parsed, printed and sent to SQS successfully")
// 	}

// 	// Log the received data for debugging
// 	logrus.WithFields(logrus.Fields{
// 		"subject":    subject,
// 		"message_id": messageID,
// 		"body_plain": bodyPlain,
// 		"body_html":  bodyHTML,
// 	}).Info("Received email data")

// 	// Extract reply-to from plain text body
// 	var replyTo string
// 	if bodyPlain != "" {
// 		logrus.Info("Parsing plain text body for 'replyTo'")
// 		replyTo = parser.ExtractReplyTo(bodyPlain) // Extract ReplyTo from plain text body
// 		logrus.WithField("replyTo", replyTo).Info("Extracted 'replyTo' from plain text body")
// 	}

// 	// Fallback to form 'reply-to' if not found in plain text
// 	if replyTo == "" {
// 		logrus.Warn("No 'replyTo' field found in the plain text body, falling back to form data")
// 		replyTo = formData.Get("reply-to")
// 		logrus.WithField("replyTo", replyTo).Info("Extracted 'replyTo' from form data")
// 	}

// 	// Check if HTML body exists and parse it if available
// 	var (
// 		orderNumber                                                                  string
// 		pickupZip, pickupCity, pickupState, pickupCountry, pickupStateCode           string
// 		deliveryZip, deliveryCity, deliveryState, deliveryCountry, deliveryStateCode string
// 		pickupCountryCode, deliveryCountryCode                                       string
// 		pickupDateTime, deliveryDateTime                                             time.Time
// 		truckSize, notes                                                             string
// 		originalTruckSize                                                            string
// 		length, width, height, weight                                                float64
// 		pieces                                                                       int
// 		stackable, hazardous                                                         bool
// 		estimatedMiles                                                               int
// 		truckTypeID                                                                  int
// 	)

// 	layout := "2006-01-02 15:04:05" // Layout for MySQL datetime format

// 	// First try extracting from HTML body
// 	var htmlParsed bool
// 	if bodyHTML != "" {
// 		logrus.Info("Parsing HTML body")
// 		doc, err := goquery.NewDocumentFromReader(strings.NewReader(bodyHTML))
// 		if err != nil {
// 			logrus.Error("Error parsing HTML: ", err)
// 		} else {
// 			orderNumber = parser.ExtractOrderNumberFromHTML(doc)
// 			pickupZip, pickupCity, pickupState, pickupStateCode, pickupCountry = parser.ExtractLocationFromHTML(doc, "Pick Up")
// 			deliveryZip, deliveryCity, deliveryState, deliveryStateCode, deliveryCountry = parser.ExtractLocationFromHTML(doc, "Delivery")
// 			pickupDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeStringFromHTML(doc, "Pick Up")))
// 			deliveryDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeStringFromHTML(doc, "Delivery")))
// 			truckSize = parser.ExtractTruckSizeFromHTML(doc)
// 			notes = parser.ExtractNotesFromHTML(doc)
// 			estimatedMiles = parser.ExtractDistanceFromHTML(doc)
// 			originalTruckSize = parser.ExtractTruckClassFromHTML(doc)
// 			// **Use the ExtractOrderItemsFromHTML function to extract order items**
// 			length, width, height, weight, pieces, stackable, hazardous = parser.ExtractOrderItemsFromHTML(doc)

// 			// Extract additional fields
// 			pickupCountryCode = "US"
// 			deliveryCountryCode = "US"

// 			// Check if extraction was successful
// 			htmlParsed = pickupCity != "" && deliveryCity != ""
// 		}
// 	}

// 	// Fallback to plain text extraction if HTML parsing failed or key fields are missing
// 	if !htmlParsed && bodyPlain != "" {
// 		logrus.Warn("HTML parsing failed or incomplete, falling back to plain text body")

// 		// Extract data using plain text parsing functions
// 		orderNumber = parser.ExtractOrderNumber(bodyPlain)
// 		pickupZip, pickupCity, pickupState, pickupCountry = parser.ExtractLocation(bodyPlain, "Pick Up")
// 		deliveryZip, deliveryCity, deliveryState, deliveryCountry = parser.ExtractLocation(bodyPlain, "Delivery")
// 		pickupDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeString(bodyPlain, "Pick Up")))
// 		deliveryDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeString(bodyPlain, "Delivery")))
// 		truckSize = parser.ExtractTruckSize(bodyPlain)
// 		notes = parser.ExtractNotes(bodyPlain)
// 		length, width, height, weight, pieces, stackable, hazardous = parser.ExtractOrderItems(bodyPlain)
// 		estimatedMiles = parser.ExtractDistance(bodyPlain) // Extract the distance in miles

// 		// Extract additional fields
// 		pickupCountryCode = "US"
// 		deliveryCountryCode = "US"
// 	}

// 	// Define truck size mappings (case-insensitive)
// 	truckSizeMap := map[string]int{
// 		"small straight":  1,
// 		"large straight":  2,
// 		"sprinter":        3,
// 		"tractor trailer": 4,
// 	}

// 	// Convert truck size to lowercase and match it with the corresponding TruckTypeID
// 	lowerTruckSize := strings.ToLower(truckSize)
// 	if id, exists := truckSizeMap[lowerTruckSize]; exists {
// 		truckTypeID = id
// 	} else {
// 		truckTypeID = 4 // Default if no match found
// 	}

// 	// Prepare data to be sent to SQS
// 	data := map[string]interface{}{
// 		"orderNumber":         orderNumber,
// 		"pickupLocation":      pickupZip + ", " + pickupCity + ", " + pickupState + ", " + pickupCountry,
// 		"deliveryLocation":    deliveryZip + ", " + deliveryCity + ", " + deliveryState + ", " + deliveryCountry,
// 		"pickupDate":          pickupDateTime,
// 		"deliveryDate":        deliveryDateTime,
// 		"suggestedTruckSize":  truckSize,
// 		"truckTypeID":         truckTypeID, // Include the TruckTypeID
// 		"originalTruckSize":   originalTruckSize,
// 		"notes":               notes,
// 		"pickupZip":           pickupZip,
// 		"deliveryZip":         deliveryZip,
// 		"pickupCity":          pickupCity,
// 		"pickupState":         pickupState,
// 		"pickupStateCode":     pickupStateCode,
// 		"pickupCountry":       pickupCountry,
// 		"pickupCountryCode":   pickupCountryCode, // Include pickup country code
// 		"pickupCountryName":   pickupCountry,
// 		"deliveryCountryName": deliveryCountry,
// 		"deliveryCity":        deliveryCity,
// 		"deliveryState":       deliveryState,
// 		"deliveryStateCode":   deliveryStateCode,
// 		"deliveryCountry":     deliveryCountry,
// 		"deliveryCountryCode": deliveryCountryCode, // Include delivery country code
// 		"estimatedMiles":      estimatedMiles,
// 		"orderTypeID":         4,
// 		"length":              length,
// 		"width":               width,
// 		"height":              height,
// 		"weight":              weight,
// 		"pieces":              pieces,
// 		"stackable":           stackable,
// 		"hazardous":           hazardous,
// 		"replyTo":             replyTo,
// 		"subject":             subject,
// 		"bodyHTML":            bodyHTML,  // Include the HTML body
// 		"bodyPlain":           bodyPlain, // Include the plain text body
// 		"messageID":           messageID,
// 		"parserLogID":         parserLog.ID, // Include the parser_log ID
// 		"createdAt":           time.Now(),
// 		"updatedAt":           time.Now(),
// 	}

// 	// Marshal the data to JSON
// 	messageBody, err := json.Marshal(data)
// 	if err != nil {
// 		logrus.Error("Error marshaling data to JSON: ", err)
// 		return c.Status(fiber.StatusInternalServerError).SendString("Failed to prepare message")
// 	}

// 	// Send the message to SQS
// 	sqsClient := sqs.New(session.Must(session.NewSession()))
// 	_, err = sqsClient.SendMessage(&sqs.SendMessageInput{
// 		QueueUrl:    aws.String(config.AppConfig.SQSQueueURL),
// 		MessageBody: aws.String(string(messageBody)),
// 	})
// 	if err != nil {
// 		logrus.Error("Failed to send message to SQS: ", err)
// 		return c.Status(fiber.StatusInternalServerError).SendString("Failed to send message")
// 	}

// 	logrus.Info("Message successfully sent to SQS")
// 	return c.SendString("Email data parsed and sent to SQS successfully")
// }

func LambdaHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logrus.Info("Mailgun route accessed")

	formData, err := url.ParseQuery(request.Body)
	if err != nil {
		logrus.Error("Error parsing form data from request body: ", err)
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "Invalid form data"}, nil
	}

	if len(formData) == 0 {
		logrus.Warn("No data received from Mailgun")
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "No data received"}, nil
	}

	subject := formData.Get("subject")
	bodyHTML := formData.Get("body-html")
	bodyPlain := formData.Get("body-plain")
	messageID := formData.Get("Message-Id")

	parserLog := &models.ParserLog{
		ParserID:   4,
		ParserType: "mail",
		BodyHtml:   bodyHTML,
		BodyPlain:  bodyPlain,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := db.Create(parserLog).Error; err != nil {
		logrus.Error("Failed to create parser log record: ", err)
		parserLog.ErrorType = "ParseError"
		parserLog.ErrorText = err.Error()
		parserLog.UpdatedAt = time.Now()
		db.Save(parserLog)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to create parser log record"}, nil
	}

	if strings.Contains(bodyHTML, "www.LandstarCarriers.com/Loads") || strings.Contains(bodyPlain, "www.LandstarCarriers.com/Loads") {
		logrus.WithFields(logrus.Fields{
			"subject":    subject,
			"message_id": messageID,
			"body_html":  bodyHTML,
			"body_plain": bodyPlain,
		}).Info("Keyword 'Landstar' found in the email. Request details printed.")

		landstarParser := &parser.LandstarParser{}
		parserResult, err := landstarParser.Parse(bodyHTML, bodyPlain)
		if err != nil {
			logrus.Error("Failed to parse Landstar email: ", err)
			return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to parse email"}, nil
		}

		if parserResult == nil {
			logrus.Warn("ParserResult is nil due to ignored truck size. Deleting parser log and skipping processing.")
			if err := db.Delete(&parserLog).Error; err != nil {
				logrus.Error("Failed to delete parser log record: ", err)
				return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to delete parser log record"}, nil
			}
			return events.APIGatewayProxyResponse{StatusCode: 200, Body: "Email ignored due to truck size and parser log deleted"}, nil
		}

		var replyTo string
		if bodyPlain != "" {
			replyTo = parser.ExtractReplyTo(bodyPlain)
		}

		data := map[string]interface{}{
			"orderNumber":         parserResult.Order.OrderNumber,
			"pickupLocation":      parserResult.OrderLocation.PickupCity + ", " + parserResult.OrderLocation.PickupState + ", " + parserResult.OrderLocation.PickupCountryName,
			"deliveryLocation":    parserResult.OrderLocation.DeliveryCity + ", " + parserResult.OrderLocation.DeliveryState + ", " + parserResult.OrderLocation.DeliveryCountryName,
			"pickupDate":          parserResult.Order.PickupDate,
			"truckTypeID":         parserResult.Order.TruckTypeID,
			"deliveryDate":        parserResult.Order.DeliveryDate,
			"suggestedTruckSize":  parserResult.Order.SuggestedTruckSize,
			"originalTruckSize":   parserResult.Order.OriginalTruckSize,
			"notes":               parserResult.Order.Notes,
			"pickupZip":           parserResult.PickupZip,
			"deliveryZip":         parserResult.DeliveryZip,
			"pickupCity":          parserResult.OrderLocation.PickupCity,
			"pickupState":         parserResult.OrderLocation.PickupState,
			"pickupStateCode":     parserResult.OrderLocation.PickupStateCode,
			"pickupCountry":       parserResult.OrderLocation.PickupCountryCode,
			"pickupCountryCode":   parserResult.OrderLocation.PickupCountryCode,
			"pickupCountryName":   parserResult.OrderLocation.PickupCountryName,
			"deliveryCountryName": parserResult.OrderLocation.DeliveryCountryName,
			"deliveryCity":        parserResult.OrderLocation.DeliveryCity,
			"deliveryState":       parserResult.OrderLocation.DeliveryState,
			"deliveryStateCode":   parserResult.OrderLocation.DeliveryStateCode,
			"deliveryCountry":     parserResult.OrderLocation.DeliveryCountryCode,
			"deliveryCountryCode": parserResult.OrderLocation.DeliveryCountryCode,
			"estimatedMiles":      parserResult.Order.EstimatedMiles,
			"length":              parserResult.OrderItem.Length,
			"width":               parserResult.OrderItem.Width,
			"height":              parserResult.OrderItem.Height,
			"weight":              parserResult.OrderItem.Weight,
			"pieces":              parserResult.OrderItem.Pieces,
			"stackable":           parserResult.OrderItem.Stackable,
			"hazardous":           parserResult.OrderItem.Hazardous,
			"orderTypeID":         5,
			"replyTo":             replyTo,
			"subject":             subject,
			"bodyHTML":            bodyHTML,
			"parserLogID":         parserLog.ID,
			"bodyPlain":           bodyPlain,
			"messageID":           messageID,
			"createdAt":           time.Now(),
			"updatedAt":           time.Now(),
		}

		logrus.Info("Parsed data from Landstar email:")
		for key, value := range data {
			if key == "bodyHTML" || key == "bodyPlain" {
				continue
			}
			logrus.Infof("%s: %v", key, value)
		}

		messageBody, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			logrus.Error("Error marshaling data to JSON: ", err)
			return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to prepare message"}, nil
		}

		sess := session.Must(session.NewSession())
		sqsClient := sqs.New(sess)
		_, err = sqsClient.SendMessage(&sqs.SendMessageInput{
			QueueUrl:    aws.String(config.AppConfig.SQSQueueURL),
			MessageBody: aws.String(string(messageBody)),
		})
		if err != nil {
			logrus.Error("Failed to send message to SQS: ", err)
			return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to send message"}, nil
		}
		logrus.Info("Landstar email data parsed and sent to SQS successfully")
		return events.APIGatewayProxyResponse{StatusCode: 200, Body: "Landstar email data parsed, printed and sent to SQS successfully"}, nil
	}

	logrus.WithFields(logrus.Fields{
		"subject":    subject,
		"message_id": messageID,
		"body_plain": bodyPlain,
		"body_html":  bodyHTML,
	}).Info("Received email data")

	var replyTo string
	if bodyPlain != "" {
		logrus.Info("Parsing plain text body for 'replyTo'")
		replyTo = parser.ExtractReplyTo(bodyPlain)
		logrus.WithField("replyTo", replyTo).Info("Extracted 'replyTo' from plain text body")
	}

	if replyTo == "" {
		logrus.Warn("No 'replyTo' field found in the plain text body, falling back to form data")
		replyTo = formData.Get("reply-to")
		logrus.WithField("replyTo", replyTo).Info("Extracted 'replyTo' from form data")
	}

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

	layout := "2006-01-02 15:04:05"

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
			length, width, height, weight, pieces, stackable, hazardous = parser.ExtractOrderItemsFromHTML(doc)
			pickupCountryCode = "US"
			deliveryCountryCode = "US"
			htmlParsed = pickupCity != "" && deliveryCity != ""
		}
	}

	if !htmlParsed && bodyPlain != "" {
		logrus.Warn("HTML parsing failed or incomplete, falling back to plain text body")
		orderNumber = parser.ExtractOrderNumber(bodyPlain)
		pickupZip, pickupCity, pickupState, pickupCountry = parser.ExtractLocation(bodyPlain, "Pick Up")
		deliveryZip, deliveryCity, deliveryState, deliveryCountry = parser.ExtractLocation(bodyPlain, "Delivery")
		pickupDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeString(bodyPlain, "Pick Up")))
		deliveryDateTime, _ = time.Parse(layout, parser.FormatDateTimeString(parser.ExtractDateTimeString(bodyPlain, "Delivery")))
		truckSize = parser.ExtractTruckSize(bodyPlain)
		notes = parser.ExtractNotes(bodyPlain)
		length, width, height, weight, pieces, stackable, hazardous = parser.ExtractOrderItems(bodyPlain)
		estimatedMiles = parser.ExtractDistance(bodyPlain)
		pickupCountryCode = "US"
		deliveryCountryCode = "US"
	}

	truckSizeMap := map[string]int{
		"small straight":  1,
		"large straight":  2,
		"sprinter":        3,
		"tractor trailer": 4,
	}

	lowerTruckSize := strings.ToLower(truckSize)
	if id, exists := truckSizeMap[lowerTruckSize]; exists {
		truckTypeID = id
	} else {
		truckTypeID = 4
	}

	data := map[string]interface{}{
		"orderNumber":         orderNumber,
		"pickupLocation":      pickupZip + ", " + pickupCity + ", " + pickupState + ", " + pickupCountry,
		"deliveryLocation":    deliveryZip + ", " + deliveryCity + ", " + deliveryState + ", " + deliveryCountry,
		"pickupDate":          pickupDateTime,
		"deliveryDate":        deliveryDateTime,
		"suggestedTruckSize":  truckSize,
		"truckTypeID":         truckTypeID,
		"originalTruckSize":   originalTruckSize,
		"notes":               notes,
		"pickupZip":           pickupZip,
		"deliveryZip":         deliveryZip,
		"pickupCity":          pickupCity,
		"pickupState":         pickupState,
		"pickupStateCode":     pickupStateCode,
		"pickupCountry":       pickupCountry,
		"pickupCountryCode":   pickupCountryCode,
		"pickupCountryName":   pickupCountry,
		"deliveryCountryName": deliveryCountry,
		"deliveryCity":        deliveryCity,
		"deliveryState":       deliveryState,
		"deliveryStateCode":   deliveryStateCode,
		"deliveryCountry":     deliveryCountry,
		"deliveryCountryCode": deliveryCountryCode,
		"estimatedMiles":      estimatedMiles,
		"orderTypeID":         4,
		"length":              length,
		"width":               width,
		"height":              height,
		"weight":              weight,
		"pieces":              pieces,
		"stackable":           stackable,
		"hazardous":           hazardous,
		"replyTo":             replyTo,
		"subject":             subject,
		"bodyHTML":            bodyHTML,
		"bodyPlain":           bodyPlain,
		"messageID":           messageID,
		"parserLogID":         parserLog.ID,
		"createdAt":           time.Now(),
		"updatedAt":           time.Now(),
	}

	messageBody, err := json.Marshal(data)
	if err != nil {
		logrus.Error("Error marshaling data to JSON: ", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to prepare message"}, nil
	}

	sess := session.Must(session.NewSession())
	sqsClient := sqs.New(sess)
	_, err = sqsClient.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(config.AppConfig.SQSQueueURL),
		MessageBody: aws.String(string(messageBody)),
	})
	if err != nil {
		logrus.Error("Failed to send message to SQS: ", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to send message"}, nil
	}

	logrus.Info("Message successfully sent to SQS")
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: "Email data parsed and sent to SQS successfully"}, nil
}
