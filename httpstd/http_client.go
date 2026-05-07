package httpstd

import (
	"net/http"
	"time"
)

type reqClient struct {
	httpClient *http.Client
}

func newClient() *reqClient {
	return &reqClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}
