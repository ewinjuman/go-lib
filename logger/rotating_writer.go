package logger

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// dailyRotatingWriter wraps lumberjack.Logger untuk daily log rotation.
//
// Active log file diberi nama dengan tanggal saat ini: <name>-YYYY-MM-DD<ext>.
// Saat tanggal berganti (terdeteksi pada Write berikutnya), writer otomatis membuka
// file baru dengan tanggal baru tanpa perlu restart proses.
//
// Size-based rotation tetap dikelola oleh lumberjack di dalam setiap file harian.
//
// Thread-safe: menggunakan sync.RWMutex dengan double-checked locking pattern
// agar fast-path (hari yang sama) tidak memerlukan write lock.
type dailyRotatingWriter struct {
	mu          sync.RWMutex
	writer      *lumberjack.Logger
	currentDate string // format "2006-01-02"

	// Konfigurasi (immutable setelah konstruksi)
	baseDir    string
	baseName   string
	ext        string
	maxSize    int
	maxBackups int
	maxAge     int
	compress   bool
}

// newDailyRotatingWriter membuat writer baru dan langsung membuka file untuk hari ini.
func newDailyRotatingWriter(opts Options) *dailyRotatingWriter {
	dir := filepath.Dir(opts.Filename)
	base := filepath.Base(opts.Filename)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	if ext == "" {
		ext = ".log"
	}

	w := &dailyRotatingWriter{
		baseDir:    dir,
		baseName:   name,
		ext:        ext,
		maxSize:    opts.MaxSize,
		maxBackups: opts.MaxBackups,
		maxAge:     opts.MaxAge,
		compress:   opts.Compress,
	}

	// Buka file untuk hari ini saat konstruksi.
	w.openWriter(time.Now().Format("2006-01-02"))
	return w
}

// filename mengembalikan full path untuk tanggal tertentu.
// Contoh: logs/app-2026-05-25.log
func (w *dailyRotatingWriter) filename(date string) string {
	return filepath.Join(w.baseDir, w.baseName+"-"+date+w.ext)
}

// openWriter membuat lumberjack writer baru untuk tanggal yang diberikan.
// Caller HARUS memegang write lock (w.mu.Lock()).
func (w *dailyRotatingWriter) openWriter(date string) {
	if w.writer != nil {
		// Tutup writer lama agar file handle dilepas.
		_ = w.writer.Close()
	}
	w.writer = &lumberjack.Logger{
		Filename:   w.filename(date),
		MaxSize:    w.maxSize,
		MaxBackups: w.maxBackups,
		MaxAge:     w.maxAge,
		Compress:   w.compress,
		LocalTime:  true,
	}
	w.currentDate = date
}

// Write implements io.Writer.
//
// Fast path (hari sama): hanya RLock → write. Tidak ada alokasi tambahan.
// Slow path (ganti hari): upgrade ke write lock, rotate, lalu write.
// Double-checked locking memastikan rotate hanya terjadi sekali meski ada
// banyak goroutine yang masuk bersamaan saat pergantian hari.
func (w *dailyRotatingWriter) Write(p []byte) (n int, err error) {
	today := time.Now().Format("2006-01-02")

	// Fast path: tanggal belum berubah.
	w.mu.RLock()
	if today == w.currentDate {
		n, err = w.writer.Write(p)
		w.mu.RUnlock()
		return
	}
	w.mu.RUnlock()

	// Tanggal berubah — ambil write lock dan rotate.
	// Double-check setelah lock untuk mencegah rotate ganda dari goroutine
	// yang masuk bersamaan ke slow path.
	w.mu.Lock()
	if today != w.currentDate {
		w.openWriter(today)
	}
	n, err = w.writer.Write(p)
	w.mu.Unlock()
	return
}

// Close menutup underlying writer dan melepas file handle.
func (w *dailyRotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.writer != nil {
		return w.writer.Close()
	}
	return nil
}

// CurrentFilename mengembalikan path file yang sedang aktif ditulis.
// Berguna untuk debugging / health check.
func (w *dailyRotatingWriter) CurrentFilename() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.filename(w.currentDate)
}
