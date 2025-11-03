package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents a shared HTTP client for API calls
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	timeout    time.Duration
	userAgent  string
}

// NewClient creates a new API client
func NewClient(baseURL, token string, timeout time.Duration) *Client {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL:   baseURL,
		token:     token,
		timeout:   timeout,
		userAgent: "CalmsToolkit/1.0",
	}
}

// NewClientWithHTTPClient creates a new API client with a custom HTTP client
func NewClientWithHTTPClient(baseURL, token string, httpClient *http.Client) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		token:      token,
		userAgent:  "CalmsToolkit/1.0",
	}
}

// SetUserAgent sets the user agent for the client
func (c *Client) SetUserAgent(userAgent string) {
	c.userAgent = userAgent
}

// SetTimeout sets the timeout for the client
func (c *Client) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
	c.httpClient.Timeout = timeout
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, endpoint string) (*Response, error) {
	return c.Do(ctx, "GET", endpoint, nil)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, endpoint string, body interface{}) (*Response, error) {
	return c.Do(ctx, "POST", endpoint, body)
}

// Put performs a PUT request
func (c *Client) Put(ctx context.Context, endpoint string, body interface{}) (*Response, error) {
	return c.Do(ctx, "PUT", endpoint, body)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, endpoint string) (*Response, error) {
	return c.Do(ctx, "DELETE", endpoint, nil)
}

// Do performs a generic HTTP request
func (c *Client) Do(ctx context.Context, method, endpoint string, body interface{}) (*Response, error) {
	url := c.baseURL + endpoint

	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if c.token != "" {
		req.Header.Set("X-Api-Key", c.token)
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return &Response{
		Response: resp,
		client:   c,
	}, nil
}

// Response wraps an HTTP response with convenience methods
type Response struct {
	*http.Response
	client *Client
}

// JSON unmarshals the response body into the provided interface
func (r *Response) JSON(v interface{}) error {
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return nil
}

// Text returns the response body as a string
func (r *Response) Text() (string, error) {
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

// Bytes returns the response body as bytes
func (r *Response) Bytes() ([]byte, error) {
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// IsSuccess returns true if the response status code indicates success (2xx)
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// IsClientError returns true if the response status code indicates a client error (4xx)
func (r *Response) IsClientError() bool {
	return r.StatusCode >= 400 && r.StatusCode < 500
}

// IsServerError returns true if the response status code indicates a server error (5xx)
func (r *Response) IsServerError() bool {
	return r.StatusCode >= 500 && r.StatusCode < 600
}

// EnsureSuccess returns an error if the response is not successful
func (r *Response) EnsureSuccess() error {
	if r.IsSuccess() {
		return nil
	}

	// Try to get error details from response body
	var errorBody struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Details string `json:"details"`
	}

	if err := r.JSON(&errorBody); err != nil {
		// If we can't parse the error body, return a generic error
		return fmt.Errorf("request failed with status %d", r.StatusCode)
	}

	// Construct error message from available fields
	errorMsg := fmt.Sprintf("request failed with status %d", r.StatusCode)
	if errorBody.Error != "" {
		errorMsg += ": " + errorBody.Error
	} else if errorBody.Message != "" {
		errorMsg += ": " + errorBody.Message
	} else if errorBody.Details != "" {
		errorMsg += ": " + errorBody.Details
	}

	return fmt.Errorf("%s", errorMsg)
}

// APIError represents an API error response
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	ErrorMsg   string `json:"error"`
	Details    string `json:"details"`
}

func (e APIError) Error() string {
	if e.ErrorMsg != "" {
		return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.ErrorMsg)
	}
	if e.Message != "" {
		return fmt.Sprintf("API error (%d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("API error (%d)", e.StatusCode)
}

// TestConnection tests if the client can successfully connect to the API
func (c *Client) TestConnection(ctx context.Context) error {
	// Try a simple GET request to a common endpoint
	// Different services have different health check endpoints
	endpoints := []string{
		"/api/v1/auth/me", // Overseerr/Jellyseerr
		"/identity",       // Sonarr/Radarr
		"/system/status",  // Plex
		"/System/Ping",    // Jellyfin
		"/health",         // Generic health check
	}

	for _, endpoint := range endpoints {
		resp, err := c.Get(ctx, endpoint)
		if err != nil {
			continue // Try next endpoint
		}
		resp.Body.Close()

		// If we get any response (even 404), the connection works
		if resp.StatusCode < 500 {
			return nil
		}
	}

	return fmt.Errorf("unable to connect to API at %s", c.baseURL)
}

// GetBaseURL returns the base URL of the client
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// GetToken returns the authentication token of the client
func (c *Client) GetToken() string {
	return c.token
}

// GetTimeout returns the timeout of the client
func (c *Client) GetTimeout() time.Duration {
	return c.timeout
}
