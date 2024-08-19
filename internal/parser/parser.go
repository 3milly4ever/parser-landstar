package parser

import (
	"regexp"
	"strings"
)

func ExtractOrderNumber(body string) string {
	// Updated regex pattern to capture "Order", "Order Number", "Order Ref", "Order.Ref", "Ref.#", "Reference", followed by a number
	re := regexp.MustCompile(`(?i)(order(?:\.ref)?|order\s*number|ref\.\s*#|reference)\s*[:#]\s*(\d+)`)
	matches := re.FindStringSubmatch(body)
	if len(matches) > 2 {
		return matches[2]
	}
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
