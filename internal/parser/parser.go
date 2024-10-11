package parser

import (
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	models "github.com/3milly4ever/parser-landstar/internal/model"
	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

type EmailParser interface {
	Parse(bodyHTML, bodyPlain string) (ParserResult, error)
}

type ParserResult struct {
	Order         models.Order
	OrderLocation models.OrderLocation
	OrderItem     models.OrderItem
	OrderEmail    models.OrderEmail
}

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
