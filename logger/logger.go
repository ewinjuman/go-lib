package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ewinjuman/go-lib/v2/constant"
	"github.com/ewinjuman/go-lib/v2/utils"
	"github.com/gofiber/fiber/v2/log"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Level type untuk custom log levels
type Level string

const (
	DebugLevel Level = "debug"
	InfoLevel  Level = "info"
	WarnLevel  Level = "warn"
	ErrorLevel Level = "error"
	FatalLevel Level = "fatal"
)

// BufferSize menentukan ukuran buffer untuk log entries
const (
	DefaultBufferSize     = 256
	DefaultFlushInterval  = 500 * time.Millisecond
	DefaultWorkerPoolSize = 2
)

// LogEntry merepresentasikan sebuah log entry untuk diproses secara asinkron
type LogEntry struct {
	Level     Level
	Message   string
	Fields    []Field
	Context   context.Context
	Timestamp time.Time
}

// Writer interface
type Writer interface {
	Print(ctx context.Context, message string, value ...interface{})
}

type DefaultWriter struct {
	ID string
}

func sortHeaderKeys(hdrs http.Header) []string {
	keys := make([]string, 0, len(hdrs))
	for key := range hdrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func composeHeaders(hdrs http.Header) string {
	str := make([]string, 0, len(hdrs))
	for _, k := range sortHeaderKeys(hdrs) {
		str = append(str, "\t"+strings.TrimSpace(fmt.Sprintf("%25s: %s", k, strings.Join(hdrs[k], ", "))))
	}
	return strings.Join(str, "\n")
}

// if message == "http_request" ==> [0] = http Method, [1] = url, [2] = request body, [3] = http.Header, [4] = query param
// if message == "http_response" ==> [0] = http Method, [1] = url, [2] = response.StatusCode, [3] = response.Body, [4] = resultRequest.Header, [5] =  response Time, [6] = error
func (w *DefaultWriter) Print(ctx context.Context, message string, value ...interface{}) {
	if len(value) < 2 {
		return
	}

	if message == "http_request" {
		var jsonRequest interface{}
		if value[2] != nil {
			js, _ := json.Marshal(value[2])
			jsonRequest = string(js)
		}
		reqLog := "\n==============================================================================\n" +
			"**** REQUEST ****\n" +
			fmt.Sprintf("%s\n", value[0]) +
			fmt.Sprintf("URL    : %s\n", value[1]) +
			fmt.Sprintf("HEADERS:\n%s\n", composeHeaders(value[3].(http.Header))) +
			fmt.Sprintf("BODY   :\n%v\n", jsonRequest) +
			"------------------------------------------------------------------------------\n"
		log.Debug(reqLog)
	} else if message == "http_response" {
		var jsonResponse interface{}
		if value[3] != nil {
			js, _ := json.Marshal(value[3])
			jsonResponse = string(js)
		}
		debugLog := "\n**** RESPONSE ****\n" +
			fmt.Sprintf("STATUS       : %v\n", value[2].(int)) +
			fmt.Sprintf("TIME DURATION: %v\n", value[5]) +
			"HEADERS      :\n" +
			composeHeaders(value[4].(http.Header)) + "\n" +
			fmt.Sprintf("BODY         :\n%v\n", jsonResponse)
		log.Debug(debugLog)
	}
}

// Options untuk konfigurasi logger
type Options struct {
	AppName        string            // Nama aplikasi
	Environment    string            // Environment (dev/staging/prod)
	Stdout         bool              // Logger ke console
	Write          bool              // Logger ke file
	Filename       string            // Nama file log
	MaxSize        int               // Ukuran maksimal file dalam MB
	MaxBackups     int               // Jumlah backup file yang disimpan
	MaxAge         int               // Umur maksimal file log dalam hari
	Compress       bool              // Kompres file backup
	Level          Level             // Minimum log level
	FunctionKey    string            // key jika ingin caller function di log
	MaskingPaths   []string          // key param JSON yang perlu dimasking
	RedactionPaths []string          // key param yang perlu redaction
	DefaultFields  map[string]string // Fields default yang selalu ada di log
	EnableTrace    bool              // Enable stack trace untuk error
	Development    bool              // Mode development untuk pretty print
	BufferSize     int               // Ukuran buffer untuk log entries
	FlushInterval  time.Duration     // Interval waktu untuk flush buffer
	WorkerPoolSize int               // Jumlah worker goroutine untuk memproses log
	DisableAsync   bool              // Disable asynchronous logging
	DisableMasking bool              // Disable data masking untuk testing/dev
}

// Logger struct utama
type Logger struct {
	logger        *zap.Logger
	maskingPaths  []string
	redactionPath []string
	defaultFields map[string]string
	options       Options

	// Buffer untuk asynchronous logging
	logBuffer  chan LogEntry
	shutdownCh chan struct{}
	flushCh    chan struct{}
	wg         sync.WaitGroup

	// Dedicated goroutine pool untuk processing masking
	maskingPool chan func()

	// Cache untuk path masking matching (optimasi)
	maskingCache sync.Map
}

func (l *Logger) Write(p []byte) (n int, err error) {
	//TODO implement me
	panic("implement me")
}

// DefaultOptions mengembalikan default configuration
func DefaultOptions() Options {
	return Options{
		AppName:        "app",
		Environment:    "development",
		Stdout:         true,
		MaxSize:        100,
		MaxBackups:     7,
		MaxAge:         24,
		Compress:       true,
		Level:          InfoLevel,
		EnableTrace:    false,
		BufferSize:     DefaultBufferSize,
		FlushInterval:  DefaultFlushInterval,
		WorkerPoolSize: DefaultWorkerPoolSize,
		DisableAsync:   false,
		DisableMasking: false,
	}
}

// New membuat instance logger baru
func New(opts Options) (*Logger, error) {
	// Merge dengan default options
	defaultOpts := DefaultOptions()
	if opts.AppName != "" {
		defaultOpts.AppName = opts.AppName
	}
	if opts.Environment != "" {
		defaultOpts.Environment = opts.Environment
	}
	if opts.BufferSize <= 0 {
		opts.BufferSize = DefaultBufferSize
	}
	if opts.FlushInterval <= 0 {
		opts.FlushInterval = DefaultFlushInterval
	}
	if opts.WorkerPoolSize <= 0 {
		opts.WorkerPoolSize = DefaultWorkerPoolSize
	}
	// ... merge other options

	// Setup cores
	var cores []zapcore.Core

	// Encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:     "timestamp",
		LevelKey:    "level",
		NameKey:     "logger",
		CallerKey:   zapcore.OmitKey,
		FunctionKey: zapcore.OmitKey,
		MessageKey:  "message",
		//StacktraceKey: "stacktrace",
		LineEnding:  zapcore.DefaultLineEnding,
		EncodeLevel: zapcore.CapitalLevelEncoder,
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format(time.RFC3339Nano))
		},
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	if opts.FunctionKey != zapcore.OmitKey {
		encoderConfig.FunctionKey = opts.FunctionKey
	}

	// Level
	var zapLevel zapcore.Level
	switch opts.Level {
	case DebugLevel:
		zapLevel = zapcore.DebugLevel
	case InfoLevel:
		zapLevel = zapcore.InfoLevel
	case WarnLevel:
		zapLevel = zapcore.WarnLevel
	case ErrorLevel:
		zapLevel = zapcore.ErrorLevel
	case FatalLevel:
		zapLevel = zapcore.FatalLevel
	default:
		zapLevel = zapcore.InfoLevel
	}

	// Console output
	if opts.Stdout {
		var encoder zapcore.Encoder
		if opts.Development {
			encoder = zapcore.NewConsoleEncoder(encoderConfig)
		} else {
			encoder = zapcore.NewJSONEncoder(encoderConfig)
		}
		cores = append(cores, zapcore.NewCore(
			encoder,
			zapcore.AddSync(os.Stdout),
			zapLevel,
		))
	}

	// File output
	if opts.Filename != "" && opts.Write {
		if err := os.MkdirAll(filepath.Dir(opts.Filename), 0744); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		writer := zapcore.AddSync(&lumberjack.Logger{
			Filename:   opts.Filename,
			MaxSize:    opts.MaxSize,
			MaxBackups: opts.MaxBackups,
			MaxAge:     opts.MaxAge,
			Compress:   opts.Compress,
			LocalTime:  true,
		})

		cores = append(cores, zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			writer,
			zapLevel,
		))
	}

	// Combine cores
	core := zapcore.NewTee(cores...)

	// Create logger
	zapLogger := zap.New(
		core,
		zap.AddCaller(),
		zap.AddCallerSkip(1),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	// Add default fields
	if len(opts.DefaultFields) > 0 {
		fields := make([]zap.Field, 0, len(opts.DefaultFields))
		for k, v := range opts.DefaultFields {
			fields = append(fields, zap.String(k, v))
		}
		zapLogger = zapLogger.With(fields...)
	}

	logger := &Logger{
		logger:        zapLogger,
		maskingPaths:  opts.MaskingPaths,
		redactionPath: opts.RedactionPaths,
		options:       opts,
		logBuffer:     make(chan LogEntry, opts.BufferSize),
		shutdownCh:    make(chan struct{}),
		flushCh:       make(chan struct{}),
		maskingPool:   make(chan func(), opts.WorkerPoolSize),
	}

	// Start async processing if not disabled
	if !opts.DisableAsync {
		// Start worker pools
		for i := 0; i < opts.WorkerPoolSize; i++ {
			logger.wg.Add(1)
			go logger.maskingWorker()
		}

		// Start log processor goroutine
		logger.wg.Add(1)
		go logger.processLogEntries()
	}

	return logger, nil
}

