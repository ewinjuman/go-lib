package sessionX

import (
	"context"
	"fmt"
	Logger "github.com/ewinjuman/go-lib/v2/logger"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	JsonIter "github.com/json-iterator/go"
	Map "github.com/orcaman/concurrent-map"
)

const (
	AppSession = "AppSession"
	ThreadId   = "ThreadId"
)

type Session struct {
	InstitutionID           string
	Context                 context.Context
	Map                     Map.ConcurrentMap
	Logger                  *Logger.Logger
	RequestTime             time.Time
	ThreadID                string
	PersonalId              string
	AppName, AppVersion, IP string
	Port                    int
	SrcIP, URL, Method      string
	Header, Request         interface{}
	ErrorMessage            string
}

func New(ctx context.Context, logger *Logger.Logger) *Session {
	sessionID := strconv.Itoa(int(time.Now().UnixNano() / int64(time.Microsecond)))
	session := &Session{
		RequestTime: time.Now(),
		Logger:      logger,
		Map:         Map.New(),
		Context:     ctx,
	}
	session.ThreadID = sessionID
	return session
}

func (session *Session) SetThreadID(sessionID string) *Session {
	session.ThreadID = sessionID
	return session
}

func (session *Session) SetMethod(method string) *Session {
	session.Method = method
	return session
}

func (session *Session) SetAppName(appName string) *Session {
	session.AppName = appName
	return session
}

func (session *Session) SetAppVersion(appVersion string) *Session {
	session.AppVersion = appVersion
	return session
}

func (session *Session) SetURL(url string) *Session {
	session.URL = url
	return session
}

func (session *Session) SetIP(ip string) *Session {
	session.IP = ip
	return session
}

func (session *Session) SetPort(port int) *Session {
	session.Port = port
	return session
}

func (session *Session) SetSrcIP(srcIp string) *Session {
	session.SrcIP = srcIp
	return session
}

func (session *Session) SetHeader(header interface{}) *Session {
	session.Header = header
	return session
}

func (session *Session) SetRequest(request interface{}) *Session {
	session.Request = request
	return session
}

func (session *Session) SetErrorMessage(errorMessage string) *Session {
	session.ErrorMessage = errorMessage
	return session
}

func (session *Session) SetInstitutionID(institutionID string) *Session {
	session.InstitutionID = institutionID
	return session
}

func (session *Session) SetPersonalIdentifier(phone string) *Session {
	session.PersonalId = phone
	return session
}

func (session *Session) Get(key string, defaultValue ...interface{}) (data interface{}) {
	data, ok := session.Map.Get(key)
	if !ok {
		if len(defaultValue) > 0 {
			return defaultValue[0]
		}
	}
	return
}

func (session *Session) Put(key string, data interface{}) {
	session.Map.Set(key, data)
}

func (session *Session) LogDatabase(sql string, result interface{}, error interface{}) {
	session.Logger.Info(session.Context, "",
		Logger.String("sql", sql),
		Logger.Interface("result", result),
		Logger.Interface("error", error),
	)
}

func (session *Session) LogRequest(message ...interface{}) {
	msg := "request_started"
	if message != nil {
		msg = formatResponse(message...)
	}
	session.Logger.Info(session.Context, msg,
		Logger.String("method", session.Method),
		Logger.String("url", session.URL),
		Logger.Interface("request", session.Request),
		Logger.Interface("header", session.Header),
	)
}

func (session *Session) LogResponse(response interface{}, message ...interface{}) {
	stop := time.Now()
	rt := stop.Sub(session.RequestTime).Milliseconds()

	msg := "request_completed"
	if message != nil {
		msg = formatResponse(message...)
	}

	session.Logger.Info(session.Context, msg,
		Logger.String("method", session.Method),
		Logger.String("url", session.URL),
		Logger.Interface("response", response),
		Logger.String("response_time", fmt.Sprintf("%d ms", rt)),
	)
}

func (session *Session) LogRequestHttp(url string, method string, body interface{}, header interface{}, params interface{}) {
	session.Logger.Info(session.Context, "request_http_started",
		Logger.String("method", method),
		Logger.String("url", url),
		Logger.Interface("request", body),
		Logger.Interface("params", params),
		Logger.Interface("header", header),
	)
}

func (session *Session) LogResponseHttp(responseTime time.Duration, code int, url string, method string, body interface{}, err error) {
	if err != nil {
		session.Logger.Error(session.Context, "request_http_completed",
			Logger.String("method", method),
			Logger.String("url", url),
			Logger.Int("http_status", code),
			Logger.Error(err),
			Logger.Interface("response", body),
			Logger.String("process_time", fmt.Sprintf("%d ms", responseTime.Milliseconds())),
		)
	} else {
		session.Logger.Info(session.Context, "request_http_completed",
			Logger.String("method", method),
			Logger.String("url", url),
			Logger.Int("http_status", code),
			Logger.Interface("response", body),
			Logger.String("process_time", fmt.Sprintf("%d ms", responseTime.Milliseconds())),
		)
	}
}

func (session *Session) LogRequestGrpc(url string, method string, body interface{}, header interface{}) {

	session.Logger.Info(session.Context, "request_grpc_started",
		Logger.String("method", method),
		Logger.String("url", url),
		Logger.Interface("request", body),
		Logger.Interface("header", header),
	)
}

func (session *Session) LogResponseGrpc(startProcessTime time.Time, url string, method string, body interface{}) {
	stop := time.Now()
	session.Logger.Info(session.Context, "response_grpc_started",
		Logger.String("method", method),
		Logger.String("url", url),
		Logger.Interface("response", body),
		Logger.String("process_time", fmt.Sprintf("%d ms", stop.Sub(startProcessTime).Milliseconds())),
	)
}

func (session *Session) LogMessage(message interface{}, data interface{}) {
	session.Logger.Info(session.Context, message.(string),
		Logger.String("request_id", session.ThreadID),
		Logger.Interface("message", message),
		Logger.Interface("data", data),
	)
}

var json = JsonIter.ConfigCompatibleWithStandardLibrary

func formatResponse(message ...interface{}) string {
	sb := strings.Builder{}

	for _, msg := range message {
		var m []byte
		if reflect.ValueOf(msg).Kind().String() == "string" {
			m = []byte(msg.(string))
		} else {
			m, _ = json.Marshal(msg)
		}

		sb.Write(m)
	}

	return sb.String()
}

// GetSession For Fiber
func GetSession(c *fiber.Ctx) *Session {
	return c.Locals(AppSession).(*Session)
}
