package appContext

import (
	"context"
	"sync"
	"time"

	"github.com/ewinjuman/go-lib/v2/constant"
	Logger "github.com/ewinjuman/go-lib/v2/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// AppContext adalah request-scoped context carrier.
//
// Fields yang dipropagasi ke context (via ToContext):
//
//	requestID, traceID, userID, requestTime, method, url, ip, userAgent
//
// Fields yang hanya disimpan lokal (tidak ke context):
//
//	port, srcIP, header, request
//
// Semua field unexported; gunakan Set*/Get* accessor yang disediakan.
type AppContext struct {
	mu        sync.RWMutex
	cachedCtx context.Context // nil = dirty / belum dibangun

	requestID   string
	traceID     string
	requestTime time.Time
	userID      string
	ip          string
	userAgent   string
	url         string
	method      string

	// Local-only — tidak dipropagasi ke context.
	port    int
	srcIP   string
	header  interface{}
	request interface{}

	logger    *Logger.Logger
	parentCtx context.Context
	cMap      sync.Map // general-purpose key-value store untuk request scope
}

// New membuat instance baru AppContext dengan parent context.
// ctx digunakan sebagai base untuk ToContext() sehingga deadline dan cancellation tetap terbawa.
// log boleh nil; jika nil akan menggunakan global singleton logger.
func New(ctx context.Context, log *Logger.Logger) *AppContext {
	if ctx == nil {
		ctx = context.Background()
	}
	if log == nil {
		log = Logger.GetLogger()
	}
	return &AppContext{
		requestID:   uuid.New().String(),
		requestTime: time.Now(),
		logger:      log,
		parentCtx:   ctx,
	}
}

// FromFiber mengambil AppContext dari Fiber Locals.
// Mengembalikan instance baru (tidak pernah nil) jika belum ada yang disimpan.
func FromFiber(c *fiber.Ctx) *AppContext {
	if ac, ok := c.Locals(constant.AppContextKey).(*AppContext); ok {
		return ac
	}
	return New(c.UserContext(), nil)
}

// ── Getters ──────────────────────────────────────────────────────────────────

func (ac *AppContext) GetRequestID() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.requestID
}

func (ac *AppContext) GetTraceID() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.traceID
}

func (ac *AppContext) GetUserID() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.userID
}

func (ac *AppContext) GetRequestTime() time.Time {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.requestTime
}

func (ac *AppContext) GetIP() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.ip
}

func (ac *AppContext) GetUserAgent() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.userAgent
}

func (ac *AppContext) GetURL() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.url
}

func (ac *AppContext) GetMethod() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.method
}

// GetPort mengembalikan port (local-only, tidak dipropagasi ke context).
func (ac *AppContext) GetPort() int {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.port
}

// GetSrcIP mengembalikan source IP (local-only, tidak dipropagasi ke context).
func (ac *AppContext) GetSrcIP() string {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.srcIP
}

// GetHeader mengembalikan header (local-only, tidak dipropagasi ke context).
func (ac *AppContext) GetHeader() interface{} {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.header
}

// GetRequest mengembalikan request body (local-only, tidak dipropagasi ke context).
func (ac *AppContext) GetRequest() interface{} {
	ac.mu.RLock()
	defer ac.mu.RUnlock()
	return ac.request
}

// ── Setters ───────────────────────────────────────────────────────────────────

// invalidate menghapus cached context; dipanggil saat field yang dipropagasi berubah.
// Caller harus memegang ac.mu write lock.
func (ac *AppContext) invalidate() {
	ac.cachedCtx = nil
}

func (ac *AppContext) SetTraceID(traceID string) *AppContext {
	ac.mu.Lock()
	ac.traceID = traceID
	ac.invalidate()
	ac.mu.Unlock()
	return ac
}

func (ac *AppContext) SetUserID(userID string) *AppContext {
	ac.mu.Lock()
	ac.userID = userID
	ac.invalidate()
	ac.mu.Unlock()
	return ac
}

func (ac *AppContext) SetRequestID(requestID string) *AppContext {
	ac.mu.Lock()
	ac.requestID = requestID
	ac.invalidate()
	ac.mu.Unlock()
	return ac
}

func (ac *AppContext) SetRequestTime(requestTime time.Time) *AppContext {
	ac.mu.Lock()
	ac.requestTime = requestTime
	ac.invalidate()
	ac.mu.Unlock()
	return ac
}

func (ac *AppContext) SetLogger(logger *Logger.Logger) *AppContext {
	ac.mu.Lock()
	ac.logger = logger
	ac.mu.Unlock()
	return ac
}

func (ac *AppContext) SetIP(ip string) *AppContext {
	ac.mu.Lock()
	ac.ip = ip
	ac.invalidate()
	ac.mu.Unlock()
	return ac
}

