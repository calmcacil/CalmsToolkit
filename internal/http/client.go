package http

import (
	"net/http"
	"time"
)

type Client struct {
	*http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		Client: &http.Client{
			Timeout: timeout,
		},
	}
}

func NewTransportClient(timeout time.Duration) *Client {
	return &Client{
		Client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  false,
			},
		},
	}
}
