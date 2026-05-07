package logger

import (
	"regexp"
	"strings"
)

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,4}$`)
	return emailRegex.MatchString(email)
}

func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return strings.Repeat("*", len(email))
	}

	username := parts[0]
	domain := parts[1]

	if len(username) <= 2 {
		maskedUsername := strings.Repeat("*", len(username))
		return maskedUsername + "@" + domain
	}

	maskedUsername := username[0:1] + strings.Repeat("*", len(username)-2) + username[len(username)-1:]
	return maskedUsername + "@" + domain
}
