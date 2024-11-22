package context

import (
	"context"
	Logger "github.com/ewinjuman/go-lib/v2/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"time"
)

type ContextKey string

const (
	AppContextKey     ContextKey = "app_context"
	RequestTimeKey    ContextKey = "request_time"
	RequestMethodKey  ContextKey = "request_method"
	RequestPathKey    ContextKey = "request_path"
	RequestIPKey      ContextKey = "request_ip"
	RequestAgentKey   ContextKey = "request_user_agent"
	ResponseStatusKey ContextKey = "response_status"
	ResponseTimeKey   ContextKey = "response_time"
	RequestIDKey      ContextKey = "request_id"
	TraceIDKey        ContextKey = "trace_id"
	UserIDKey         ContextKey = "user_id"
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
	ctx := c.Locals(AppContextKey)
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
	ctx = context.WithValue(ctx, RequestIDKey, ac.RequestID)
	ctx = context.WithValue(ctx, TraceIDKey, ac.TraceID)
	ctx = context.WithValue(ctx, UserIDKey, ac.UserID)
	ctx = context.WithValue(ctx, RequestTimeKey, ac.RequestTime)
	ctx = context.WithValue(ctx, RequestMethodKey, ac.Method)
	ctx = context.WithValue(ctx, RequestPathKey, ac.URL)
	ctx = context.WithValue(ctx, RequestIPKey, ac.IP)
	ctx = context.WithValue(ctx, RequestAgentKey, ac.UserAgent)
	return ctx
}
