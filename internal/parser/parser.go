package parser

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	models "github.com/3milly4ever/parser-landstar/internal/model"
	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

type LandstarParser struct{}

// ParserResult holds the parsed data
type ParserResult struct {
	Order         models.Order
	OrderLocation models.OrderLocation
	OrderItem     models.OrderItem
	OrderEmail    models.OrderEmail
}

// Parse parses the email content and returns a ParserResult
func (p *LandstarParser) Parse(bodyHTML, bodyPlain string) (*ParserResult, error) {
	if bodyHTML == "" {
		return nil, fmt.Errorf("bodyHTML is empty")
	}

	parserResult, err := ExtractDataFromLandstarHTML(bodyHTML)
	if err != nil {
		return nil, err
	}

	return parserResult, nil
}

// ExtractDataFromLandstarHTML extracts data from the Landstar HTML email content
func ExtractDataFromLandstarHTML(bodyHTML string) (*ParserResult, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(bodyHTML))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %v", err)
	}

	// Initialize models
	order := models.Order{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	orderLocation := models.OrderLocation{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	orderItem := models.OrderItem{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Extract OrderNumber
	order.OrderNumber = ExtractOrderNumberFromLandstarHTML(doc)
	logrus.Infof("Extracted Order Number: %s", order.OrderNumber)

	// Extract Trailer Type (SuggestedTruckSize)
	order.SuggestedTruckSize = ExtractTrailerTypeFromLandstarHTML(doc)
	order.OriginalTruckSize = order.SuggestedTruckSize
	logrus.Infof("Extracted Suggested Truck Size: %s", order.SuggestedTruckSize)

	// Extract EstimatedMiles
	order.EstimatedMiles = ExtractMilesFromLandstarHTML(doc)
	orderLocation.EstimatedMiles = float64(order.EstimatedMiles)
	logrus.Infof("Extracted Estimated Miles: %d", order.EstimatedMiles)

	// Extract Origin and PickupDate
	order.PickupLocation = ExtractOriginFromLandstarHTML(doc)
	orderLocation.PickupLabel = order.PickupLocation
	logrus.Infof("Extracted Pickup Location: %s", order.PickupLocation)

	pickupDate, err := ExtractPickupDateFromLandstarHTML(doc)
	if err == nil {
		order.PickupDate = pickupDate
		logrus.Infof("Extracted Pickup Date: %s", order.PickupDate)
	} else {
		logrus.Warnf("Failed to parse Pickup Date: %v", err)
	}

	// Extract Destination and DeliveryDate
	order.DeliveryLocation = ExtractDestinationFromLandstarHTML(doc)
	orderLocation.DeliveryLabel = order.DeliveryLocation
	logrus.Infof("Extracted Delivery Location: %s", order.DeliveryLocation)

	deliveryDate, err := ExtractDeliveryDateFromLandstarHTML(doc)
	if err == nil {
		order.DeliveryDate = deliveryDate
		logrus.Infof("Extracted Delivery Date: %s", order.DeliveryDate)
	} else {
		logrus.Warnf("Failed to parse Delivery Date: %v", err)
	}

	// Extract Notes from Comments
	order.Notes = ExtractNotesFromLandstarHTML(doc)
	logrus.Infof("Extracted Notes: %s", order.Notes)

	// Extract Commodity details (OrderItem)
	length, width, height, weight, hazardous := ExtractCommodityFromLandstarHTML(doc)
	orderItem.Length = length
	orderItem.Width = width
	orderItem.Height = height
	orderItem.Weight = weight
	orderItem.Hazardous = hazardous
	logrus.Infof("Extracted Commodity - Length: %f, Width: %f, Height: %f, Weight: %f, Hazardous: %t", length, width, height, weight, hazardous)

	// Pieces and Stackable are not specified; set default values
	orderItem.Pieces = 1
	orderItem.Stackable = false

	// Extract City, State from Origin and Destination
	originCity, originState := parseCityState(order.PickupLocation)
	orderLocation.PickupCity = originCity
	orderLocation.PickupState = originState

	destCity, destState := parseCityState(order.DeliveryLocation)
	orderLocation.DeliveryCity = destCity
	orderLocation.DeliveryState = destState

	// Set default country codes
	orderLocation.PickupCountryCode = "US"
	orderLocation.DeliveryCountryCode = "US"
	orderLocation.PickupCountryName = "United States"
	orderLocation.DeliveryCountryName = "United States"

	// Create ParserResult
	parserResult := &ParserResult{
		Order:         order,
		OrderLocation: orderLocation,
		OrderItem:     orderItem,
	}

	return parserResult, nil
}

// GetValueAfterLabel extracts the value after a specific label in a <td> element
func GetValueAfterLabel(doc *goquery.Document, label string) string {
	value := ""
	doc.Find("td").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), label) {
			html, _ := s.Html()
			// Remove the label and any HTML tags
			text := strings.ReplaceAll(html, fmt.Sprintf("<label><b>%s</b></label>&nbsp;", label), "")
			text = strings.ReplaceAll(text, fmt.Sprintf("<b>%s</b>", label), "")
			value = strings.TrimSpace(stripHTMLTags(text))
		}
	})
	return value
}

