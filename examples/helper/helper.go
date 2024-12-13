package helper

import (
	"github.com/ewinjuman/go-lib/v2/logger"
	"sync"
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
