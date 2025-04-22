package logger

import (
	"context"
	"fmt"
	"time"
	
	gormlogger "gorm.io/gorm/logger"
)

// GormLogger is a custom logger for GORM that doesn't block
type GormLogger struct {
	ctx                       context.Context
	Logger                    *Logger
	SlowThreshold             time.Duration
	LogLevel                  gormlogger.LogLevel
	IgnoreRecordNotFoundError bool
	ParameterizedQueries      bool
}

// NewGormLogger creates a new non-blocking GORM logger
func NewGormLogger(ctx context.Context, logger *Logger, config gormlogger.Config) *GormLogger {
	return &GormLogger{
		ctx:                       ctx,
		Logger:                    logger,
		SlowThreshold:             config.SlowThreshold,
		LogLevel:                  config.LogLevel,
		IgnoreRecordNotFoundError: config.IgnoreRecordNotFoundError,
		ParameterizedQueries:      config.ParameterizedQueries,
	}
}

// LogMode returns a new logger with updated LogLevel
func (l *GormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info logs info messages in a non-blocking way
func (l *GormLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Info {
		formattedMsg := fmt.Sprintf(msg, data...)
		go l.Logger.Info(ctx, "GORM", String("message", formattedMsg))
	}
}

// Warn logs warn messages in a non-blocking way
func (l *GormLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Warn {
		formattedMsg := fmt.Sprintf(msg, data...)
		go l.Logger.Warn(ctx, "GORM", String("message", formattedMsg))
	}
}

// Error logs error messages in a non-blocking way
func (l *GormLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Error {
		formattedMsg := fmt.Sprintf(msg, data...)
		go l.Logger.Error(ctx, "GORM", String("message", formattedMsg))
	}
}

// Trace logs SQL operations in a non-blocking way
func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= gormlogger.Silent {
		return
	}
	if l.ctx != nil {
		ctx = l.ctx
	}

	// Execute the function to get SQL and rows now, before going into the goroutine
	// This ensures the SQL query is captured correctly from the current context
	sql, rows := fc()
	elapsed := time.Since(begin)

	// Now process the logging asynchronously
	go func(sqlText string, rowCount int64, elapsedTime time.Duration, queryErr error) {
		fields := []Field{
			String("sql", sqlText),
			Int64("rows", rowCount),
			Duration("elapsed", elapsedTime),
		}

		switch {
		case queryErr != nil && l.LogLevel >= gormlogger.Error && (gormlogger.ErrRecordNotFound != queryErr || !l.IgnoreRecordNotFoundError):
			fields = append(fields, Error(queryErr))
			l.Logger.Error(ctx, "GORM Query Error", fields...)
		case elapsedTime > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= gormlogger.Warn:
			fields = append(fields, String("slow_query", "true"))
			l.Logger.Warn(ctx, "GORM Slow Query", fields...)
		case l.LogLevel >= gormlogger.Info:
			l.Logger.Info(ctx, "GORM Query", fields...)
		}
	}(sql, rows, elapsed, err)
}

// Usage example:
//
// func ConfigureDatabase(dbConfig *config.DatabaseConfig, logger *Logger) (*gorm.DB, error) {
//     // Configure DSN and other things...
//
//     // Create non-blocking GORM logger
//     gormLoggerConfig := gormlogger.Config{
//         SlowThreshold:             time.Second,
//         LogLevel:                  getGormLogLevel(dbConfig.LogLevel),
//         IgnoreRecordNotFoundError: false,
//         ParameterizedQueries:      false,
//     }
//
//     gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
//         Logger: NewGormLogger(logger, gormLoggerConfig),
//     })
//
//     return gormDB, err
// }
//
// func getGormLogLevel(level string) gormlogger.LogLevel {
//     switch strings.ToLower(level) {
//     case "silent":
//         return gormlogger.Silent
//     case "error":
//         return gormlogger.Error
//     case "warn":
//         return gormlogger.Warn
//     case "info":
//         return gormlogger.Info
//     default:
//         return gormlogger.Error // Default
//     }
// }