// stripHTMLTags removes HTML tags from a string
func stripHTMLTags(s string) string {
	re := regexp.MustCompile("<.*?>")
	return re.ReplaceAllString(s, "")
}

// ExtractOrderNumberFromLandstarHTML extracts the order number
func ExtractOrderNumberFromLandstarHTML(doc *goquery.Document) string {
	return GetValueAfterLabel(doc, "Load #")
}

// ExtractTrailerTypeFromLandstarHTML extracts the trailer type
func ExtractTrailerTypeFromLandstarHTML(doc *goquery.Document) string {
	return GetValueAfterLabel(doc, "Trailer Type")
}

// ExtractMilesFromLandstarHTML extracts the miles
func ExtractMilesFromLandstarHTML(doc *goquery.Document) int {
	milesStr := GetValueAfterLabel(doc, "Miles")
	milesStr = strings.ReplaceAll(milesStr, ",", "")
	milesStr = strings.TrimSpace(milesStr)
	miles, err := strconv.Atoi(milesStr)
	if err != nil {
		return 0
	}
	return miles
}

// ExtractOriginFromLandstarHTML extracts the origin location
func ExtractOriginFromLandstarHTML(doc *goquery.Document) string {
	return GetValueAfterLabel(doc, "Origin")
}

// ExtractDestinationFromLandstarHTML extracts the destination location
func ExtractDestinationFromLandstarHTML(doc *goquery.Document) string {
	return GetValueAfterLabel(doc, "Destination")
}

// ExtractPickupDateFromLandstarHTML extracts the pickup date
func ExtractPickupDateFromLandstarHTML(doc *goquery.Document) (time.Time, error) {
	pickupText := GetValueAfterLabel(doc, "Pickup")
	return parseDateRange(pickupText)
}

// ExtractDeliveryDateFromLandstarHTML extracts the delivery date
func ExtractDeliveryDateFromLandstarHTML(doc *goquery.Document) (time.Time, error) {
	deliveryText := GetValueAfterLabel(doc, "Delivery")
	return parseDateRange(deliveryText)
}

// ExtractNotesFromLandstarHTML extracts the notes from the comments section
func ExtractNotesFromLandstarHTML(doc *goquery.Document) string {
	notes := ""
	doc.Find("table#comments").Next().Find("td").Each(func(i int, s *goquery.Selection) {
		notes += s.Text()
	})
	notes = strings.TrimSpace(notes)
	if notes == "" {
		// Try to find "Comments" in td
		doc.Find("td").Each(func(i int, s *goquery.Selection) {
			if strings.Contains(s.Text(), "Comments") {
				nextTr := s.Parent().Next()
				notes = nextTr.Find("td").Text()
				notes = strings.TrimSpace(notes)
			}
		})
	}
	return notes
}

// ExtractCommodityFromLandstarHTML extracts commodity details
func ExtractCommodityFromLandstarHTML(doc *goquery.Document) (length, width, height, weight float64, hazardous bool) {
	doc.Find("#commodityDiv table tr").Each(func(i int, s *goquery.Selection) {
		if i == 1 { // Skip the header row
			lengthText := s.Find("td").Eq(2).Text()
			widthText := s.Find("td").Eq(3).Text()
			heightText := s.Find("td").Eq(4).Text()
			weightText := s.Find("td").Eq(5).Text()
			hazmatText := s.Find("td").Eq(6).Text()

			length = parseFeetInches(lengthText)
			width = parseFeetInches(widthText)
			height = parseFeetInches(heightText)
			weight = parseWeight(weightText)
			hazardous = strings.TrimSpace(hazmatText) == "Y"
		}
	})
	return
}

