package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/3milly4ever/parser-landstar/internal/metrics"
	models "github.com/3milly4ever/parser-landstar/internal/model"
	config "github.com/3milly4ever/parser-landstar/pkg"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const maxWorkers = 10

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
	// Create a channel to receive messages
	messageChan := make(chan *sqs.Message, 10)

	// Create separate wait groups
	var wgWorkers sync.WaitGroup
	var wgPoller sync.WaitGroup

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	// Signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start worker goroutines
	for i := 0; i < maxWorkers; i++ {
		wgWorkers.Add(1)
		go func() {
			defer wgWorkers.Done()
			for {
				select {
				case message, ok := <-messageChan:
					if !ok {
						return // Channel closed, stop the worker
					}
					// Process the message
					if err := worker.processMessage(message); err != nil {
						log.Printf("Error processing message: %v", err)
					}
					// Delete the message from SQS
					_, err := worker.sqsClient.DeleteMessage(&sqs.DeleteMessageInput{
						QueueUrl:      aws.String(worker.queueURL),
						ReceiptHandle: message.ReceiptHandle,
					})
					if err != nil {
						log.Printf("Error deleting message: %v", err)
					}
					metrics.MessagesDeleted.Inc()
				case <-ctx.Done():
					return // Stop the worker if context is canceled
				}
			}
		}()
	}

	// Start the polling goroutine
	wgPoller.Add(1)
	go func() {
		defer wgPoller.Done()
		for {
			// Check if context is done before polling
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Poll SQS for messages
			result, err := worker.sqsClient.ReceiveMessage(&sqs.ReceiveMessageInput{
				QueueUrl:            aws.String(worker.queueURL),
				MaxNumberOfMessages: aws.Int64(10),
				WaitTimeSeconds:     aws.Int64(20),
			})
			if err != nil {
				log.Printf("Error receiving message: %v", err)
				continue
			}

			// Send messages to the channel
			for _, message := range result.Messages {
				select {
				case messageChan <- message:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	// Wait for a shutdown signal
	<-sigChan
	log.Println("Shutting down...")
	cancel() // Signal cancellation

	// Wait for the polling goroutine to finish
	wgPoller.Wait()

	// Close the message channel after polling goroutine has stopped
	close(messageChan)

	// Wait for all worker goroutines to finish
	wgWorkers.Wait()
	log.Println("Workers stopped")
}

func ExtractCoordinatesAndCounty(geocodingData map[string]interface{}) (float64, float64, string, error) {
	// Default values in case fields are missing
	var lat, lng float64
	var county string

	// Ensure that "features" exist and are an array
	if features, ok := geocodingData["features"].([]interface{}); ok && len(features) > 0 {
		// Process the first feature (best match)
		firstFeature := features[0].(map[string]interface{})

		// Extract coordinates from geometry
		if geometry, ok := firstFeature["geometry"].(map[string]interface{}); ok {
			if coordinates, ok := geometry["coordinates"].([]interface{}); ok && len(coordinates) == 2 {
				lng = coordinates[0].(float64) // Longitude
				lat = coordinates[1].(float64) // Latitude
			}
		}

		// Extract county from properties
		if properties, ok := firstFeature["properties"].(map[string]interface{}); ok {
			if countyValue, ok := properties["county"].(string); ok {
				county = countyValue
			} else {
				logrus.Warn("County not found in properties")
			}
		}
	} else {
		return 0, 0, "", errors.New("no features found in geocoding data")
	}

	return lat, lng, county, nil
}

func GeocodeLocation(address string) (float64, float64, string, error) {
	// Prepare the base URL and query parameters
	baseURL := "http://207.244.250.222:4000/v1/search"
	params := url.Values{}
	params.Add("text", address)

	// Construct the full URL
	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// Make the HTTP request
	resp, err := http.Get(fullURL)
	if err != nil {
		logrus.WithField("url", fullURL).Error("Failed to make geocoding request: ", err)
		return 0, 0, "", fmt.Errorf("failed to make geocoding request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.WithField("url", fullURL).Error("Failed to read geocoding response body: ", err)
		return 0, 0, "", fmt.Errorf("failed to read geocoding response body: %w", err)
	}

	// Unmarshal the response into a generic interface
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"url":  fullURL,
			"body": string(body),
		}).Error("Failed to unmarshal geocoding response: ", err)
		return 0, 0, "", fmt.Errorf("failed to unmarshal geocoding response: %w", err)
	}

	// Print the full URL in case of error
	logrus.WithField("url", fullURL).Info("Geocoding URL sent for address")

	// Check if there are any features in the response
	features, ok := result["features"].([]interface{})
	if !ok || len(features) == 0 {
		logrus.WithField("url", fullURL).Error("No geocoding features found in the response")
		return 0, 0, "", fmt.Errorf("no geocoding features found in the response")
	}

	// Extract the first feature's coordinates and county
	firstFeature := features[0].(map[string]interface{})

	// Extract geometry safely, checking for nil values
	geometry, ok := firstFeature["geometry"].(map[string]interface{})
	if !ok {
		logrus.Error("Geometry field is missing or of invalid type")
		return 0, 0, "", fmt.Errorf("geometry field is missing")
	}

	coordinates, ok := geometry["coordinates"].([]interface{})
	if !ok || len(coordinates) < 2 {
		logrus.Error("Coordinates field is missing or invalid")
		return 0, 0, "", fmt.Errorf("coordinates field is missing or invalid")
	}

	// Ensure coordinates are valid floats
	lat, latOk := coordinates[1].(float64)
	lng, lngOk := coordinates[0].(float64)
	if !latOk || !lngOk {
		logrus.Error("Coordinates could not be converted to float64")
		return 0, 0, "", fmt.Errorf("invalid coordinates format")
	}

	// Extract county, safely checking for nil
	properties, ok := firstFeature["properties"].(map[string]interface{})
	if !ok {
		logrus.Warn("Properties field is missing in geocoding response")
		return lat, lng, "", nil
	}

	county, ok := properties["county"].(string)
	if !ok {
		logrus.Warn("County field is missing or not a string in geocoding response")
		county = ""
	}

	return lat, lng, county, nil
}

func (worker *SQSWorker) processMessage(message *sqs.Message) error {
	// Log the raw message
	logrus.WithField("raw_message", *message.Body).Info("Processing SQS message")

	// Increment the messagesReceived counter
	metrics.MessagesReceived.Inc()

	// Parse the message body
	var data map[string]interface{}
	err := json.Unmarshal([]byte(*message.Body), &data)
	if err != nil {
		logrus.Error("Error unmarshalling message: ", err)
		metrics.MessagesFailed.Inc() // Increment the failure counter
		return err
	}

	// Extract parserLogID from the message
	parserLogID := getIntValue(data["parserLogID"])

	// Fetch the existing parser_log record
	var parserLog models.ParserLog
	if err := worker.db.First(&parserLog, parserLogID).Error; err != nil {
		logrus.Error("Failed to find parser log record: ", err)
		metrics.MessagesFailed.Inc()
		return err
	}

	// Log the parsed data to identify potential issues
	logrus.WithField("parsed_data", data).Info("Parsed SQS message data")

	// Extract key fields
	pickupCity := getStringValue(data["pickupCity"])
	pickupZip := getStringValue(data["pickupZip"])
	pickupState := getStringValue(data["pickupState"])
	pickupCountryCode := getStringValue(data["pickupCountryCode"])
	deliveryCity := getStringValue(data["deliveryCity"])
	deliveryZip := getStringValue(data["deliveryZip"])
	deliveryState := getStringValue(data["deliveryState"])
	deliveryCountryCode := getStringValue(data["deliveryCountryCode"])
	orderNumber := getStringValue(data["orderNumber"])
	truckTypeID := getIntValue(data["truckTypeID"]) // Ensure this is extracted

	// Log the extracted fields to check if they are empty
	logrus.WithFields(logrus.Fields{
		"pickupCity":          pickupCity,
		"pickupZip":           pickupZip,
		"pickupState":         pickupState,
		"pickupCountryCode":   pickupCountryCode,
		"deliveryCity":        deliveryCity,
		"deliveryZip":         deliveryZip,
		"deliveryState":       deliveryState,
		"deliveryCountryCode": deliveryCountryCode,
		"orderNumber":         orderNumber,
		"truckTypeID":         truckTypeID,
	}).Info("Extracted key fields")

	// Check if key fields are missing or empty
	if pickupCity == "" || deliveryCity == "" || orderNumber == "" {
		logrus.Warn("Missing key fields: pickupCity, deliveryCity, or orderNumber is empty. Skipping message.")
		metrics.MessagesFailed.Inc() // Increment the failure counter
		return nil                   // Skip processing this message
	}

	// Extract the reply-to email from the parsed data
	replyTo := getStringValue(data["replyTo"])
	if replyTo == "" {
		logrus.Warn("No 'replyTo' field found in the message.")
	}

	// Construct addresses for geocoding only if the necessary fields are present
	var pickupAddress, deliveryAddress string
	if pickupZip != "" && pickupCity != "" && pickupState != "" && pickupCountryCode != "" {
		pickupAddress = fmt.Sprintf("%s, %s, %s, %s", pickupZip, pickupCity, pickupState, pickupCountryCode)
	} else {
		logrus.Warn("Missing fields for pickup address. Skipping geocoding for pickup location.")
	}

	if deliveryZip != "" && deliveryCity != "" && deliveryState != "" && deliveryCountryCode != "" {
		deliveryAddress = fmt.Sprintf("%s, %s, %s, %s", deliveryZip, deliveryCity, deliveryState, deliveryCountryCode)
	} else {
		logrus.Warn("Missing fields for delivery address. Skipping geocoding for delivery location.")
	}

	// Fetch geolocation and county information if addresses are not empty
	var pickupLat, pickupLng float64
	var pickupCounty, deliveryCounty string
	var deliveryLat, deliveryLng float64

	if pickupAddress != "" {
		pickupLat, pickupLng, pickupCounty, err = GeocodeLocation(pickupAddress)
		if err != nil {
			logrus.Error("Failed to geocode pickup location: ", err)
			metrics.MessagesFailed.Inc() // Increment the failure counter
			return err
		}
		logrus.WithFields(logrus.Fields{
			"pickupLat":    pickupLat,
			"pickupLng":    pickupLng,
			"pickupCounty": pickupCounty,
		}).Info("Geocoded pickup location")
	}

	if deliveryAddress != "" {
		deliveryLat, deliveryLng, deliveryCounty, err = GeocodeLocation(deliveryAddress)
		if err != nil {
			logrus.Error("Failed to geocode delivery location: ", err)
			metrics.MessagesFailed.Inc() // Increment the failure counter
			return err
		}
		logrus.WithFields(logrus.Fields{
			"deliveryLat":    deliveryLat,
			"deliveryLng":    deliveryLng,
			"deliveryCounty": deliveryCounty,
		}).Info("Geocoded delivery location")
	}

	// Parse dates with a helper function
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

	// Parse pickup and delivery dates
	pickupDate, err := parseDateTime(getStringValue(data["pickupDate"]))
	if err != nil {
		logrus.WithField("pickupDate", data["pickupDate"]).Error("Failed to parse pickupDate: ", err)
		metrics.MessagesFailed.Inc() // Increment the failure counter
		return err
	}

	deliveryDate, err := parseDateTime(getStringValue(data["deliveryDate"]))
	if err != nil {
		logrus.WithField("deliveryDate", data["deliveryDate"]).Error("Failed to parse deliveryDate: ", err)
		metrics.MessagesFailed.Inc() // Increment the failure counter
		return err
	}

	//orderTypeID := 4
	// Create and save the Order record to the database, including TruckTypeID
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
		OrderTypeID:        getIntValue(data["orderTypeID"]),
		TruckTypeID:        truckTypeID, // Ensure TruckTypeID from SQS is used
		OriginalTruckSize:  getStringValue(data["originalTruckSize"]),
		EstimatedMiles:     getIntValue(data["estimatedMiles"]),
	}

	logrus.Infof("Inserting order with TruckTypeID: %d", order.TruckTypeID)

	if err := worker.db.Create(&order).Error; err != nil {
		logrus.Error("Failed to save order: ", err)
		metrics.MessagesFailed.Inc() // Increment the failure counter
		return err
	}
	logrus.WithField("order_id", order.ID).Info("Order saved to database")

	// Create and save the OrderLocation record to the database
	orderLocation := models.OrderLocation{
		// Construct the pickup and delivery labels
		PickupLabel:         fmt.Sprintf("%s, %s, %s, %s", getStringValue(data["pickupZip"]), getStringValue(data["pickupCity"]), getStringValue(data["pickupState"]), getStringValue(data["pickupCountryCode"])),
		DeliveryLabel:       fmt.Sprintf("%s, %s, %s, %s", getStringValue(data["deliveryZip"]), getStringValue(data["deliveryCity"]), getStringValue(data["deliveryState"]), getStringValue(data["deliveryCountryCode"])),
		DeliveryStreet:      getStringValue(data["deliveryStreet"]),
		PickupStreet:        getStringValue(data["pickupStreet"]),
		OrderID:             order.ID,
		PickupCountryCode:   getStringValue(data["pickupCountryCode"]),
		PickupCountryName:   getStringValue(data["pickupCountryName"]),
		PickupStateCode:     getStringValue(data["pickupStateCode"]),
		PickupState:         getStringValue(data["pickupState"]),
		PickupCity:          getStringValue(data["pickupCity"]),
		PickupPostalCode:    getStringValue(data["pickupZip"]),
		PickupLat:           pickupLat,    // Latitude from geocoding
		PickupLng:           pickupLng,    // Longitude from geocoding
		PickupCounty:        pickupCounty, // County from geocoding
		DeliveryCountryCode: getStringValue(data["deliveryCountryCode"]),
		DeliveryCountryName: getStringValue(data["deliveryCountryName"]),
		DeliveryStateCode:   getStringValue(data["deliveryStateCode"]),
		DeliveryState:       getStringValue(data["deliveryState"]),
		DeliveryCity:        getStringValue(data["deliveryCity"]),
		DeliveryPostalCode:  getStringValue(data["deliveryZip"]),
		DeliveryLat:         deliveryLat,    // Latitude from geocoding
		DeliveryLng:         deliveryLng,    // Longitude from geocoding
		DeliveryCounty:      deliveryCounty, // County from geocoding
		EstimatedMiles:      getFloatValue(data["estimatedMiles"]),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	if err := worker.db.Create(&orderLocation).Error; err != nil {
		logrus.Error("Failed to save order location: ", err)
		metrics.MessagesFailed.Inc() // Increment the failure counter
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
		metrics.MessagesFailed.Inc() // Increment the failure counter
		return err
	}
	logrus.WithField("order_item_id", orderItem.ID).Info("OrderItem saved to database")

	// Create and save the OrderEmail record to the database
	orderEmail := models.OrderEmail{
		ReplyTo:   replyTo,
		Subject:   getStringValue(data["subject"]),
		MessageID: getStringValue(data["messageID"]),
		OrderID:   order.ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := worker.db.Create(&orderEmail).Error; err != nil {
		logrus.Error("Failed to save order email: ", err)
		metrics.MessagesFailed.Inc() // Increment the failure counter
		return err
	}
	logrus.WithField("order_email_id", orderEmail.ID).Info("OrderEmail saved to database")

	// Once processing is complete, update the parser_log record
	parserLog.Subject = getStringValue(data["subject"])
	parserLog.BodyHtml = getStringValue(data["bodyHTML"])
	parserLog.BodyPlain = getStringValue(data["bodyPlain"])
	parserLog.OrderID = order.ID
	parserLog.ParserID = 4
	parserLog.UpdatedAt = time.Now()

	// Save the updated parser_log record
	if err := worker.db.Save(&parserLog).Error; err != nil {
		logrus.Error("Failed to update parser log record: ", err)
		metrics.MessagesFailed.Inc()
		return err
	} else {
		logrus.WithField("parser_log_id", parserLog.ID).Info("ParserLog updated in database")
	}

	// // Send order ID to external API
	// orderIDPayload := map[string]int{"order_id": order.ID}
	// payloadBytes, err := json.Marshal(orderIDPayload)
	// if err != nil {
	// 	logrus.Error("Failed to marshal order ID payload: ", err)
	// 	return err
	// }

	// // Send order ID to external API
	req, err := http.NewRequest("GET", fmt.Sprintf("https://platform.hfield.net/api/send_order?order_id=%d", order.ID), nil)
	if err != nil {
		logrus.Error("Failed to create HTTP request: ", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json") // Optional for GET, but can be included if necessary
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logrus.Error("Failed to send order ID to external API: ", err)
		return err
	} else {
		logrus.Info("Successfully sent order ID to external API")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logrus.WithField("status_code", resp.StatusCode).Error("External API call failed")
		return fmt.Errorf("external API call failed with status code: %d", resp.StatusCode)
	}

	// Log additional details about the order processing
	logrus.WithFields(logrus.Fields{
		"order_id":    order.ID,
		"orderNumber": order.OrderNumber,
		"replyTo":     replyTo,
		"subject":     orderEmail.Subject,
	}).Info("Successfully processed and saved order")

	// Increment the messagesParsed counter after successful processing
	metrics.MessagesParsed.Inc()

	logrus.Infof("Order type ID: %v", getIntValue(data["orderTypeID"]))
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

// Helper functions to handle type conversion
func getStringValue(value interface{}) string {
	if v, ok := value.(string); ok {
		return v
	}
	return ""
}