// maskingWorker is a dedicated worker for masking operations
func (l *Logger) maskingWorker() {
	defer l.wg.Done()

	for {
		select {
		case task := <-l.maskingPool:
			task()
		case <-l.shutdownCh:
			return
		}
	}
}

// processLogEntries processes buffered log entries asynchronously
func (l *Logger) processLogEntries() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.options.FlushInterval)
	defer ticker.Stop()

	// Batch flush untuk efisiensi
	batch := make([]LogEntry, 0, l.options.BufferSize)

	flushBatch := func() {
		if len(batch) == 0 {
			return
		}

		// Process each entry in the batch
		for _, entry := range batch {
			l.processLogEntry(entry)
		}

		// Clear the batch
		batch = batch[:0]
	}

	for {
		select {
		case entry := <-l.logBuffer:
			batch = append(batch, entry)

			// Flush immediately if batch is full
			if len(batch) >= l.options.BufferSize {
				flushBatch()
			}

		case <-ticker.C:
			// Flush periodically
			flushBatch()

		case <-l.flushCh:
			// Manual flush requested
			flushBatch()

		case <-l.shutdownCh:
			// Flush remaining entries before shutdown
			flushBatch()
			return
		}
	}
}

// processLogEntry processes a single log entry
func (l *Logger) processLogEntry(entry LogEntry) {
	// Convert fields
	zapFields := l.convertToZapFields(entry.Fields)

	// Add caller information
	zapFields = append(zapFields, zap.String("caller", utils.FileWithLineNum()))

	// Get zap logger with context
	logger := l.WithContext(entry.Context)

	// Log based on level
	switch entry.Level {
	case DebugLevel:
		logger.Debug(entry.Message, zapFields...)
	case InfoLevel:
		logger.Info(entry.Message, zapFields...)
	case WarnLevel:
		logger.Warn(entry.Message, zapFields...)
	case ErrorLevel:
		// Add stack trace for errors if enabled
		if l.options.EnableTrace {
			zapFields = append(zapFields, zap.String("stack_trace", getStackTrace()))
		}
		logger.Error(entry.Message, zapFields...)
	case FatalLevel:
		if l.options.EnableTrace {
			zapFields = append(zapFields, zap.String("stack_trace", getStackTrace()))
		}
		// For fatal, we need to log synchronously then exit
		logger.Fatal(entry.Message, zapFields...)
	}
}