// parseDateRange parses the date and time from a range string
func parseDateRange(dateRange string) (time.Time, error) {
	// Format: 10/11/2024 08:00 - 10/11/2024 15:00
	dateRange = strings.TrimSpace(dateRange)
	parts := strings.Split(dateRange, "-")
	if len(parts) == 0 {
		return time.Time{}, fmt.Errorf("invalid date range")
	}
	dateStr := strings.TrimSpace(parts[0])
	layout := "01/02/2006 15:04"
	t, err := time.Parse(layout, dateStr)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// parseFeetInches parses a string like "53' 0"" into feet as float64
func parseFeetInches(text string) float64 {
	// Remove spaces and special characters
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "\n", "")
	text = strings.TrimSpace(text)

	re := regexp.MustCompile(`(\d+)'`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		feet, _ := strconv.Atoi(matches[1])
		return float64(feet)
	}
	return 0.0
}

// parseWeight parses a weight string like "48,000 lbs" into float64
func parseWeight(weightText string) float64 {
	// Weight text is like: 48,000 lbs
	weightText = strings.TrimSpace(weightText)
	weightText = strings.ReplaceAll(weightText, ",", "")
	re := regexp.MustCompile(`(\d+)\s*lbs`)
	matches := re.FindStringSubmatch(weightText)
	if len(matches) > 1 {
		weight, _ := strconv.ParseFloat(matches[1], 64)
		return weight
	}
	return 0.0
}

// parseCityState splits a location string into city and state
func parseCityState(location string) (string, string) {
	location = strings.TrimSpace(location)
	parts := strings.Split(location, ",")
	if len(parts) >= 2 {
		city := strings.TrimSpace(parts[0])
		state := strings.TrimSpace(parts[1])
		return city, state
	}
	return "", ""
}

//FullCircle parser below

// ExtractOrderNumberFromHTML extracts the order number from the HTML body.
func ExtractOrderNumberFromHTML(doc *goquery.Document) string {
	// Extract order number from the specific tag or context
	orderNumberText := doc.Find("p:contains('ORDER NUMBER')").Text()
	return ExtractOrderNumber(orderNumberText) // Reuse the regex-based extraction function
}

// State code to state name mapping
var stateCodeToName = map[string]string{
	"AL": "Alabama", "AK": "Alaska", "AZ": "Arizona", "AR": "Arkansas", "CA": "California",
	"CO": "Colorado", "CT": "Connecticut", "DE": "Delaware", "FL": "Florida", "GA": "Georgia",
	"HI": "Hawaii", "ID": "Idaho", "IL": "Illinois", "IN": "Indiana", "IA": "Iowa",
	"KS": "Kansas", "KY": "Kentucky", "LA": "Louisiana", "ME": "Maine", "MD": "Maryland",
	"MA": "Massachusetts", "MI": "Michigan", "MN": "Minnesota", "MS": "Mississippi", "MO": "Missouri",
	"MT": "Montana", "NE": "Nebraska", "NV": "Nevada", "NH": "New Hampshire", "NJ": "New Jersey",
	"NM": "New Mexico", "NY": "New York", "NC": "North Carolina", "ND": "North Dakota", "OH": "Ohio",
	"OK": "Oklahoma", "OR": "Oregon", "PA": "Pennsylvania", "RI": "Rhode Island", "SC": "South Carolina",
	"SD": "South Dakota", "TN": "Tennessee", "TX": "Texas", "UT": "Utah", "VT": "Vermont",
	"VA": "Virginia", "WA": "Washington", "WV": "West Virginia", "WI": "Wisconsin", "WY": "Wyoming",
}

// ExtractLocationFromHTML extracts the location details (zip, city, state, country) from the HTML
// Now it will return both the state and stateCode
func ExtractLocationFromHTML(doc *goquery.Document, event string) (string, string, string, string, string) {
	var zip, city, state, stateCode, country string

	// Find the correct table row based on the event name (Pick Up or Delivery)
	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find("td").Eq(1).Text(), event) {
			city = strings.TrimSpace(s.Find("td").Eq(2).Text())
			stateOrCode := strings.TrimSpace(s.Find("td").Eq(3).Text())
			zip = strings.TrimSpace(s.Find("td").Eq(4).Text())
			country = strings.TrimSpace(s.Find("td").Eq(5).Text())

			// Check if the extracted state is a state code, and map to full state name if it is
			if fullName, found := stateCodeToName[stateOrCode]; found {
				stateCode = stateOrCode
				state = fullName
			} else {
				// If it's not a recognized state code, assume it's the full state name
				state = stateOrCode
				stateCode = "" // If no state code was detected, leave it empty
			}
		}
	})

	logrus.WithFields(logrus.Fields{
		"event":     event,
		"city":      city,
		"state":     state,
		"stateCode": stateCode,
		"zip":       zip,
		"country":   country,
	}).Info("Extracted location data")

	return zip, city, state, stateCode, country
}

