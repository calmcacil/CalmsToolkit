package api

import (
	"net/http"
	"time"
)

// Client provides a shared HTTP client for API requests
type Client struct {
	httpClient *http.Client
	timeout    time.Duration
}

// NewClient creates a new API client with the specified timeout
func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		timeout: timeout,
	}
}

// HTTP returns the underlying HTTP client
func (c *Client) HTTP() *http.Client {
	return c.httpClient
}

// SetTimeout updates the client timeout
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
	c.httpClient.Timeout = timeout
}
