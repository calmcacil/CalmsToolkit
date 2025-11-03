package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
	timeout    time.Duration
}

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

func (c *Client) Get(endpoint string) (*http.Response, error) {
	return c.MakeRequest("GET", endpoint, nil)
}

func (c *Client) Post(endpoint string, body interface{}) (*http.Response, error) {
	return c.MakeRequest("POST", endpoint, body)
}

func (c *Client) Put(endpoint string, body interface{}) (*http.Response, error) {
	return c.MakeRequest("PUT", endpoint, body)
}

func (c *Client) Delete(endpoint string) (*http.Response, error) {
	return c.MakeRequest("DELETE", endpoint, nil)
}

func (c *Client) MakeRequest(method, endpoint string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = strings.NewReader(string(jsonBody))
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("X-Api-Key", c.token)
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

func (c *Client) TestConnection() error {
	resp, err := c.Get("/api/v1/auth/me")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("connection test failed with status: %d", resp.StatusCode)
	}

	return nil
}
