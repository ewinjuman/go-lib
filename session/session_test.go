package session

import (
	"context"
	"github.com/pkg/errors"
	"reflect"
	"sync"
	"testing"
	"time"

	Logger "github.com/ewinjuman/go-lib/v2/logger"
	cmap "github.com/orcaman/concurrent-map"
)

var (
	// instance singleton dari logger
	instance *Logger.Logger
	once     sync.Once
)

// InitLogger inisialisasi logger sekali saja
func InitLogger(opts Logger.Options) {
	once.Do(func() {
		logger, err := Logger.New(opts)
		if err != nil {
			panic(err)
		}
		instance = logger
	})
}

func GetLogger() *Logger.Logger {
	if instance == nil {
		// Default config jika belum diinisialisasi
		InitLogger(Logger.DefaultOptions())
	}
	return instance
}
func TestNew(t *testing.T) {
	type args struct {
		logger *Logger.Logger
		ctx    context.Context
	}
	tests := []struct {
		name string
		args args
		want *Session
	}{
		{
			"try 1",
			args{logger: GetLogger(), ctx: Logger.NewContext()},
			&Session{},
		},
		{
			"try 2",
			args{logger: GetLogger(), ctx: Logger.NewContext()},
			&Session{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := New(tt.args.ctx, tt.args.logger); got == nil {
				t.Errorf("New() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSession_SetThreadID(t *testing.T) {
	type fields struct {
		Logger   *Logger.Logger
		ThreadID string
	}
	type args struct {
		sessionID string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set thread id",
			fields{
				Logger:   Logger.GetLogger(),
				ThreadID: "12345",
			},
			args{sessionID: "s12345"},
			&Session{ThreadID: "s12345"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Logger:   tt.fields.Logger,
				ThreadID: tt.fields.ThreadID,
			}
			if got := session.SetThreadID(tt.args.sessionID); !reflect.DeepEqual(got.ThreadID, tt.want.ThreadID) {
				t.Errorf("SetThreadID() = %v, want %v", got.ThreadID, tt.want.ThreadID)
			}
		})
	}
}

func TestSession_SetMethod(t *testing.T) {
	type fields struct {
		Method string
	}
	type args struct {
		method string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set thread id",
			fields{
				Method: "POST",
			},
			args{method: "GET"},
			&Session{Method: "GET"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Method: tt.fields.Method,
			}
			if got := session.SetMethod(tt.args.method); !reflect.DeepEqual(got.Method, tt.want.Method) {
				t.Errorf("SetMethod() = %v, want %v", got.Method, tt.want.Method)
			}
		})
	}
}

func TestSession_SetAppName(t *testing.T) {
	type fields struct {
		AppName string
	}
	type args struct {
		appName string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set app name",
			fields{
				AppName: "Otto",
			},
			args{appName: "OttoApp"},
			&Session{AppName: "OttoApp"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				AppName: tt.fields.AppName,
			}
			if got := session.SetAppName(tt.args.appName); !reflect.DeepEqual(got.AppName, tt.want.AppName) {
				t.Errorf("SetAppName() = %v, want %v", got.AppName, tt.want.AppName)
			}
		})
	}
}

func TestSession_SetAppVersion(t *testing.T) {
	type fields struct {
		AppVersion string
	}
	type args struct {
		appVersion string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set app version",
			fields{
				AppVersion: "1.1.1",
			},
			args{appVersion: "1.2.3"},
			&Session{AppVersion: "1.2.3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				AppVersion: tt.fields.AppVersion,
			}
			if got := session.SetAppVersion(tt.args.appVersion); !reflect.DeepEqual(got.AppVersion, tt.want.AppVersion) {
				t.Errorf("SetAppVersion() = %v, want %v", got.AppVersion, tt.want.AppVersion)
			}
		})
	}
}

func TestSession_SetURL(t *testing.T) {
	type fields struct {
		URL string
	}
	type args struct {
		url string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set url",
			fields{
				URL: "https://example.com",
			},
			args{url: "https://realX.id"},
			&Session{URL: "https://realX.id"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				URL: tt.fields.URL,
			}
			if got := session.SetURL(tt.args.url); !reflect.DeepEqual(got.URL, tt.want.URL) {
				t.Errorf("SetURL() = %v, want %v", got.URL, tt.want.URL)
			}
		})
	}
}

