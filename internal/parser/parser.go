package parser

import (
	"regexp"
	"strings"
)

// ExtractOrderNumber takes the plain text body of an email and extracts the order number.
func ExtractOrderNumber(emailBody string) string {
	// Define a regex pattern to find the order number
	// Assuming the order number is something like "#123456" or similar
	orderNumberPattern := `Order\s#(\d+)`

	// Compile the regex
	re := regexp.MustCompile(orderNumberPattern)

	// Find the order number in the email body
	match := re.FindStringSubmatch(emailBody)

	// If a match is found, return the order number
	if len(match) > 1 {
		return match[1]
	}

	// If no match is found, return an empty string
	return ""
}

// FormatEmailBody takes the plain text body of an email and formats it for better readability.
func FormatEmailBody(emailBody string) string {
	// Replace multiple spaces with a single space
	formattedBody := strings.ReplaceAll(emailBody, "\n", "\n\n") // Add extra newlines for better spacing
	formattedBody = strings.ReplaceAll(formattedBody, "\r", "")
	formattedBody = strings.TrimSpace(formattedBody) // Remove leading/trailing whitespace

	return formattedBody
}
