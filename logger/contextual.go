package logger

import (
	"context"
	"net/http"
	"time"
)

// ContextualLogger wraps Logger with a bound context so callers don't need to pass ctx on every call.
type ContextualLogger struct {
	logger *Logger
	ctx    context.Context
}

// NewContextualLogger creates a ContextualLogger bound to the given context.
func NewContextualLogger(l *Logger, ctx context.Context) *ContextualLogger {
	return &ContextualLogger{logger: l, ctx: ctx}
}

// WithContext returns a new ContextualLogger with a different context.
func (c *ContextualLogger) WithContext(ctx context.Context) *ContextualLogger {
	return &ContextualLogger{logger: c.logger, ctx: ctx}
}

// Underlying returns the raw Logger for cases that need direct access.
func (c *ContextualLogger) Underlying() *Logger {
	return c.logger
}

func (c *ContextualLogger) Debug(msg string, fields ...Field) { c.logger.Debug(c.ctx, msg, fields...) }
func (c *ContextualLogger) Info(msg string, fields ...Field)  { c.logger.Info(c.ctx, msg, fields...) }
func (c *ContextualLogger) Warn(msg string, fields ...Field)  { c.logger.Warn(c.ctx, msg, fields...) }
func (c *ContextualLogger) Error(msg string, fields ...Field) { c.logger.Error(c.ctx, msg, fields...) }
func (c *ContextualLogger) Fatal(msg string, fields ...Field) { c.logger.Fatal(c.ctx, msg, fields...) }

func (c *ContextualLogger) LogRequest(url, method string, requestTime time.Time, headers http.Header, request interface{}, message ...interface{}) {
	c.logger.LogRequest(c.ctx, url, method, requestTime, headers, request, message...)
}

func (c *ContextualLogger) LogResponse(url, method string, requestTime time.Time, response interface{}, message ...interface{}) {
	c.logger.LogResponse(c.ctx, url, method, requestTime, response, message...)
}

func (c *ContextualLogger) LogRequestHttp(url, method string, body, header, params interface{}) {
	c.logger.LogRequestHttp(c.ctx, url, method, body, header, params)
}

func (c *ContextualLogger) LogResponseHttp(responseTime time.Duration, code int, url, method string, body interface{}, err error) {
	c.logger.LogResponseHttp(c.ctx, responseTime, code, url, method, body, err)
}

func (c *ContextualLogger) LogRequestGrpc(url, method string, body, header interface{}) {
	c.logger.LogRequestGrpc(c.ctx, url, method, body, header)
}

func (c *ContextualLogger) LogResponseGrpc(startProcessTime time.Time, url, method string, body interface{}) {
	c.logger.LogResponseGrpc(c.ctx, startProcessTime, url, method, body)
}

func (c *ContextualLogger) LogDatabase(sql string, result, errorVal interface{}) {
	c.logger.LogDatabase(c.ctx, sql, result, errorVal)
}