package http

import (
	"crypto/tls"
	"github.com/go-resty/resty/v2"
	"time"
)

type reqClient struct {
	httpClient     *resty.Client
	circuitBreaker *CircuitBreaker
}

func httpclient() *reqClient {
	httpClient := resty.New()
	httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	httpClient.SetTimeout(5 * time.Second)
	httpClient.SetDebug(false)
	return &reqClient{
		httpClient:     httpClient,
		circuitBreaker: NewCircuitBreaker(),
	}
}
