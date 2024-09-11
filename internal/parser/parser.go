package parser

import (
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/sirupsen/logrus"
)

// ExtractOrderNumberFromHTML extracts the order number from the HTML body.
func ExtractOrderNumberFromHTML(doc *goquery.Document) string {
	// Extract order number from the specific tag or context
	orderNumberText := doc.Find("p:contains('ORDER NUMBER')").Text()
	return ExtractOrderNumber(orderNumberText) // Reuse the regex-based extraction function
}

func ExtractLocationFromHTML(doc *goquery.Document, event string) (string, string, string, string) {
	var zip, city, state, country string

	// Find the correct table row based on the event name (Pick Up or Delivery)
	doc.Find("tr").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find("td").Eq(1).Text(), event) {
			city = strings.TrimSpace(s.Find("td").Eq(2).Text())
			state = strings.TrimSpace(s.Find("td").Eq(3).Text())
			zip = strings.TrimSpace(s.Find("td").Eq(4).Text())
			country = strings.TrimSpace(s.Find("td").Eq(5).Text())
		}
	})

	logrus.WithFields(logrus.Fields{
		"event":   event,
		"city":    city,
		"state":   state,
		"zip":     zip,
		"country": country,
	}).Info("Extracted location data")

	return zip, city, state, country
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

// ExtractNotesFromHTML extracts any shared order notes from the HTML body.
func ExtractNotesFromHTML(doc *goquery.Document) string {
	notes := doc.Find("td:contains('Shared Order notes')").Next().Text()
	return notes
}

// ExtractOrderItemsFromHTML extracts the order items (dimensions, weight, etc.) from the HTML body.
func ExtractOrderItemsFromHTML(doc *goquery.Document) (length, width, height, weight float64, pieces int, stackable, hazardous bool) {
	// Log the entire HTML document for debugging
	if html, err := doc.Html(); err == nil {
		logrus.Infof("Entire HTML Document: %s", html)
	} else {
		logrus.Errorf("Error retrieving entire HTML document: %v", err)
	}

	// Find the <p> tag containing "Dimensions"
	dimensionsParagraph := doc.Find("p:contains('Dimensions')")

	// Select the next <table> element following the <p> tag
	dimensionsTable := dimensionsParagraph.NextFiltered("table")

	// Debugging: Log the entire table's HTML to see if we're selecting the correct one
	if html, err := dimensionsTable.Html(); err == nil {
		logrus.Infof("Dimensions Table HTML: %s", html)
	} else {
		logrus.Errorf("Error retrieving Dimensions Table HTML: %v", err)
	}

	// Now try to find the rows within that table
	dimensionsTable.Find("tr").Each(func(i int, s *goquery.Selection) {
		logrus.Infof("Row %d: %s", i, s.Text())
		if i == 1 { // Ensure you are selecting the correct data row
			lengthStr := s.Find("td").Eq(0).Text()
			widthStr := s.Find("td").Eq(1).Text()
			heightStr := s.Find("td").Eq(2).Text()
			stackableStr := s.Find("td").Eq(3).Text()

			logrus.Infof("Extracted Length: %s, Width: %s, Height: %s, Stackable: %s", lengthStr, widthStr, heightStr, stackableStr)

			length = parseFloat(strings.TrimSuffix(lengthStr, " in"))
			width = parseFloat(strings.TrimSuffix(widthStr, " in"))
			height = parseFloat(strings.TrimSuffix(heightStr, " in"))
			stackable = strings.TrimSpace(stackableStr) == "Yes"
		}
	})

	// Extract weight
	weightText := doc.Find("p:contains('Total Weight')").Text()
	reWeight := regexp.MustCompile(`Total Weight:\s*(\d+)\s*lbs`)
	matches := reWeight.FindStringSubmatch(weightText)
	if len(matches) > 1 {
		weight = parseFloat(matches[1])
	}

	// Extract pieces
	piecesText := doc.Find("p:contains('Total Pieces')").Text()
	rePieces := regexp.MustCompile(`Total Pieces:\s*(\d+)`)
	matches = rePieces.FindStringSubmatch(piecesText)
	if len(matches) > 1 {
		pieces, _ = strconv.Atoi(matches[1])
	}

	// Extract hazardous
	hazardousText := doc.Find("p:contains('Hazardous?')").Text()
	hazardous = strings.Contains(hazardousText, "No")

	return length, width, height, weight, pieces, stackable, hazardous
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
