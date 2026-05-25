package main

import (
	"context"
	"fmt"
	"time"

	"github.com/ewinjuman/go-lib/v2/appContext"
	"github.com/ewinjuman/go-lib/v2/logger"
)

// getLogger mengembalikan global singleton logger via logger.GetLogger().
// Jika ingin konfigurasi kustom, panggil logger.New() terlebih dulu sebelum getLogger().
func getLogger() *logger.Logger {
	return logger.GetLogger()
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
	log := getLogger()
	appCtx := appContext.New(context.Background(), log)
	defer log.Shutdown()

	appCtx.Log().Info("Start", logger.String("user", "kamu"), logger.String("token", "udhs908711"))
	appCtx.Log().Info("print struct", logger.Interface("user", user))
	appCtx.Log().Info("masking", logger.String("token", "12345789"), logger.String("Email", "user@example.com"))
	appCtx.Log().Info("redaction", logger.String("pin", "123456"), logger.String("Email", "user@example.com"))
	stop := time.Now()
	println(fmt.Sprintf("%d ms", stop.Sub(start).Milliseconds()))
}