// Shutdown gracefully shuts down the logger
func (l *Logger) Shutdown() {
	// Send shutdown signal to all workers
	close(l.shutdownCh)

	// Wait for all workers to finish
	l.wg.Wait()

	// Sync the underlying zap logger
	_ = l.logger.Sync()
}

// Flush manually flushes the log buffer
func (l *Logger) Flush() {
	select {
	case l.flushCh <- struct{}{}:
		// Signal sent successfully
	default:
		// Channel is full or closed, skip
	}
}

// WithContext menambahkan appContext ke log entry
func (l *Logger) WithContext(ctx context.Context) *zap.Logger {
	fields := []zap.Field{}

	// Add trace ID
	if traceID, ok := ctx.Value(constant.TraceIDKey).(string); ok {
		fields = append(fields, zap.String("trace_id", traceID))
	}

	// Add request ID
	if requestID, ok := ctx.Value(constant.RequestIDKey).(string); ok {
		fields = append(fields, zap.String("request_id", requestID))
	}

	// Add user ID
	if userID, ok := ctx.Value(constant.UserIDKey).(string); ok {
		fields = append(fields, zap.String("user_id", userID))
	}

	return l.logger.With(fields...)
}

// Field adalah struct untuk menyimpan key-value logging
type Field struct {
	Key   string
	Value interface{}
}

// NewField membuat field baru
func NewField(key string, value interface{}) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

// Helper functions untuk membuat Field
func String(key string, value string) Field {
	return NewField(key, value)
}

