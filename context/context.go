package context

import (
	"context"
	"github.com/ewinjuman/go-lib/v2/constant"
	Logger "github.com/ewinjuman/go-lib/v2/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"time"
)

type AppContext struct {
	RequestID          string
	TraceID            string
	RequestTime        time.Time
	UserID             string
	logger             *Logger.Logger
	IP, UserAgent      string
	Port               int
	SrcIP, URL, Method string
	Header, Request    interface{}
}

// New membuat instance baru AppContext
func New(log *Logger.Logger) *AppContext {

	return &AppContext{
		RequestID:   uuid.New().String(),
		RequestTime: time.Now(),
		logger:      log,
	}
}

// FromFiber mengambil RequestContext dari fiber.Ctx
func FromFiber(c *fiber.Ctx) *AppContext {
	ctx := c.Locals(constant.AppContextKey)
	if requestCtx, ok := ctx.(*AppContext); ok {
		return requestCtx
	}
	return nil
}

func (ac *AppContext) Log() *Logger.Logger {
	return ac.logger
}

func (ac *AppContext) ToContext() context.Context {
	ctx := context.Background()
	ctx = setContextIfNotEmpty(ctx, constant.RequestIDKey, ac.RequestID)
	ctx = setContextIfNotEmpty(ctx, constant.TraceIDKey, ac.TraceID)
	ctx = setContextIfNotEmpty(ctx, constant.UserIDKey, ac.UserID)
	ctx = setContextIfNotZeroTime(ctx, constant.RequestTimeKey, ac.RequestTime)
	ctx = setContextIfNotEmpty(ctx, constant.RequestMethodKey, ac.Method)
	ctx = setContextIfNotEmpty(ctx, constant.RequestPathKey, ac.URL)
	ctx = setContextIfNotEmpty(ctx, constant.RequestIPKey, ac.IP)
	ctx = setContextIfNotEmpty(ctx, constant.RequestAgentKey, ac.UserAgent)
	return ctx
}

// Helper function untuk mengecek string kosong
func setContextIfNotEmpty(ctx context.Context, key interface{}, value string) context.Context {
	if value != "" {
		return context.WithValue(ctx, key, value)
	}
	return ctx
}

// Helper function untuk mengecek time.Time tidak nol
func setContextIfNotZeroTime(ctx context.Context, key interface{}, value time.Time) context.Context {
	if !value.IsZero() {
		return context.WithValue(ctx, key, value)
	}
	return ctx
}

// Helper function untuk mengecek interface{} tidak nil
func setContextIfNotNil(ctx context.Context, key interface{}, value interface{}) context.Context {
	if value != nil {
		return context.WithValue(ctx, key, value)
	}
	return ctx
}
