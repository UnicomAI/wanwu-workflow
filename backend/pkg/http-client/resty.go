package http_client

import (
	"time"

	"github.com/go-resty/resty/v2"
)

// GetRestyClient returns a resty client with trace propagation enabled
func GetRestyClient() *resty.Client {
	client := resty.New()
	client.SetTransport(newHttpClient().Transport)
	return client
}

// GetRestyClientWithTimeout returns a resty client with trace propagation and custom timeout
func GetRestyClientWithTimeout(timeout time.Duration) *resty.Client {
	client := resty.New()
	client.SetTransport(newHttpClient().Transport)
	client.SetTimeout(timeout)
	return client
}
