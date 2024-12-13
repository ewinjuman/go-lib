package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"
)

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

func (l *Logger) LogRequest(ctx context.Context, url, method string, requestTime time.Time, headers http.Header, request interface{}, message ...interface{}) {
	msg := "request_started"
	if message != nil {
		msg = formatResponse(message...)
	}
	l.Info(ctx, msg,
		String("method", method),
		String("url", url),
		Interface("request", request),
		Interface("header", headers),
	)
}

func (l *Logger) LogResponse(ctx context.Context, url, method string, requestTime time.Time, response interface{}, message ...interface{}) {
	stop := time.Now()
	rt := stop.Sub(requestTime).Milliseconds()

	msg := "request_completed"
	if message != nil {
		msg = formatResponse(message...)
	}

	l.Info(ctx, msg,
		String("method", method),
		String("url", url),
		Interface("response", response),
		String("response_time", fmt.Sprintf("%d ms", rt)),
	)
}

func (l *Logger) LogRequestHttp(ctx context.Context, url string, method string, body interface{}, header interface{}, params interface{}) {
	l.Info(ctx, "request_http_started",
		String("method", method),
		String("url", url),
		Interface("request", body),
		Interface("params", params),
		Interface("header", header),
	)
}

func (l *Logger) LogResponseHttp(ctx context.Context, responseTime time.Duration, code int, url string, method string, body interface{}, err error) {
	if err != nil {
		l.Error(ctx, "request_http_completed",
			String("method", method),
			String("url", url),
			Int("http_status", code),
			Error(err),
			Interface("response", body),
			String("process_time", fmt.Sprintf("%d ms", responseTime.Milliseconds())),
		)
	} else {
		l.Info(ctx, "request_http_completed",
			String("method", method),
			String("url", url),
			Int("http_status", code),
			Interface("response", body),
			String("process_time", fmt.Sprintf("%d ms", responseTime.Milliseconds())),
		)
	}
}

func (l *Logger) LogRequestGrpc(ctx context.Context, url string, method string, body interface{}, header interface{}) {

	l.Info(ctx, "request_grpc_started",
		String("method", method),
		String("url", url),
		Interface("request", body),
		Interface("header", header),
	)
}

func (l *Logger) LogResponseGrpc(ctx context.Context, startProcessTime time.Time, url string, method string, body interface{}) {
	stop := time.Now()
	l.Info(ctx, "response_grpc_started",
		String("method", method),
		String("url", url),
		Interface("response", body),
		String("process_time", fmt.Sprintf("%d ms", stop.Sub(startProcessTime).Milliseconds())),
	)
}

func (l *Logger) LogDatabase(ctx context.Context, sql string, result interface{}, error interface{}) {
	l.Info(ctx, "",
		String("sql", sql),
		Interface("result", result),
		Interface("error", error),
	)
}
