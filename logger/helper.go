package logger

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

func getLogFilename(basePath string) string {
	return fmt.Sprintf("%s-%s.log",
		strings.TrimSuffix(basePath, ".log"),
		time.Now().Format("2006-01-02"))
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)
	return emailRegex.MatchString(email)
}

func maskEmail(email string) string {
	// Memisahkan username dan domain
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return email
	}

	// Masking username
	username := parts[0]
	if len(username) > 2 {
		maskedUsername := username[0:1] + strings.Repeat("*", len(username)-2) + username[len(username)-1:]
		return fmt.Sprintf("%s@%s", maskedUsername, parts[1])
	} else {
		return fmt.Sprintf("%s@%s", strings.Repeat("*", len(username)), parts[0])
	}
	return email
}
