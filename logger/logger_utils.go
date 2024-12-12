package logger

import (
	"context"
	"fmt"
	"time"
)

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