func TestSession_SetIP(t *testing.T) {
	type fields struct {
		IP string
	}
	type args struct {
		ip string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set IP",
			fields{
				IP: "127.0.0.1",
			},
			args{ip: "123.32.2.33"},
			&Session{IP: "123.32.2.33"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				IP: tt.fields.IP,
			}
			if got := session.SetIP(tt.args.ip); !reflect.DeepEqual(got.IP, tt.want.IP) {
				t.Errorf("SetIP() = %v, want %v", got.IP, tt.want.IP)
			}
		})
	}
}

func TestSession_SetPort(t *testing.T) {
	type fields struct {
		Port int
	}
	type args struct {
		port int
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set Port",
			fields{
				Port: 8888,
			},
			args{port: 9292},
			&Session{Port: 9292},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Port: tt.fields.Port,
			}
			if got := session.SetPort(tt.args.port); !reflect.DeepEqual(got.Port, tt.want.Port) {
				t.Errorf("SetPort() = %v, want %v", got.Port, tt.want.Port)
			}
		})
	}
}

func TestSession_SetSrcIP(t *testing.T) {
	type fields struct {
		SrcIP string
	}
	type args struct {
		srcIp string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set Src IP",
			fields{
				SrcIP: "127.0.0.1",
			},
			args{srcIp: "12.23.45.678"},
			&Session{SrcIP: "12.23.45.678"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				SrcIP: tt.fields.SrcIP,
			}
			if got := session.SetSrcIP(tt.args.srcIp); !reflect.DeepEqual(got.SrcIP, tt.want.SrcIP) {
				t.Errorf("SetSrcIP() = %v, want %v", got.SrcIP, tt.want.SrcIP)
			}
		})
	}
}

func TestSession_SetHeader(t *testing.T) {
	type fields struct {
		Header interface{}
	}
	type args struct {
		header interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set Header",
			fields{
				Header: "RequestID:12345",
			},
			args{header: "RequestID:9876"},
			&Session{Header: "RequestID:9876"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Header: tt.fields.Header,
			}
			if got := session.SetHeader(tt.args.header); !reflect.DeepEqual(got.Header, tt.want.Header) {
				t.Errorf("SetHeader() = %v, want %v", got.Header, tt.want.Header)
			}
		})
	}
}

func TestSession_SetRequest(t *testing.T) {
	type exReq struct {
		Id int `json:"id"`
	}
	type fields struct {
		Request interface{}
	}
	type args struct {
		request interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"Set Request",
			fields{Request: exReq{Id: 1}},
			args{request: exReq{Id: 22}},
			&Session{Request: exReq{Id: 22}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Request: tt.fields.Request,
			}
			if got := session.SetRequest(tt.args.request); !reflect.DeepEqual(got.Request, tt.want.Request) {
				t.Errorf("SetRequest() = %v, want %v", got.Request, tt.want.Request)
			}
		})
	}
}

func TestSession_SetErrorMessage(t *testing.T) {
	type fields struct {
		ErrorMessage string
	}
	type args struct {
		errorMessage string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set Error Message",
			fields{
				ErrorMessage: "gagal",
			},
			args{errorMessage: "error data"},
			&Session{ErrorMessage: "error data"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				ErrorMessage: tt.fields.ErrorMessage,
			}
			if got := session.SetErrorMessage(tt.args.errorMessage); !reflect.DeepEqual(got.ErrorMessage, tt.want.ErrorMessage) {
				t.Errorf("SetErrorMessage() = %v, want %v", got.ErrorMessage, tt.want.ErrorMessage)
			}
		})
	}
}

func TestSession_SetInstitutionID(t *testing.T) {
	type fields struct {
		InstitutionID string
	}
	type args struct {
		institutionID string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set Institution ID",
			fields{
				InstitutionID: "ID123",
			},
			args{institutionID: "ID09876"},
			&Session{InstitutionID: "ID09876"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				InstitutionID: tt.fields.InstitutionID,
			}
			if got := session.SetInstitutionID(tt.args.institutionID); !reflect.DeepEqual(got.InstitutionID, tt.want.InstitutionID) {
				t.Errorf("SetInstitutionID() = %v, want %v", got.InstitutionID, tt.want.InstitutionID)
			}
		})
	}
}

func TestSession_SetPersonalIdentifier(t *testing.T) {
	type fields struct {
		PersonalId string
	}
	type args struct {
		phone string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *Session
	}{
		{"set Personal ID",
			fields{
				PersonalId: "0812878039",
			},
			args{phone: "0819891892"},
			&Session{PersonalId: "0819891892"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				PersonalId: tt.fields.PersonalId,
			}
			if got := session.SetPersonalIdentifier(tt.args.phone); !reflect.DeepEqual(got.PersonalId, tt.want.PersonalId) {
				t.Errorf("SetPersonalIdentifier() = %v, want %v", got.PersonalId, tt.want.PersonalId)
			}
		})
	}
}