// ExtractDateTimeStringFromHTML extracts the datetime as a string associated with the pickup or delivery event from the HTML body.
func ExtractDateTimeStringFromHTML(doc *goquery.Document, event string) string {
	var datetimeString string
	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		if s.Find("td").Eq(1).Text() == event {
			datetimeString = s.Find("td").Eq(6).Text()
		}
	})
	return FormatDateTimeString(datetimeString)
}

// formatDateTimeString reformats the datetime string to MySQL format.
func FormatDateTimeString(datetimeString string) string {
	// Extract the date and time without the timezone
	re := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2})`)
	matches := re.FindStringSubmatch(datetimeString)
	if len(matches) > 2 {
		datePart := matches[1]
		timePart := matches[2]
		combined := datePart + " " + timePart

		// Parse the date and time without timezone
		parsedTime, err := time.Parse("2006-01-02 15:04", combined)
		if err == nil {
			// Format as MySQL datetime
			return parsedTime.Format("2006-01-02 15:04:05")
		}
	}
	return ""
}

// ExtractTruckSizeFromHTML extracts the suggested truck size from the HTML body.
func ExtractTruckSizeFromHTML(doc *goquery.Document) string {
	truckSize := doc.Find("p:contains('Requested Vehicle Class')").Text()
	re := regexp.MustCompile(`Requested Vehicle Class:\s*(\w+\s\w+)`)
	matches := re.FindStringSubmatch(truckSize)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ExtractOrderItemsFromHTML extracts the order items (dimensions, weight, etc.) from the HTML body.
func ExtractOrderItemsFromHTML(doc *goquery.Document) (length, width, height, weight float64, pieces int, stackable, hazardous bool) {
	// Extract dimensions from the table following the "Dimensions" paragraph
	dimensionsParagraph := doc.Find("p:contains('Dimensions')")
	dimensionsTable := dimensionsParagraph.NextFiltered("table")

	// Ensure the correct table is selected for dimensions
	dimensionsTable.Find("tr").Each(func(i int, s *goquery.Selection) {
		if i == 1 { // Assuming the second row contains the dimension values
			lengthStr := s.Find("td").Eq(0).Text()
			widthStr := s.Find("td").Eq(1).Text()
			heightStr := s.Find("td").Eq(2).Text()
			stackableStr := s.Find("td").Eq(3).Text()

			// Log extracted values for debugging
			logrus.Infof("Extracted Length: %s, Width: %s, Height: %s, Stackable: %s", lengthStr, widthStr, heightStr, stackableStr)

			// Parse the extracted dimensions, removing units like " in"
			length = parseFloatFromText(strings.TrimSpace(lengthStr))
			width = parseFloatFromText(strings.TrimSpace(widthStr))
			height = parseFloatFromText(strings.TrimSpace(heightStr))
			stackable = strings.TrimSpace(stackableStr) == "Yes"
		}
	})

	// Extract weight using a regex pattern
	weightText := doc.Find("p:contains('Total Weight')").Text()
	reWeight := regexp.MustCompile(`Total Weight:\s*(\d+)\s*lbs`)
	matches := reWeight.FindStringSubmatch(weightText)
	if len(matches) > 1 {
		weight = parseFloat(matches[1])
	}

	// Extract pieces using regex
	piecesText := doc.Find("p:contains('Total Pieces')").Text()
	rePieces := regexp.MustCompile(`Total Pieces:\s*(\d+)`)
	matches = rePieces.FindStringSubmatch(piecesText)
	if len(matches) > 1 {
		pieces, _ = strconv.Atoi(matches[1])
	}

	// Extract hazardous info
	hazardousText := doc.Find("p:contains('Hazardous?')").Text()
	logrus.Infof("Extracted Hazardous Text: %s", hazardousText)

	// Split the text into lines to isolate the "Hazardous? : Yes/No" line
	lines := strings.Split(hazardousText, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Hazardous?") {
			// Extract the value after "Hazardous? :"
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				hazardousValue := strings.TrimSpace(strings.ToLower(parts[1]))
				logrus.Infof("Extracted Hazardous Value: %s", hazardousValue)
				if hazardousValue == "yes" {
					hazardous = true
				} else if hazardousValue == "no" {
					hazardous = false
				} else {
					hazardous = false // Default to false if value is unclear
				}
			}
			break // Exit loop after finding the hazardous line
		}
	}

	// If "Hazardous?" line not found, default to false
	// hazardous variable remains false unless set to true in the loop

	return length, width, height, weight, pieces, stackable, hazardous
}

// Helper function to parse float values from strings with optional units (like " in", " lbs")
func parseFloatFromText(text string) float64 {
	// Use regex to extract numeric part from string
	re := regexp.MustCompile(`(\d+(\.\d+)?)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 0 {
		parsedValue, _ := strconv.ParseFloat(matches[0], 64)
		return parsedValue
	}
	return 0.0
}

