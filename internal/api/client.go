package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client provides a unified HTTP client for API requests
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	timeout    time.Duration
}

// NewClient creates a new API client
func NewClient(baseURL, token string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		httpClient: &http.Client{Timeout: timeout},
		baseURL:    baseURL,
		token:      token,
		timeout:    timeout,
	}
}

// Get makes a GET request to the specified endpoint
func (c *Client) Get(endpoint string) (*http.Response, error) {
	return c.makeRequest("GET", endpoint, nil)
}

// Post makes a POST request to the specified endpoint
func (c *Client) Post(endpoint string, body interface{}) (*http.Response, error) {
	return c.makeRequest("POST", endpoint, body)
}

// Put makes a PUT request to the specified endpoint
func (c *Client) Put(endpoint string, body interface{}) (*http.Response, error) {
	return c.makeRequest("PUT", endpoint, body)
}

// Delete makes a DELETE request to the specified endpoint
func (c *Client) Delete(endpoint string) (*http.Response, error) {
	return c.makeRequest("DELETE", endpoint, nil)
}

// makeRequest is the internal method for making HTTP requests
func (c *Client) makeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	fullURL := c.baseURL + "/api/v1" + endpoint

	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("X-Api-Key", c.token)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	req.Header.Set("Accept", "application/json")

	return c.httpClient.Do(req)
}

// TestConnection tests the connection to the API
func (c *Client) TestConnection() error {
	resp, err := c.Get("/auth/me")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("invalid API key")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}
