package constant

// contextKey untuk menyimpan values di appContext
type ContextKey string

const (
	AppContextKey     ContextKey = "appContext"
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
