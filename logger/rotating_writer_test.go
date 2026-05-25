package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestDailyRotatingWriter_DateInFilename memverifikasi bahwa file yang dibuat
// mengandung tanggal hari ini dalam namanya.
func TestDailyRotatingWriter_DateInFilename(t *testing.T) {
	dir := t.TempDir()
	opts := Options{
		Filename:   filepath.Join(dir, "app.log"),
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
		Compress:   false,
	}

	w := newDailyRotatingWriter(opts)
	defer w.Close()

	today := time.Now().Format("2006-01-02")
	expected := fmt.Sprintf("app-%s.log", today)

	got := filepath.Base(w.CurrentFilename())
	if got != expected {
		t.Errorf("filename = %q, want %q", got, expected)
	}
}

// TestDailyRotatingWriter_WritesToFile memverifikasi bahwa log entry benar-benar
// ditulis ke file harian.
func TestDailyRotatingWriter_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	opts := Options{
		Filename:   filepath.Join(dir, "app.log"),
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
		Compress:   false,
	}

	w := newDailyRotatingWriter(opts)

	msg := "hello daily rotation\n"
	n, err := w.Write([]byte(msg))
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != len(msg) {
		t.Errorf("Write() = %d bytes, want %d", n, len(msg))
	}

	filename := w.CurrentFilename()
	// Tutup writer agar file di-flush ke disk
	w.Close()

	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", filename, err)
	}
	if !strings.Contains(string(content), "hello daily rotation") {
		t.Errorf("file content %q missing expected text", string(content))
	}
}

// TestDailyRotatingWriter_DayRollover mensimulasikan pergantian hari:
// logger dianggap sudah berjalan sejak kemarin, lalu Write pertama hari ini
// harus otomatis membuka file dengan tanggal baru.
func TestDailyRotatingWriter_DayRollover(t *testing.T) {
	dir := t.TempDir()
	opts := Options{
		Filename:   filepath.Join(dir, "app.log"),
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
		Compress:   false,
	}

	w := newDailyRotatingWriter(opts)
	defer w.Close()

	// Paksa writer ke kondisi "kemarin": buka file dengan tanggal kemarin
	// dan tulis langsung ke w.writer internal (bypass date-check di Write).
	// Ini mensimulasikan proses yang sudah berjalan sejak kemarin.
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	w.mu.Lock()
	w.openWriter(yesterday)
	_, _ = w.writer.Write([]byte("yesterday log\n")) // tulis bypass date-check
	w.mu.Unlock()

	oldFile := w.CurrentFilename()
	if !strings.Contains(filepath.Base(oldFile), yesterday) {
		t.Fatalf("oldFile %q should contain yesterday's date %q", oldFile, yesterday)
	}

	// Write hari ini — harus memicu rotation ke file baru
	_, err := w.Write([]byte("today log\n"))
	if err != nil {
		t.Fatalf("Write after day rollover: %v", err)
	}

	newFile := w.CurrentFilename()
	today := time.Now().Format("2006-01-02")

	if oldFile == newFile {
		t.Error("expected new dated filename after day rollover, got same file")
	}
	if !strings.Contains(filepath.Base(newFile), today) {
		t.Errorf("new filename %q does not contain today's date %q", newFile, today)
	}
}

// TestDailyRotatingWriter_ConcurrentWrites memverifikasi tidak ada race condition
// saat banyak goroutine menulis bersamaan.
func TestDailyRotatingWriter_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	opts := Options{
		Filename:   filepath.Join(dir, "app.log"),
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
		Compress:   false,
	}

	w := newDailyRotatingWriter(opts)
	defer w.Close()

	const goroutines = 20
	const writesPerGoroutine = 50

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				msg := fmt.Sprintf("goroutine %d write %d\n", id, j)
				if _, err := w.Write([]byte(msg)); err != nil {
					t.Errorf("concurrent Write error: %v", err)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}
