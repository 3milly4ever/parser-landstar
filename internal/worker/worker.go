package worker

import (
	"encoding/json"
	"log"
	"time"

	models "github.com/3milly4ever/parser-landstar/internal/model"
	config "github.com/3milly4ever/parser-landstar/pkg"
	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type SQSWorker struct {
	sqsClient *sqs.SQS
	db        *gorm.DB
	queueURL  string
}

func NewSQSWorker(queueURL string, awsRegion string) (*SQSWorker, error) {
	// Initialize SQS client
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))
	sqsClient := sqs.New(sess)

	// Initialize MySQL database connection
	dsn := config.AppConfig.MySQLDSN // Update with your DSN
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return &SQSWorker{
		sqsClient: sqsClient,
		db:        db,
		queueURL:  queueURL,
	}, nil
}

func (worker *SQSWorker) Start() {
	for {
		// Poll SQS for messages
		result, err := worker.sqsClient.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(worker.queueURL),
			MaxNumberOfMessages: aws.Int64(10), // Number of messages to pull
			WaitTimeSeconds:     aws.Int64(20), // Long polling
		})
		if err != nil {
			log.Printf("Error receiving message: %v", err)
			continue
		}

		for _, message := range result.Messages {
			// Process each message
			err := worker.processMessage(message)
			if err != nil {
				log.Printf("Error processing message: %v", err)
				continue
			}

			// Delete message from the queue after processing
			_, err = worker.sqsClient.DeleteMessage(&sqs.DeleteMessageInput{
				QueueUrl:      aws.String(worker.queueURL),
				ReceiptHandle: message.ReceiptHandle,
			})
			if err != nil {
				log.Printf("Error deleting message: %v", err)
			}
		}
	}
}

