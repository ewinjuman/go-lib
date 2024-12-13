package http_v2

import (
	"crypto/tls"
	"github.com/go-resty/resty/v2"
	"time"
)

type ReqClient struct {
	httpClient     *resty.Client
	circuitBreaker *CircuitBreaker
}

func httpclient() *ReqClient {
	httpClient := resty.New()
	httpClient.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})

	httpClient.SetTimeout(5 * time.Second)
	httpClient.SetDebug(false)
	return &ReqClient{
		httpClient:     httpClient,
		circuitBreaker: NewCircuitBreaker(),
	}
}
