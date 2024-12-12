package http_v2

import (
	"crypto/tls"
	"github.com/go-resty/resty/v2"
	"time"
)

func httpclient() *resty.Client {
	httpClient := resty.New()
	httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	httpClient.SetTimeout(5 * time.Second)
	httpClient.SetDebug(false)
	return httpClient
}
