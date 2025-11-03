package api

import (
	"fmt"
	"net/http"
	"time"
)

// Client represents a generic API client for Sonarr/Radarr
type Client struct {
	httpClient *http.Client
	BaseURL    string
	Token      string
	timeout    time.Duration
}

// NewClient creates a new API client
func NewClient(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		BaseURL:    baseURL,
		Token:      token,
		timeout:    timeout,
	}
}

// Do executes an HTTP request with proper headers
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("X-Api-Key", c.Token)
	req.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(req)
}

// Get performs a GET request to the given endpoint
func (c *Client) Get(endpoint string) (*http.Response, error) {
	url := c.BaseURL + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", c.BaseURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return resp, nil
}
