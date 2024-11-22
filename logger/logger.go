package logger

import (
	"context"
	"fmt"
	"github.com/ewinjuman/go-lib/v2/constant"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"
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

// Options untuk konfigurasi logger
type Options struct {
	AppName       string            // Nama aplikasi
	Environment   string            // Environment (dev/staging/prod)
	Stdout        bool              // Logger ke console
	Filename      string            // Nama file log
	MaxSize       int               // Ukuran maksimal file dalam MB
	MaxBackups    int               // Jumlah backup file yang disimpan
	MaxAge        int               // Umur maksimal file log dalam hari
	Compress      bool              // Kompres file backup
	Level         Level             // Minimum log level
	MaskingPaths  []string          // Path JSON yang perlu dimasking
	DefaultFields map[string]string // Fields default yang selalu ada di log
	EnableTrace   bool              // Enable stack trace untuk error
	Development   bool              // Mode development untuk pretty print
}

// Logger struct utama
type Logger struct {
	sync.RWMutex
	logger        *zap.Logger
	maskingPaths  []string
	defaultFields map[string]string
	options       Options
}

// DefaultOptions mengembalikan default configuration
func DefaultOptions() Options {
	hostname, _ := os.Hostname()
	return Options{
		AppName:     "app",
		Environment: "development",
		Stdout:      true,
		MaxSize:     100,
		MaxBackups:  7,
		MaxAge:      30,
		Compress:    true,
		Level:       InfoLevel,
		EnableTrace: true,
		DefaultFields: map[string]string{
			"hostname": hostname,
			"app":      "app",
		},
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
	// ... merge other options

	// Setup cores
	var cores []zapcore.Core

	// Encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:       "timestamp",
		LevelKey:      "level",
		NameKey:       "logger",
		CallerKey:     "caller",
		FunctionKey:   zapcore.OmitKey,
		MessageKey:    "message",
		StacktraceKey: "stacktrace",
		LineEnding:    zapcore.DefaultLineEnding,
		EncodeLevel:   zapcore.CapitalLevelEncoder,
		EncodeTime: func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
			enc.AppendString(t.Format(time.RFC3339Nano))
		},
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
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
	if opts.Filename != "" {
		opts.Filename = getLogFilename(opts.Filename)
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

	return &Logger{
		logger:       zapLogger,
		maskingPaths: opts.MaskingPaths,
		options:      opts,
	}, nil
}

func getLogFilename(basePath string) string {
	return fmt.Sprintf("%s-%s.log",
		strings.TrimSuffix(basePath, ".log"),
		time.Now().Format("2006-01-02"))
}

// WithContext menambahkan context ke log entry
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

// maskSensitiveData melakukan masking pada data sensitif
func (l *Logger) maskSensitiveData(fields ...zap.Field) []zap.Field {
	l.RLock()
	defer l.RUnlock()

	if len(l.maskingPaths) == 0 {
		return fields
	}

	maskedFields := make([]zap.Field, len(fields))
	for i, field := range fields {

		//maskedFields[i] = l.maskComplexValue(field.Interface, field.Key)
		// Check if field needs masking
		needsMasking := false
		for _, path := range l.maskingPaths {
			if strings.Contains(field.Key, path) {
				needsMasking = true
				break
			}
		}

		if needsMasking && field.Type == zapcore.StringType {
			// Mask value keeping first and last 4 chars
			value := field.String
			if len(value) > 8 && len(value) <= 50 {
				masked := value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
				maskedFields[i] = zap.String(field.Key, masked)
			} else if len(value) > 50 {
				masked := value[:4] + strings.Repeat("*", 40) + value[len(value)-4:]
				maskedFields[i] = zap.String(field.Key, masked)
			} else {
				maskedFields[i] = zap.String(field.Key, "****")
			}
		} else {
			maskedFields[i] = zap.Any(field.Key, l.maskComplexValue(field.Interface, field.Key))
		}
	}

	return maskedFields
}

// Logger methods
func (l *Logger) Debug(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).Debug(msg, l.maskSensitiveData(fields...)...)
}

func (l *Logger) Info(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).Info(msg, l.maskSensitiveData(fields...)...)
}

