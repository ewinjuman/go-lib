package appContext

import (
	"context"
	"github.com/ewinjuman/go-lib/v2/constant"
	Logger "github.com/ewinjuman/go-lib/v2/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	Map "github.com/orcaman/concurrent-map"
	"time"
)

type AppContext struct {
	cMap               Map.ConcurrentMap
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
		cMap:        Map.New(),
	}
}

func (app *AppContext) SetTraceID(traceID string) *AppContext {
	app.TraceID = traceID
	return app
}

func (app *AppContext) SetUserID(userID string) *AppContext {
	app.UserID = userID
	return app
}

func (app *AppContext) SetRequestID(requestID string) *AppContext {
	app.RequestID = requestID
	return app
}

func (app *AppContext) SetRequestTime(requestTime time.Time) *AppContext {
	app.RequestTime = requestTime
	return app
}

func (app *AppContext) SetLogger(logger *Logger.Logger) *AppContext {
	app.logger = logger
	return app
}

func (app *AppContext) SetIP(ip string) *AppContext {
	app.IP = ip
	return app
}

func (app *AppContext) SetUserAgent(userAgent string) *AppContext {
	app.UserAgent = userAgent
	return app
}

func (app *AppContext) SetPort(port int) *AppContext {
	app.Port = port
	return app
}

func (app *AppContext) SetSrcIP(srcIP string) *AppContext {
	app.SrcIP = srcIP
	return app
}

func (app *AppContext) SetMethod(method string) *AppContext {
	app.Method = method
	return app
}
func (app *AppContext) SetHeader(header interface{}) *AppContext {
	app.Header = header
	return app
}
func (app *AppContext) SetRequest(request interface{}) *AppContext {
	app.Request = request
	return app
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

func (ac *AppContext) Get(key string, defaultValue ...interface{}) (data interface{}) {
	data, ok := ac.cMap.Get(key)
	if !ok {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
	}
	return
}

func (ac *AppContext) Put(key string, data interface{}) {
	ac.cMap.Set(key, data)
}

func (ac *AppContext) Remove(key string) {
	ac.cMap.Remove(key)
}

func (ac *AppContext) LogInfo(msg string, fields ...Logger.Field) {
	ac.Log().Info(ac.ToContext(), msg, fields...)
}

func (ac *AppContext) LogError(msg string, fields ...Logger.Field) {
	ac.Log().Error(ac.ToContext(), msg, fields...)
}

func (ac *AppContext) LogDebug(msg string, fields ...Logger.Field) {
	ac.Log().Debug(ac.ToContext(), msg, fields...)
}
func (ac *AppContext) LogWarn(msg string, fields ...Logger.Field) {
	ac.Log().Warn(ac.ToContext(), msg, fields...)
}
func (ac *AppContext) LogFatal(msg string, fields ...Logger.Field) {
	ac.Log().Fatal(ac.ToContext(), msg, fields...)
}
