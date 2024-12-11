package handler

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync"
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
var (
	db        *gorm.DB
	initDB    sync.Once
	sqsClient *sqs.SQS
)

func SetDB(database *gorm.DB) {
	db = database

}

// Add retries incase an initial connection fails
func init() {
	sess := session.Must(session.NewSession())
	sqsClient = sqs.New(sess, aws.NewConfig().WithMaxRetries(3))
}

// Initialize AWS session and SQS client in the init function for reuse across Lambda invocations
// Set connection pooling limits and ensure connection is available throughout the Lambda lifecycle
func InitializeDB() (*gorm.DB, error) {
	var err error
	initDB.Do(func() {
		dsn := config.AppConfig.MySQLDSN
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			logrus.Error("Failed to connect to the database: ", err)
		}

		// Set connection pooling limits
		sqlDB, err := db.DB()
		if err != nil {
			logrus.Fatalf("Failed to get sql.DB from GORM: %v", err)
		}
		sqlDB.SetMaxOpenConns(10)
		sqlDB.SetMaxIdleConns(5)
		sqlDB.SetConnMaxLifetime(time.Minute * 5)
	})

	if db == nil {
		logrus.Error("Database initialization failed. DB is nil.")
		return nil, err
	}
	return db, err
}

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

		messageBodyBytes, err := json.Marshal(data)
		if err != nil {
			logrus.Error("Error marshaling data to JSON: ", err)
			return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to prepare message"}, nil
		}

		_, err = sqsClient.SendMessage(&sqs.SendMessageInput{
			QueueUrl:    aws.String(config.AppConfig.SQSQueueURL),
			MessageBody: aws.String(string(messageBodyBytes)),
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

	messageBodyBytes, err := json.Marshal(data)
	if err != nil {
		logrus.Error("Error marshaling data to JSON: ", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to prepare message"}, nil
	}

	_, err = sqsClient.SendMessage(&sqs.SendMessageInput{
		QueueUrl:    aws.String(config.AppConfig.SQSQueueURL),
		MessageBody: aws.String(string(messageBodyBytes)),
	})
	if err != nil {
		logrus.Error("Failed to send message to SQS: ", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to send message"}, nil
	}

	logrus.Info("Message successfully sent to SQS")
	return events.APIGatewayProxyResponse{StatusCode: 200, Body: "Email data parsed and sent to SQS successfully"}, nil
}