func TestSession_Get(t *testing.T) {
	type fields struct {
		Map cmap.ConcurrentMap
	}
	type args struct {
		getKey       string
		key          string
		value        string
		defaultValue interface{}
	}
	tests := []struct {
		name     string
		fields   fields
		args     args
		wantData interface{}
		wantErr  bool
	}{
		{
			"get from map",
			fields{Map: cmap.New()},
			args{getKey: "name", key: "name", value: "Ewin", defaultValue: "Ewin"},
			"Ewin",
			false,
		},
		{
			"get from map and default",
			fields{Map: cmap.New()},
			args{getKey: "id", key: "name", value: "Ewin", defaultValue: "Ewin"},
			"Ewin",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.fields.Map.Set(tt.args.key, tt.args.value)
			session := &Session{
				Map: tt.fields.Map,
			}
			gotData := session.Get(tt.args.getKey, tt.args.defaultValue)
			if !reflect.DeepEqual(gotData, tt.wantData) {
				t.Errorf("Get() gotData = %v, want %v", gotData, tt.wantData)
			}
		})
	}
}

func TestSession_Put(t *testing.T) {
	type fields struct {
		Map cmap.ConcurrentMap
		ctx context.Context
	}
	type args struct {
		key  string
		data interface{}
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"Put to Map",
			fields{Map: cmap.New(), ctx: Logger.NewContext()},
			args{key: "name", data: "Ewin"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Map:     tt.fields.Map,
				Context: tt.fields.ctx,
			}
			session.Put(tt.args.key, tt.args.data)
		})
	}
}

func TestSession_LogRequest(t *testing.T) {
	type fields struct {
		Logger  *Logger.Logger
		ctx     context.Context
		Request interface{}
	}
	type args struct {
		message []interface{}
	}
	type request struct {
		Id int `json:"id"`
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"Write logger Request",
			fields{
				Logger:  GetLogger(),
				ctx:     Logger.NewContext(),
				Request: request{Id: 12},
			},
			args{message: []interface{}{"logger Request"}},
		},
		{
			"Write logger Request nil",
			fields{
				Logger: GetLogger(), ctx: Logger.NewContext(),
			},
			args{message: []interface{}{"logger Request"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Logger:  tt.fields.Logger,
				Context: tt.fields.ctx,
				Request: tt.fields.Request,
			}
			session.LogRequest(tt.args.message...)
		})
	}
}

func TestSession_LogResponse(t *testing.T) {
	type fields struct {
		Logger *Logger.Logger
		ctx    context.Context
	}
	type args struct {
		response interface{}
		message  []interface{}
	}
	type response struct {
		Id string `json:"id"`
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"Write logger Response nil",
			fields{
				Logger: GetLogger(),
				ctx:    Logger.NewContext(),
			},
			args{message: []interface{}{"logger Response"}},
		},
		{
			"Write logger Response not nil",
			fields{
				Logger: GetLogger(),
				ctx:    Logger.NewContext(),
			},
			args{response: response{Id: "123"}, message: []interface{}{"logger Response"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Logger:  tt.fields.Logger,
				Context: tt.fields.ctx,
			}
			session.LogResponse(tt.args.response, tt.args.message...)
		})
	}
}

func TestSession_LogRequestHttp(t *testing.T) {
	type fields struct {
		Logger  *Logger.Logger
		ctx     context.Context
		Request interface{}
	}
	type args struct {
		url    string
		method string
		body   interface{}
		header interface{}
		params interface{}
	}
	type request struct {
		Id int `json:"id"`
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"Write logger Request HTTP",
			fields{
				Logger:  GetLogger(),
				ctx:     Logger.NewContext(),
				Request: request{Id: 12},
			},
			args{
				url:    "http://gg.com",
				method: "GET",
				body:   request{Id: 13},
			},
		},
		{
			"Write logger Request HTTP nil",
			fields{
				Logger:  GetLogger(),
				ctx:     Logger.NewContext(),
				Request: request{Id: 12},
			},
			args{
				url:    "http://gg.com",
				method: "GET",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Logger:  tt.fields.Logger,
				Context: tt.fields.ctx,
				Request: tt.fields.Request,
			}
			session.LogRequestHttp(tt.args.url, tt.args.method, tt.args.body, tt.args.header, tt.args.params)
		})
	}
}

