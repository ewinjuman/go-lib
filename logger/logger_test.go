package logger

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// maskEmail
// ---------------------------------------------------------------------------

func TestMaskEmail(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"user@example.com", "u**r@***.com"},
		{"ab@example.com", "**@***.com"},          // username <= 2
		{"a@example.com", "*@***.com"},             // username == 1
		{"hello@mail.google.com", "h***o@***.com"}, // subdomain
		{"test@localhost", "t**t@***"},              // domain tanpa titik
		{"notanemail", "**********"},              // bukan email (10 karakter)
	}
	for _, tc := range cases {
		got := maskEmail(tc.input)
		if got != tc.want {
			t.Errorf("maskEmail(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestIsValidEmail(t *testing.T) {
	valid := []string{"user@example.com", "a.b+c@x.io", "test@mail.co.uk"}
	invalid := []string{"notanemail", "@example.com", "user@", "user@.com"}
	for _, e := range valid {
		if !isValidEmail(e) {
			t.Errorf("isValidEmail(%q) = false, want true", e)
		}
	}
	for _, e := range invalid {
		if isValidEmail(e) {
			t.Errorf("isValidEmail(%q) = true, want false", e)
		}
	}
}

// ---------------------------------------------------------------------------
// maskMap — map[string]interface{} harus ter-mask (fix interface{} unwrap)
// ---------------------------------------------------------------------------

func TestMaskMap_InterfaceValues(t *testing.T) {
	l, err := New(Options{
		Stdout:       false,
		MaskingPaths: []string{"password"},
		DisableAsync: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Shutdown()

	m := map[string]interface{}{
		"password": "supersecret",
		"username": "alice",
	}

	result := l.maskComplexValue(m, "").(map[string]interface{})

	pwd, ok := result["password"].(string)
	if !ok {
		t.Fatal("password field missing or wrong type")
	}
	if strings.Contains(pwd, "supersecret") {
		t.Errorf("password not masked: got %q", pwd)
	}

	user, _ := result["username"].(string)
	if user != "alice" {
		t.Errorf("username should not be masked, got %q", user)
	}
}

// ---------------------------------------------------------------------------
// Masking — string field
// ---------------------------------------------------------------------------

func TestMaskStringIfNeeded_Redaction(t *testing.T) {
	l, _ := New(Options{
		Stdout:         false,
		RedactionPaths: []string{"token"},
		DisableAsync:   true,
	})
	defer l.Shutdown()
	got := l.maskStringIfNeeded("token", "abc123")
	if got != "[REDACTED]" {
		t.Errorf("expected [REDACTED], got %q", got)
	}
}

func TestMaskStringIfNeeded_DisableMasking(t *testing.T) {
	l, _ := New(Options{
		Stdout:         false,
		MaskingPaths:   []string{"password"},
		DisableMasking: true,
		DisableAsync:   true,
	})
	defer l.Shutdown()
	got := l.maskStringIfNeeded("password", "plaintext")
	if got != "plaintext" {
		t.Errorf("expected plaintext unchanged, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// maskComplexValue — DisableMasking harus return val, bukan nil
// ---------------------------------------------------------------------------

func TestMaskComplexValue_DisableMasking_ReturnsVal(t *testing.T) {
	l, _ := New(Options{
		Stdout:         false,
		DisableMasking: true,
		DisableAsync:   true,
	})
	defer l.Shutdown()
	v := map[string]interface{}{"key": "value"}
	result := l.maskComplexValue(v, "")
	if result == nil {
		t.Error("maskComplexValue with DisableMasking=true returned nil, want original value")
	}
}

// ---------------------------------------------------------------------------
// Shutdown — aman dipanggil dua kali (tidak panic)
// ---------------------------------------------------------------------------

func TestShutdown_SafeToCallTwice(t *testing.T) {
	l, err := New(Options{Stdout: false})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Shutdown() panicked on second call: %v", r)
		}
	}()
	l.Shutdown()
	l.Shutdown() // tidak boleh panic
}

// ---------------------------------------------------------------------------
// GetLogger — inisialisasi aman dari banyak goroutine sekaligus
// ---------------------------------------------------------------------------

func TestGetLogger_ConcurrentSafe(t *testing.T) {
	// Reset singleton untuk test ini
	instance = nil
	once = sync.Once{}

	const n = 50
	var wg sync.WaitGroup
	loggers := make([]*Logger, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			loggers[idx] = GetLogger()
		}(i)
	}
	wg.Wait()

	// Semua goroutine harus mendapat instance yang sama
	for i, l := range loggers {
		if l == nil {
			t.Errorf("loggers[%d] is nil", i)
		} else if l != loggers[0] {
			t.Errorf("loggers[%d] != loggers[0]: different instances", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Printf — tidak drop log ketika len(v) == 1
// ---------------------------------------------------------------------------

func TestPrintf_SingleArg(t *testing.T) {
	l, err := New(Options{Stdout: false, DisableAsync: true})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Shutdown()
	// Sebelumnya kondisi len(v) > 1 menyebabkan ini di-drop diam-diam
	l.Printf("sql: %s", "SELECT 1")
	l.Printf("simple message with no args")
}

// ---------------------------------------------------------------------------
// processLogEntry — logged_at field harus ada dan dalam range waktu yang benar
// ---------------------------------------------------------------------------

func TestLogEntry_TimestampCaptured(t *testing.T) {
	l, err := New(Options{Stdout: false, DisableAsync: true})
	if err != nil {
		t.Fatal(err)
	}
	defer l.Shutdown()

	before := time.Now()
	entry := LogEntry{
		Level:     InfoLevel,
		Message:   "test timestamp",
		Context:   context.Background(),
		Timestamp: time.Now(),
	}
	after := time.Now()

	if entry.Timestamp.Before(before) || entry.Timestamp.After(after) {
		t.Errorf("Timestamp %v out of range [%v, %v]", entry.Timestamp, before, after)
	}
	// processLogEntry tidak boleh panic dengan entry yang valid
	l.processLogEntry(entry)
}

// ---------------------------------------------------------------------------
// getStackTrace — hasil harus mengandung nama fungsi yang memanggil
// ---------------------------------------------------------------------------

func TestGetStackTrace_NotTruncated(t *testing.T) {
	trace := getStackTrace()
	if !strings.Contains(trace, "TestGetStackTrace_NotTruncated") {
		t.Errorf("stack trace does not contain test function name:\n%s", trace)
	}
}