// ExtractOrderNumber extracts the order number from the plain text body.
func ExtractOrderNumber(body string) string {
	re := regexp.MustCompile(`(?i)(order(?:\.ref)?|order\s*number|ref\.\s*#|reference)\s*[:#]\s*(\d+)`)
	matches := re.FindStringSubmatch(body)
	if len(matches) > 2 {
		return matches[2]
	}
	return ""
}

// FormatEmailBody takes the plain text body of an email and formats it for better readability.
func FormatEmailBody(emailBody string) string {
	formattedBody := strings.ReplaceAll(emailBody, "\n", "\n\n") // Add extra newlines for better spacing
	formattedBody = strings.ReplaceAll(formattedBody, "\r", "")
	formattedBody = strings.TrimSpace(formattedBody) // Remove leading/trailing whitespace
	return formattedBody
}

// ExtractLocation extracts the pickup or delivery location from the plain text body.
func ExtractLocation(body, event string) (string, string, string, string) { // returns zip, city, state, country
	re := regexp.MustCompile(event + `\s+(\w+)\s+([A-Za-z\s]+)\s+([A-Z]{2})\s+(\d{5})\s+([A-Z]{3})`)
	matches := re.FindStringSubmatch(body)
	if len(matches) == 6 {
		return matches[4], matches[2], matches[3], matches[5]
	}
	return "", "", "", ""
}

// ExtractDateTimeString extracts the datetime as a string associated with the pickup or delivery event from the plain text body.
func ExtractDateTimeString(body, event string) string {
	re := regexp.MustCompile(event + `.*?(\d{4}-\d{2}-\d{2} \d{2}:\d{2}) [A-Z]{3} \(UTC[^\)]+\)`)
	matches := re.FindStringSubmatch(body)
	if len(matches) > 1 {
		return FormatDateTimeString(matches[1])
	}
	return ""
}