func TestSession_LogResponseHttp(t *testing.T) {
	type fields struct {
		Logger       *Logger.Logger
		ctx          context.Context
		ErrorMessage string
	}
	type args struct {
		responseTime time.Duration
		code         int
		url          string
		method       string
		body         interface{}
		messageError error
	}
	type request struct {
		Id int `json:"id"`
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"Write logger Response Http",
			fields{
				Logger:       GetLogger(),
				ctx:          Logger.NewContext(),
				ErrorMessage: "",
			},
			args{
				url:          "http://v.com",
				method:       "GET",
				body:         request{Id: 123},
				messageError: nil,
			},
		},
		{
			"Write logger Response Http body nil",
			fields{
				Logger:       GetLogger(),
				ctx:          Logger.NewContext(),
				ErrorMessage: "",
			},
			args{
				url:          "http://v.com",
				method:       "GET",
				messageError: nil,
			},
		},
		{
			"Write logger Response Http message error not nil",
			fields{
				Logger:       GetLogger(),
				ctx:          Logger.NewContext(),
				ErrorMessage: "",
			},
			args{
				url:          "http://v.com",
				method:       "GET",
				body:         request{Id: 123},
				messageError: errors.New("test error"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Logger:       tt.fields.Logger,
				Context:      tt.fields.ctx,
				ErrorMessage: tt.fields.ErrorMessage,
			}
			session.LogResponseHttp(tt.args.responseTime, tt.args.code, tt.args.url, tt.args.method, tt.args.body, tt.args.messageError)
		})
	}
}

func TestSession_LogRequestGrpc(t *testing.T) {
	type fields struct {
		Logger  *Logger.Logger
		ctx     context.Context
		Request interface{}
	}
	type args struct {
		url    string
		method string
		body   interface{}
		header interface{}
		params interface{}
	}
	type request struct {
		Id int `json:"id"`
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"Write logger Request GRPC",
			fields{
				Logger:  GetLogger(),
				ctx:     Logger.NewContext(),
				Request: request{Id: 12},
			},
			args{
				url:    "/user.User/TokenValidation",
				method: "GRPC",
				body:   request{Id: 13},
			},
		},
		{
			"Write logger Request GRPC nil",
			fields{
				Logger:  GetLogger(),
				ctx:     Logger.NewContext(),
				Request: request{Id: 12},
			},
			args{
				url:    "/user.User/TokenValidation",
				method: "GRPC",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Logger:  tt.fields.Logger,
				Context: tt.fields.ctx,
				Request: tt.fields.Request,
			}
			session.LogRequestGrpc(tt.args.url, tt.args.method, tt.args.body, tt.args.header)
		})
	}
}

func TestSession_LogResponseGrpc(t *testing.T) {
	type fields struct {
		Logger       *Logger.Logger
		ctx          context.Context
		ErrorMessage string
	}
	type args struct {
		startProcessTime time.Time
		code             int
		url              string
		method           string
		body             interface{}
		messageError     []string
	}
	type request struct {
		Id int `json:"id"`
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			"Write logger Response GRPC",
			fields{
				Logger:       GetLogger(),
				ctx:          Logger.NewContext(),
				ErrorMessage: "",
			},
			args{
				url:          "/user.User/TokenValidation",
				method:       "GRPC",
				body:         request{Id: 123},
				messageError: nil,
			},
		},
		{
			"Write logger Response GRPC body nil",
			fields{
				Logger:       GetLogger(),
				ctx:          Logger.NewContext(),
				ErrorMessage: "",
			},
			args{
				url:          "/user.User/TokenValidation",
				method:       "GRPC",
				messageError: nil,
			},
		},
		{
			"Write logger Response GRPC message error not nil",
			fields{
				Logger:       GetLogger(),
				ctx:          Logger.NewContext(),
				ErrorMessage: "",
			},
			args{
				url:          "/user.User/TokenValidation",
				method:       "GRPC",
				body:         request{Id: 123},
				messageError: []string{"Error"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			session := &Session{
				Logger:       tt.fields.Logger,
				Context:      tt.fields.ctx,
				ErrorMessage: tt.fields.ErrorMessage,
			}
			session.LogResponseGrpc(tt.args.startProcessTime, tt.args.url, tt.args.method, tt.args.body)
		})
	}
}