func Int(key string, value int) Field {
	return NewField(key, value)
}

func Int64(key string, value int64) Field {
	return NewField(key, value)
}

func Float64(key string, value float64) Field {
	return NewField(key, value)
}

func Bool(key string, value bool) Field {
	return NewField(key, value)
}

func Error(err error) Field {
	return NewField("error", err.Error())
}

func Duration(key string, value time.Duration) Field {
	return NewField(key, value)
}

func Interface(key string, value interface{}) Field {
	return NewField(key, value)
}

// convertToZapFields mengkonversi []Field ke []zap.Field dengan masking
func (l *Logger) convertToZapFields(fields []Field) []zap.Field {
	zapFields := make([]zap.Field, len(fields))

	for i, field := range fields {
		switch v := field.Value.(type) {
		case string:
			zapFields[i] = zap.String(field.Key, l.maskStringIfNeeded(field.Key, v))
		case int:
			zapFields[i] = zap.Int(field.Key, v)
		case int64:
			zapFields[i] = zap.Int64(field.Key, v)
		case float64:
			zapFields[i] = zap.Float64(field.Key, v)
		case bool:
			zapFields[i] = zap.Bool(field.Key, v)
		case time.Duration:
			zapFields[i] = zap.Duration(field.Key, v)
		case error:
			zapFields[i] = zap.Error(v)
		default:
			// Mask complex data structures
			maskedValue := l.maskComplexValue(v, field.Key)
			zapFields[i] = zap.Any(field.Key, maskedValue)
		}
	}
	return zapFields
}

// Helper methods
func getStackTrace() string {
	buf := make([]byte, 1024)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// needsMasking cek apakah sebuah field perlu dimasking (dengan cache)
func (l *Logger) needsMasking(key string) bool {
	if l.options.DisableMasking {
		return false
	}

	keyLower := strings.ToLower(key)

	// Cek cache dulu
	if val, ok := l.maskingCache.Load(keyLower); ok {
		return val.(bool)
	}

	// Cek jika perlu dimasking
	for _, path := range l.maskingPaths {
		if strings.Contains(keyLower, strings.ToLower(path)) {
			l.maskingCache.Store(keyLower, true)
			return true
		}
	}

	// Simpan ke cache untuk fast lookup di masa depan
	l.maskingCache.Store(keyLower, false)
	return false
}

// needsRedaction cek apakah sebuah field perlu diredaksi (dengan cache)
func (l *Logger) needsRedaction(key string) bool {
	if l.options.DisableMasking {
		return false
	}

	keyLower := strings.ToLower(key)

	// Cek cache dulu
	if val, ok := l.maskingCache.Load("redact_" + keyLower); ok {
		return val.(bool)
	}

	// Cek jika perlu diredaksi
	for _, path := range l.redactionPath {
		if strings.Contains(keyLower, strings.ToLower(path)) {
			l.maskingCache.Store("redact_"+keyLower, true)
			return true
		}
	}

	// Simpan ke cache untuk fast lookup di masa depan
	l.maskingCache.Store("redact_"+keyLower, false)
	return false
}

// maskStringIfNeeded melakukan masking pada string jika diperlukan (optimized)
func (l *Logger) maskStringIfNeeded(key string, value string) string {
	if l.options.DisableMasking {
		return value
	}

	// Fast path: jika string kosong, return langsung
	if value == "" {
		return value
	}

	// Cek jika perlu redaction
	if l.needsRedaction(key) {
		return "[REDACTED]"
	}

	// Cek jika perlu masking
	if l.needsMasking(key) {
		if isValidEmail(value) {
			return maskEmail(value)
		}
		if len(value) > 8 && len(value) <= 50 {
			return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
		} else if len(value) > 50 {
			return value[0:4] + strings.Repeat("*", 20) + value[len(value)-4:]
		}
		return strings.Repeat("*", len(value))
	}

	return value
}

// sendLogAsync mengirim log entry ke buffer untuk diproses secara async
func (l *Logger) sendLogAsync(level Level, ctx context.Context, msg string, fields ...Field) {
	if ctx == nil {
		ctx = context.Background()
	}

	// Buat entry
	entry := LogEntry{
		Level:     level,
		Message:   msg,
		Fields:    fields,
		Context:   ctx,
		Timestamp: time.Now(),
	}

	// Special case untuk Fatal - selalu syncronous
	if level == FatalLevel {
		l.processLogEntry(entry)
		return
	}

	// Jika async dimatikan, proses langsung
	if l.options.DisableAsync {
		l.processLogEntry(entry)
		return
	}

	// Kirim ke buffer, dengan non-blocking send
	select {
	case l.logBuffer <- entry:
		// Berhasil mengirim ke buffer
	default:
		// Buffer penuh, log secara syncronous
		l.processLogEntry(entry)
	}
}

// Public logging methods (async)
func (l *Logger) Debug(ctx context.Context, msg string, fields ...Field) {
	l.sendLogAsync(DebugLevel, ctx, msg, fields...)
}

func (l *Logger) Info(ctx context.Context, msg string, fields ...Field) {
	l.sendLogAsync(InfoLevel, ctx, msg, fields...)
}

func (l *Logger) Warn(ctx context.Context, msg string, fields ...Field) {
	l.sendLogAsync(WarnLevel, ctx, msg, fields...)
}

func (l *Logger) Error(ctx context.Context, msg string, fields ...Field) {
	l.sendLogAsync(ErrorLevel, ctx, msg, fields...)
}

func (l *Logger) Fatal(ctx context.Context, msg string, fields ...Field) {
	// Fatal selalu syncronous
	l.sendLogAsync(FatalLevel, ctx, msg, fields...)
}

// maskComplexValue melakukan masking pada struktur data kompleks
func (l *Logger) maskComplexValue(val interface{}, path string) interface{} {
	if val == nil || l.options.DisableMasking {
		return nil
	}

	value := reflect.ValueOf(val)

	// Handle pointer
	if value.Kind() == reflect.Ptr {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		return l.maskStruct(value)
	case reflect.Map:
		return l.maskMap(value)
	case reflect.Slice, reflect.Array:
		return l.maskSlice(value)
	case reflect.String:
		return l.maskStringIfNeeded(path, value.String())
	default:
		return val
	}
}

// maskStruct melakukan masking pada struct (optimized)
func (l *Logger) maskStruct(value reflect.Value) interface{} {
	result := make(map[string]interface{})
	typ := value.Type()

	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		fieldType := typ.Field(i)

		// Skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		// Get field name from json tag or struct field name
		fieldName := fieldType.Tag.Get("json")
		if fieldName == "" || fieldName == "-" {
			fieldName = strings.ToLower(fieldType.Name)
		}
		// Remove json tag options (e.g., omitempty)
		fieldName = strings.Split(fieldName, ",")[0]

		// Jika field adalah string, cek masking berdasarkan nama field
		if field.Kind() == reflect.String {
			result[fieldName] = l.maskStringIfNeeded(fieldName, field.String())
		} else {
			// Recursively mask the field value
			result[fieldName] = l.maskComplexValue(field.Interface(), fieldName)
		}
	}

	return result
}

