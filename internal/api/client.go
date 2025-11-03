package api

import (
	"bytes"
	"fmt"
	"net/http"
	"time"
)

// Client represents a generic API client
type Client struct {
	httpClient *http.Client
	baseURL    string
	Token      string
	timeout    time.Duration
}

// NewClient creates a new API client
func NewClient(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
		Token:   token,
		timeout: timeout,
	}
}

// Do executes an HTTP request with proper headers
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.Token != "" {
		req.Header.Set("X-Api-Key", c.Token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CalmsToolkit/1.0")

	return c.httpClient.Do(req)
}

// Get creates and executes a GET request
func (c *Client) Get(endpoint string) (*http.Response, error) {
	url := c.baseURL + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}

	return c.Do(req)
}

// Post creates and executes a POST request
func (c *Client) Post(endpoint string, body []byte) (*http.Response, error) {
	url := c.baseURL + endpoint
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}

	if body != nil {
		req.Body = nopCloser{Reader: bytes.NewReader(body)}
	}

	return c.Do(req)
}

// Delete creates and executes a DELETE request
func (c *Client) Delete(endpoint string) (*http.Response, error) {
	url := c.baseURL + endpoint
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create DELETE request: %w", err)
	}

	return c.Do(req)
}

// nopCloser is used to convert a Reader to a ReadCloser
type nopCloser struct {
	*bytes.Reader
}

func (nopCloser) Close() error { return nil }