func (l *Logger) Warn(ctx context.Context, msg string, fields ...zap.Field) {
	l.WithContext(ctx).Warn(msg, l.maskSensitiveData(fields...)...)
}

func (l *Logger) Error(ctx context.Context, msg string, fields ...zap.Field) {
	// Add stack trace for errors
	if l.options.EnableTrace {
		fields = append(fields, zap.String("stack_trace", getStackTrace()))
	}
	l.WithContext(ctx).Error(msg, l.maskSensitiveData(fields...)...)
}

func (l *Logger) Fatal(ctx context.Context, msg string, fields ...zap.Field) {
	if l.options.EnableTrace {
		fields = append(fields, zap.String("stack_trace", getStackTrace()))
	}
	l.WithContext(ctx).Fatal(msg, l.maskSensitiveData(fields...)...)
}

// Helper methods
func getStackTrace() string {
	buf := make([]byte, 1024)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
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

// Logger methods
func (l *Logger) LogDebug(ctx context.Context, msg string, fields ...Field) {
	l.WithContext(ctx).Debug(msg, l.convertToZapFields(fields)...)
}

func (l *Logger) LogInfo(ctx context.Context, msg string, fields ...Field) {
	l.WithContext(ctx).Info(msg, l.convertToZapFields(fields)...)
}

func (l *Logger) LogWarn(ctx context.Context, msg string, fields ...Field) {
	l.WithContext(ctx).Warn(msg, l.convertToZapFields(fields)...)
}

func (l *Logger) LogError(ctx context.Context, msg string, fields ...Field) {
	l.WithContext(ctx).Error(msg, l.convertToZapFields(fields)...)
}

func (l *Logger) LogFatal(ctx context.Context, msg string, fields ...Field) {
	l.WithContext(ctx).Fatal(msg, l.convertToZapFields(fields)...)
}

// maskComplexValue melakukan masking pada struktur data kompleks
func (l *Logger) maskComplexValue(val interface{}, path string) interface{} {
	if val == nil {
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

// maskStruct melakukan masking pada struct
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

// maskMap melakukan masking pada map
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

// maskSlice melakukan masking pada slice/array
func (l *Logger) maskSlice(value reflect.Value) interface{} {
	result := make([]interface{}, value.Len())

	for i := 0; i < value.Len(); i++ {
		// Untuk slice, kita tidak punya nama field spesifik, jadi gunakan empty string
		result[i] = l.maskComplexValue(value.Index(i).Interface(), "")
	}

	return result
}

// maskStringIfNeeded melakukan masking pada string jika diperlukan
func (l *Logger) maskStringIfNeeded(key string, value string) string {
	l.RLock()
	defer l.RUnlock()

	// Ubah key ke lowercase untuk case-insensitive matching
	keyLower := strings.ToLower(key)

	for _, path := range l.maskingPaths {

		if strings.Contains(keyLower, strings.ToLower(path)) {
			if len(value) > 8 && len(value) <= 50 {
				return value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:]
			} else if len(value) > 50 {
				return value[:4] + strings.Repeat("*", 40) + value[len(value)-4:]
			}
			return "****"
		}
	}
	return value
}

func (l *Logger) Printf(s string, v ...interface{}) {
	if len(v) == 4 {
		l.Info(context.Background(), "",
			zap.String("query", v[3].(string)),
			zap.String("duration ", fmt.Sprintf("%.3fms", v[1].(float64))),
			zap.Int64("affected-rows", v[2].(int64)),
			zap.String("source", v[0].(string)),
		)
	} else {
		l.Info(context.Background(), "",
			zap.Any("value", v),
		)
	}
}
func (l *Logger) Print(v ...interface{}) {
	if len(v) < 2 {
		return
	}
	switch v[0] {
	case "sql":
		delimiter := "/"
		rightOfDelimiter := strings.Join(strings.Split(v[1].(string), delimiter)[4:], delimiter)
		l.Info(context.Background(), "",
			zap.String("query", v[3].(string)),
			zap.Any("values", v[4]),
			zap.Float64("duration", float64(v[2].(time.Duration))/float64(time.Millisecond)),
			zap.Int64("affected-rows", v[5].(int64)),
			zap.String("source", rightOfDelimiter),
		)
	default:
		delimiter := "/"
		rightOfDelimiter := strings.Join(strings.Split(v[1].(string), delimiter)[4:], delimiter)
		l.Info(context.Background(), "",
			zap.Any("values", v[2:]),
			zap.String("source", rightOfDelimiter),
		)
	}
}

// NewContext contoh untuk membuat context baru dengan request ID dan trace ID
func NewContext() context.Context {
	ctx := context.Background()
	requestID := uuid.New().String()
	traceID := uuid.New().String()

	ctx = context.WithValue(ctx, constant.RequestIDKey, requestID)
	ctx = context.WithValue(ctx, constant.TraceIDKey, traceID)

	return ctx
}

// Contoh penggunaan
func ExampleUsage() {
	// Initialize logger
	//logger, err := New(Options{
	//	AppName:     "myapp",
	//	Environment: "production",
	//	Stdout:      true,
	//	Filename:    "log/app.log",
	//	MaxSize:     100,
	//	MaxBackups:  7,
	//	MaxAge:      30,
	//	Level:       InfoLevel,
	//	MaskingPaths: []string{
	//		"password",
	//		"credit_card",
	//		"ssn",
	//	},
	//	DefaultFields: map[string]string{
	//		"region": "us-west",
	//		"dc":     "dc1",
	//	},
	//	EnableTrace: false,
	//	Development: false,
	//})
	//if err != nil {
	//	panic(err)
	//}

	InitLogger(Options{
		AppName:     "myapp",
		Environment: "production",
		Stdout:      true,
		Filename:    "log/app.log",
		MaxSize:     100,
		MaxBackups:  7,
		MaxAge:      30,
		Level:       InfoLevel,
		MaskingPaths: []string{
			"password",
			"credit_card",
			"ssn",
		},
		DefaultFields: map[string]string{
			"region": "us-west",
			"dc":     "dc1",
		},
		EnableTrace: false,
		Development: false,
	})

	// Create context with trace
	ctx := NewContext()

	// Example logs
	GetLogger().Info(ctx, "Application started",
		zap.String("version", "1.0.0"),
	)

	// logger with sensitive data
	GetLogger().Info(ctx, "User updated profile",
		zap.String("user_id", "123"),
		zap.String("credit_card", "4111-1111-1111-1111"), // Will be masked
	)

	// logger error with stack trace
	err := fmt.Errorf("database connection failed")
	GetLogger().Error(ctx, "Failed to process request",
		zap.Error(err),
		zap.String("user_id", "123"),
	)

	// Contoh penggunaan logger wrapper
	GetLogger().LogInfo(ctx, "Application started",
		String("version", "1.0.0"),
		String("environment", "production"),
	)

	// logger dengan data sensitif
	GetLogger().LogInfo(ctx, "User updated profile",
		String("user_id", "123"),
		String("credit_card", "4111-1111-1111-1111"), // Akan tetap dimasking
	)

	// logger error dengan data terstruktur
	user := struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}{
		ID:    "123",
		Email: "user@example.com",
	}

	err = fmt.Errorf("database connection failed")
	GetLogger().LogError(ctx, "Failed to process request",
		Error(err),
		Interface("user", user),
		Duration("response_time", 1500*time.Millisecond),
	)
}

// example singleton logger
var (
	// instance singleton dari logger
	instance *Logger
	once     sync.Once
)

// InitLogger contoh inisialisasi logger sekali saja
func InitLogger(opts Options) {
	once.Do(func() {
		logger, err := New(opts)
		if err != nil {
			panic(err)
		}
		instance = logger
	})
}

// GetLogger contoh get logger
func GetLogger() *Logger {
	if instance == nil {
		// Default config jika belum diinisialisasi
		InitLogger(DefaultOptions())
	}
	return instance
}
