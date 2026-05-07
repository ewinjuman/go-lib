package http

import (
	"github.com/go-resty/resty/v2"
	"time"
)

type reqClient struct {
	httpClient *resty.Client
}

func httpclient() *reqClient {
	httpClient := resty.New()
	httpClient.SetTimeout(30 * time.Second)
	httpClient.SetDebug(false)
	return &reqClient{
		httpClient: httpClient,
	}
}