func (ac *AppContext) SetUserAgent(userAgent string) *AppContext {
	ac.mu.Lock()
	ac.userAgent = userAgent
	ac.invalidate()
	ac.mu.Unlock()
	return ac
}

// SetURL sets the request URL and propagates it to the context via ToContext.
func (ac *AppContext) SetURL(url string) *AppContext {
	ac.mu.Lock()
	ac.url = url
	ac.invalidate()
	ac.mu.Unlock()
	return ac
}

func (ac *AppContext) SetMethod(method string) *AppContext {
	ac.mu.Lock()
	ac.method = method
	ac.invalidate()
	ac.mu.Unlock()
	return ac
}

// SetPort menyimpan port secara lokal. Tidak dipropagasi ke context.
func (ac *AppContext) SetPort(port int) *AppContext {
	ac.mu.Lock()
	ac.port = port
	ac.mu.Unlock()
	return ac
}

// SetSrcIP menyimpan source IP secara lokal. Tidak dipropagasi ke context.
func (ac *AppContext) SetSrcIP(srcIP string) *AppContext {
	ac.mu.Lock()
	ac.srcIP = srcIP
	ac.mu.Unlock()
	return ac
}

// SetHeader menyimpan request header secara lokal. Tidak dipropagasi ke context.
func (ac *AppContext) SetHeader(header interface{}) *AppContext {
	ac.mu.Lock()
	ac.header = header
	ac.mu.Unlock()
	return ac
}

// SetRequest menyimpan request body secara lokal. Tidak dipropagasi ke context.
func (ac *AppContext) SetRequest(request interface{}) *AppContext {
	ac.mu.Lock()
	ac.request = request
	ac.mu.Unlock()
	return ac
}

// ── Context ───────────────────────────────────────────────────────────────────

// Log mengembalikan ContextualLogger yang sudah terikat ke context request saat ini.
func (ac *AppContext) Log() *Logger.ContextualLogger {
	return Logger.NewContextualLogger(ac.logger, ac.ToContext())
}

// ToContext membangun (dan meng-cache) context.Context yang memuat semua field yang dipropagasi.
// Context di-rebuild hanya ketika salah satu field propagasi berubah (dipanggil Set*).
// Port, SrcIP, Header, Request adalah local-only dan tidak pernah ditambahkan ke context.
func (ac *AppContext) ToContext() context.Context {
	// Fast path: sudah ada cache.
	ac.mu.RLock()
	if ac.cachedCtx != nil {
		ctx := ac.cachedCtx
		ac.mu.RUnlock()
		return ctx
	}
	ac.mu.RUnlock()

	// Slow path: bangun ulang dan cache.
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.cachedCtx != nil { // double-check setelah upgrade ke write lock
		return ac.cachedCtx
	}
	ac.cachedCtx = ac.buildContext()
	return ac.cachedCtx
}

// buildContext merakit context.Context dari field-field yang dipropagasi.
// Harus dipanggil dengan ac.mu dipegang untuk penulisan.
func (ac *AppContext) buildContext() context.Context {
	ctx := ac.parentCtx
	ctx = setContextIfNotEmpty(ctx, constant.RequestIDKey, ac.requestID)
	ctx = setContextIfNotEmpty(ctx, constant.TraceIDKey, ac.traceID)
	ctx = setContextIfNotEmpty(ctx, constant.UserIDKey, ac.userID)
	ctx = setContextIfNotZeroTime(ctx, constant.RequestTimeKey, ac.requestTime)
	ctx = setContextIfNotEmpty(ctx, constant.RequestMethodKey, ac.method)
	ctx = setContextIfNotEmpty(ctx, constant.RequestPathKey, ac.url)
	ctx = setContextIfNotEmpty(ctx, constant.RequestIPKey, ac.ip)
	ctx = setContextIfNotEmpty(ctx, constant.RequestAgentKey, ac.userAgent)
	return ctx
}

// ── Key-value store ───────────────────────────────────────────────────────────

// Get mengambil nilai dari request-scoped store.
// Mengembalikan defaultValue[0] jika key tidak ditemukan.
func (ac *AppContext) Get(key string, defaultValue ...interface{}) interface{} {
	if v, ok := ac.cMap.Load(key); ok {
		return v
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return nil
}

// Put menyimpan nilai ke request-scoped store.
func (ac *AppContext) Put(key string, data interface{}) {
	ac.cMap.Store(key, data)
}

// Remove menghapus key dari request-scoped store.
func (ac *AppContext) Remove(key string) {
	ac.cMap.Delete(key)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func setContextIfNotEmpty(ctx context.Context, key interface{}, value string) context.Context {
	if value != "" {
		return context.WithValue(ctx, key, value)
	}
	return ctx
}

func setContextIfNotZeroTime(ctx context.Context, key interface{}, value time.Time) context.Context {
	if !value.IsZero() {
		return context.WithValue(ctx, key, value)
	}
	return ctx
}