// ExtractTruckSize extracts the suggested truck size from the plain text body.
func ExtractTruckSize(body string) string {
	re := regexp.MustCompile(`(?i)Requested Vehicle Class:\s*(\w+\s\w+)`)
	matches := re.FindStringSubmatch(body)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ExtractNotes extracts any shared order notes from the plain text body.
func ExtractNotes(body string) string {
	re := regexp.MustCompile(`(?i)Shared Order notes:\s*(.*)`)
	matches := re.FindStringSubmatch(body)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ExtractOrderItems extracts the order items (dimensions, weight, etc.) from the plain text body.
func ExtractOrderItems(body string) (length, width, height, weight float64, pieces int, stackable, hazardous bool) {
	re := regexp.MustCompile(`(\d+)\s*skids\s*\((\d+\.?\d*)\"L x (\d+\.?\d*)\"W x (\d+\.?\d*)\"H\)\s*@\s*(\d+) lbs`)
	matches := re.FindStringSubmatch(body)
	if len(matches) == 6 {
		length = parseFloat(matches[2])
		width = parseFloat(matches[3])
		height = parseFloat(matches[4])
		weight = parseFloat(matches[5])
		pieces = 4 // Assuming "4 skids" corresponds to pieces
		stackable = strings.Contains(body, "Stackable: Yes")
		hazardous = strings.Contains(body, "Hazardous? : Yes")
	}
	return
}

// Helper function to convert string to float64
func parseFloat(value string) float64 {
	result, _ := strconv.ParseFloat(value, 64)
	return result
}

// FormatLocationLabel formats the location label based on the available information.
func FormatLocationLabel(zip, city, state, country string) string {
	if zip != "" {
		return zip + ", " + city + ", " + state + ", " + country
	}
	return city + ", " + state + ", " + country
}

// Ensure the function extracts email from `mailto:` links in HTML
func ExtractReplyToFromHTML(doc *goquery.Document) string {
	replyTo := ""
	doc.Find("a[href^='mailto:']").Each(func(index int, item *goquery.Selection) {
		href, exists := item.Attr("href")
		if exists && strings.HasPrefix(href, "mailto:") {
			replyTo = strings.TrimPrefix(href, "mailto:") // Strip 'mailto:' prefix
		}
	})
	return replyTo
}

// This is a basic email regex pattern
var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

// ExtractReplyTo looks for the phrase "reply to" and extracts the email address after it.
func ExtractReplyTo(body string) string {
	// Log the body to debug what's inside it
	log.Printf("Body content before searching for reply-to:\n%s\n", body)

	// Convert body to lowercase for case-insensitive matching
	lowerBody := strings.ToLower(body)

	// Look for "reply to" followed by an email address
	keyword := "reply to"
	keywordIndex := strings.Index(lowerBody, keyword)

	if keywordIndex != -1 {
		// Extract the portion of the text after "reply to"
		afterKeyword := body[keywordIndex+len(keyword):]

		// Log the text after "reply to" for debugging
		log.Printf("Text after 'reply to': %s\n", afterKeyword)

		// Use the email regex to find the email in the extracted portion
		matches := emailRegex.FindString(afterKeyword)
		if matches != "" {
			return matches // Return the first matched email
		}
	}

	// If no email is found, return empty string
	return ""
}

// ExtractEstimatedMiles extracts the estimated distance in miles from the HTML or plain text body.
func ExtractEstimatedMiles(doc *goquery.Document, plainTextBody string) int {
	// Try extracting from HTML first
	distance := ExtractDistanceFromHTML(doc)
	if distance > 0 {
		return distance
	}

	// Fallback to plain text extraction if HTML parsing fails
	return ExtractDistance(plainTextBody)
}

// ExtractDistanceFromHTML extracts the distance in miles from the HTML body.
func ExtractDistanceFromHTML(doc *goquery.Document) int {
	distanceText := doc.Find("p:contains('Distance')").Text()
	re := regexp.MustCompile(`Distance:\s*(\d+)\s*mi`)
	matches := re.FindStringSubmatch(distanceText)
	if len(matches) > 1 {
		return parseInt(matches[1])
	}
	return 0
}

// ExtractDistance extracts the distance in miles from the plain text body.
func ExtractDistance(body string) int {
	re := regexp.MustCompile(`(?i)Distance:\s*(\d+)\s*mi`)
	matches := re.FindStringSubmatch(body)
	if len(matches) > 1 {
		return parseInt(matches[1])
	}
	return 0
}

// Helper function to convert string to int
func parseInt(value string) int {
	result, _ := strconv.Atoi(value)
	return result
}

// ExtractTruckClassFromHTML extracts the truck class (e.g., Small Straight, Large Straight, Tractor Trailer) from the HTML document.
func ExtractTruckClassFromHTML(doc *goquery.Document) string {
	var truckClass string

	// Look for the <p> tag that contains "Requested Vehicle Class" and extract the text after the colon.
	doc.Find("p").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "Requested Vehicle Class") {
			// Split the text at the colon and trim spaces to clean the result
			parts := strings.Split(s.Text(), ":")
			if len(parts) > 1 {
				// Extract only the truck class part (e.g., "Tractor Trailer")
				truckClass = strings.TrimSpace(parts[1])

				// Remove any extra trailing text after the truck class (like "We call this vehicle class")
				truckClass = strings.Split(truckClass, "\n")[0]
				truckClass = strings.TrimSpace(truckClass) // Ensure no extra spaces
			}
			logrus.Infof("Extracted Truck Class: %s", truckClass)
		}
	})

	// Return the extracted truck class or a default value if not found
	if truckClass != "" {
		return truckClass
	}
	return "Unknown Truck Class"
}

// ExtractNotesFromHTML extracts the notes from the HTML body
func ExtractNotesFromHTML(doc *goquery.Document) (notes string) {
	// Find the <p> or any tag containing "Notes:" in the text
	doc.Find("p, h4").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if strings.Contains(text, "Notes:") {
			// Extract the part after "Notes:" and trim any extra spaces
			notes = strings.TrimSpace(strings.SplitAfter(text, "Notes:")[1])

			// Log the extracted notes for debugging
			logrus.Infof("Extracted Notes: %s", notes)
		}
	})

	return notes
}
