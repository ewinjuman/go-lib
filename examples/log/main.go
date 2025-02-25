package main

import (
	"fmt"
	"github.com/ewinjuman/go-lib/v2/appContext"
	"github.com/ewinjuman/go-lib/v2/logger"
	"sync"
	"time"
)

var (
	// instance singleton dari logger
	instance *logger.Logger
	once     sync.Once
)

// InitLogger inisialisasi logger sekali saja
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
		Write:       true,
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
func main() {
	user := struct {
		ID       string `json:"id"`
		Email    string `json:"Email"`
		Token    string `json:"Token"`
		Password string `json:"password"`
	}{
		ID:       "123",
		Email:    "usAer@exAmple.Com",
		Token:    "902jdsaljldsjaldjlasjdlsdjlasjdasdlasjdlajdsadljaslnvnbvkasdjasjdakd;askd;kas;dka;k;930230",
		Password: "password",
	}

	start := time.Now()
	appCtx := appContext.New(GetLogger())
	//appCtx.SetRequestID("requestID") // set if needed
	appCtx.LogInfo("Start", logger.String("user", "kamu"), logger.String("token", "udhs908711"))
	appCtx.LogInfo("print struct", logger.Interface("user", user))
	appCtx.Log().Info(appCtx.ToContext(), "masking", logger.String("token", "12345789"), logger.String("Email", "user@example.com"))
	appCtx.Log().Info(appCtx.ToContext(), "redaction", logger.String("pin", "123456"), logger.String("Email", "user@example.com"))
	stop := time.Now()
	println(fmt.Sprintf("%d ms", stop.Sub(start).Milliseconds()))
}