func (worker *SQSWorker) processMessage(message *sqs.Message) error {
	// Log the raw message
	logrus.WithField("raw_message", *message.Body).Info("Processing SQS message")

	// Parse the message body
	var data map[string]interface{} // Use interface{} to handle mixed types
	err := json.Unmarshal([]byte(*message.Body), &data)
	if err != nil {
		logrus.Error("Error unmarshalling message: ", err)
		return err
	}

	// Log the data for debugging
	logrus.WithField("data", data).Info("Parsed SQS message data")

	// Helper function to parse datetime with multiple formats
	parseDateTime := func(dateStr string) (time.Time, error) {
		formats := []string{
			"2006-01-02 15:04:05",             // Standard MySQL datetime
			"2006-01-02 15:04",                // Without seconds
			time.RFC3339,                      // RFC3339 format
			"2006-01-02 15:04 MST (UTC-0700)", // Format with timezone
		}
		var t time.Time
		var err error
		for _, format := range formats {
			t, err = time.Parse(format, dateStr)
			if err == nil {
				return t, nil
			}
		}
		return t, err
	}

	// Attempt to parse dates with the helper function
	pickupDate, err := parseDateTime(getStringValue(data["pickupDate"]))
	if err != nil {
		logrus.WithField("pickupDate", data["pickupDate"]).Error("Failed to parse pickupDate: ", err)
		return err
	}

	deliveryDate, err := parseDateTime(getStringValue(data["deliveryDate"]))
	if err != nil {
		logrus.WithField("deliveryDate", data["deliveryDate"]).Error("Failed to parse deliveryDate: ", err)
		return err
	}

	// Determine OrderTypeID based on SuggestedTruckSize
	var orderTypeID int
	switch getStringValue(data["suggestedTruckSize"]) {
	case "Small Straight":
		orderTypeID = 1
	case "Large Straight":
		orderTypeID = 3
	default:
		orderTypeID = 2
	}

	// Create and save the Order record to the database
	order := models.Order{
		OrderNumber:        getStringValue(data["orderNumber"]),
		PickupLocation:     getStringValue(data["pickupLocation"]),
		DeliveryLocation:   getStringValue(data["deliveryLocation"]),
		PickupDate:         pickupDate,
		DeliveryDate:       deliveryDate,
		SuggestedTruckSize: getStringValue(data["suggestedTruckSize"]),
		Notes:              getStringValue(data["notes"]),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
		PickupZip:          getStringValue(data["pickupZip"]),
		DeliveryZip:        getStringValue(data["deliveryZip"]),
		OrderTypeID:        orderTypeID,
	}

	if err := worker.db.Create(&order).Error; err != nil {
		logrus.Error("Failed to save order: ", err)
		return err
	}
	logrus.WithField("order_id", order.ID).Info("Order saved to database")

	// Create and save the OrderLocation record to the database
	orderLocation := models.OrderLocation{
		OrderID:             order.ID,
		PickupLabel:         getStringValue(data["pickupLabel"]),
		PickupCountryCode:   getStringValue(data["pickupCountryCode"]),
		PickupCountryName:   getStringValue(data["pickupCountryName"]),
		PickupStateCode:     getStringValue(data["pickupStateCode"]),
		PickupState:         getStringValue(data["pickupState"]),
		PickupCity:          getStringValue(data["pickupCity"]),
		PickupPostalCode:    getStringValue(data["pickupPostalCode"]),
		DeliveryLabel:       getStringValue(data["deliveryLabel"]),
		DeliveryCountryCode: getStringValue(data["deliveryCountryCode"]),
		DeliveryCountryName: getStringValue(data["deliveryCountryName"]),
		DeliveryStateCode:   getStringValue(data["deliveryStateCode"]),
		DeliveryState:       getStringValue(data["deliveryState"]),
		DeliveryCity:        getStringValue(data["deliveryCity"]),
		DeliveryPostalCode:  getStringValue(data["deliveryPostalCode"]),
		EstimatedMiles:      getFloatValue(data["estimatedMiles"]),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	if err := worker.db.Create(&orderLocation).Error; err != nil {
		logrus.Error("Failed to save order location: ", err)
		return err
	}
	logrus.WithField("order_location_id", orderLocation.ID).Info("OrderLocation saved to database")

	// Create and save the OrderItem record to the database
	orderItem := models.OrderItem{
		OrderID:   order.ID,
		Length:    getFloatValue(data["length"]),
		Width:     getFloatValue(data["width"]),
		Height:    getFloatValue(data["height"]),
		Weight:    getFloatValue(data["weight"]),
		Pieces:    getIntValue(data["pieces"]),
		Stackable: getBoolValue(data["stackable"]),
		Hazardous: getBoolValue(data["hazardous"]),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := worker.db.Create(&orderItem).Error; err != nil {
		logrus.Error("Failed to save order item: ", err)
		return err
	}
	logrus.WithField("order_item_id", orderItem.ID).Info("OrderItem saved to database")

	// Create and save the OrderEmail record to the database
	orderEmail := models.OrderEmail{
		ReplyTo:   getStringValue(data["replyTo"]),
		Subject:   getStringValue(data["subject"]),
		MessageID: getStringValue(data["messageID"]),
		OrderID:   order.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := worker.db.Create(&orderEmail).Error; err != nil {
		logrus.Error("Failed to save order email: ", err)
		return err
	}
	logrus.WithField("order_email_id", orderEmail.ID).Info("OrderEmail saved to database")

	logrus.WithFields(logrus.Fields{
		"order_id":    order.ID,
		"orderNumber": order.OrderNumber,
	}).Info("Successfully processed and saved order")
	return nil
}

func getFloatValue(data interface{}) float64 {
	if value, ok := data.(float64); ok {
		return value
	}
	return 0.0
}

func getIntValue(data interface{}) int {
	if value, ok := data.(float64); ok {
		return int(value)
	}
	return 0
}

func getBoolValue(data interface{}) bool {
	if value, ok := data.(bool); ok {
		return value
	}
	return false
}

// Helper functions remain the same...

// Helper functions to handle type conversion
func getStringValue(value interface{}) string {
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}
