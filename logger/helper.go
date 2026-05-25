package logger

import (
	"regexp"
	"strings"
)

// emailRegex dikompilasi sekali saat package diload, bukan setiap kali isValidEmail dipanggil.
var emailRegex = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,4}$`)

func isValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

// maskEmail melakukan partial masking pada email address.
// Format hasil: <first>***<last>@***.<tld>
// Contoh: user@example.com → u**r@***.com
//
// Username: tampilkan karakter pertama dan terakhir, tengah di-mask.
// Domain: tampilkan hanya TLD (bagian setelah titik terakhir), host di-mask jadi "***".
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return strings.Repeat("*", len(email))
	}

	username := parts[0]
	domain := parts[1]

	// Mask username
	var maskedUsername string
	switch {
	case len(username) == 0:
		maskedUsername = "*"
	case len(username) <= 2:
		maskedUsername = strings.Repeat("*", len(username))
	default:
		maskedUsername = username[0:1] + strings.Repeat("*", len(username)-2) + username[len(username)-1:]
	}

	// Mask domain: tampilkan hanya TLD (setelah titik terakhir).
	// example.com → ***.com | mail.google.com → ***.com | localhost → ***
	dotIdx := strings.LastIndex(domain, ".")
	var maskedDomain string
	if dotIdx < 0 {
		maskedDomain = "***"
	} else {
		maskedDomain = "***" + domain[dotIdx:] // "***" + ".com"
	}

	return maskedUsername + "@" + maskedDomain
}
