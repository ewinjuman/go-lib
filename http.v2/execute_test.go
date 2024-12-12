package http_v2

import (
	"github.com/ewinjuman/go-lib/v2/appContext"
	"github.com/ewinjuman/go-lib/v2/logger"
	"github.com/go-resty/resty/v2"
	"github.com/stretchr/testify/assert"
	"net/http"
	"sync"
	"testing"
)

var (
	// instance singleton dari logger
	instance *logger.Logger
	once     sync.Once
)

func InitLogger(opts logger.Options) {
	once.Do(func() {
		logger, err := logger.New(opts)
		if err != nil {
			panic(err)
		}
		instance = logger
	})
}

func GetLogger() *logger.Logger {
	option := logger.Options{
		AppName:     "myapp",
		Environment: "production",
		Stdout:      true,
		Filename:    "log/app.log",
		MaxSize:     100,
		MaxBackups:  7,
		MaxAge:      0,
		Level:       logger.InfoLevel,
		MaskingPaths: []string{
			"credit_card",
			"ssn",
			"Token",
			"authorization",
			"secret",
			"key",
			"access_token",
			"email",
		},
		RedactionPaths: []string{
			"pasSworD",
			"PiN",
		},
		EnableTrace: true,
		Development: false,
	}
	if instance == nil {
		// Default config jika belum diinisialisasi
		InitLogger(option)
	}
	return instance
}

func TestRequest_DoRequest(t *testing.T) {
	type fields struct {
		request Request
	}
	type args struct {
		httpClient *resty.Client
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		validator func(response *Response) bool
	}{
		{
			"Success",
			fields{request: Request{
				appContext:  appContext.New(GetLogger()),
				URL:         "http://localhost:3000/template",
				Method:      MethodGet,
				Body:        nil,
				File:        nil,
				PathParams:  nil,
				QueryParams: nil,
				Headers:     nil,
				Context:     nil,
				Timeout:     0,
				DebugMode:   false,
				SkipTLS:     false,
			}},
			args{httpClient: httpclient()},
			func(response *Response) bool {
				assert.NotEmpty(t, response.Body)
				assert.Empty(t, response.Error)
				assert.Equal(t, http.StatusOK, response.StatusCode)
				return true
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotResponse := tt.fields.request.DoRequest(tt.args.httpClient); !tt.validator(gotResponse) {
				t.Errorf("DoRequest() = %v", gotResponse)
			}
		})
	}
}