// maskMap melakukan masking pada map (optimized)
func (l *Logger) maskMap(value reflect.Value) interface{} {
	result := make(map[string]interface{})

	for _, key := range value.MapKeys() {
		strKey := fmt.Sprint(key.Interface())
		mapValue := value.MapIndex(key)

		// Jika value adalah string, cek masking berdasarkan key
		if mapValue.Kind() == reflect.String {
			result[strKey] = l.maskStringIfNeeded(strKey, mapValue.String())
		} else {
			result[strKey] = l.maskComplexValue(mapValue.Interface(), strKey)
		}
	}

	return result
}

// maskSlice melakukan masking pada slice/array (optimized)
func (l *Logger) maskSlice(value reflect.Value) interface{} {
	result := make([]interface{}, value.Len())

	for i := 0; i < value.Len(); i++ {
		// Untuk slice, kita tidak punya nama field spesifik, jadi gunakan empty string
		result[i] = l.maskComplexValue(value.Index(i).Interface(), "")
	}

	return result
}

// Printf implementasi untuk kompatibilitas dengan gorm
func (l *Logger) Printf(format string, v ...interface{}) {
	if len(v) > 1 {
		fm := fmt.Sprintf(format, v...)
		l.Info(context.Background(), "ORM LOG", String("value", fm))
	}
}

// Print implementasi untuk kompatibilitas dengan library lain
func (l *Logger) Print(s string, v ...interface{}) {
	if len(v) < 2 {
		return
	}
	l.Info(context.Background(), s, Interface("values", v))
}

// NewContext menciptakan context baru dengan request ID dan trace ID
func NewContext() context.Context {
	ctx := context.Background()
	requestID := uuid.New().String()
	traceID := uuid.New().String()

	ctx = context.WithValue(ctx, constant.RequestIDKey, requestID)
	ctx = context.WithValue(ctx, constant.TraceIDKey, traceID)

	return ctx
}
