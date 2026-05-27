// Package http provides shared HTTP client utilities for API fetchers.
package http

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a reusable HTTP client wrapper around http.Client.
type Client struct {
	*http.Client
}

// NewClient creates a new Client with the given timeout.
func NewClient(timeout time.Duration) *Client {
	return &Client{
		Client: &http.Client{
			Timeout: timeout,
		},
	}
}

// NewTransportClient creates a new Client with a tuned transport (connection pooling).
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

// DoRequest performs an HTTP request and returns the raw body bytes and HTTP status code.
func (c *Client) DoRequest(ctx context.Context, method, url string, headers map[string]string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// DoJSON performs an HTTP request and decodes the JSON response into result.
func (c *Client) DoJSON(ctx context.Context, method, url string, headers map[string]string, body io.Reader, result interface{}) error {
	data, status, err := c.DoRequest(ctx, method, url, headers, body)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("unexpected status %d: %s", status, strings.TrimSpace(string(data)))
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("json decode: %w", err)
		}
	}
	return nil
}

// DoXML performs an HTTP request and decodes the XML response into result.
func (c *Client) DoXML(ctx context.Context, method, url string, headers map[string]string, body io.Reader, result interface{}) error {
	data, status, err := c.DoRequest(ctx, method, url, headers, body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", status, strings.TrimSpace(string(data)))
	}
	if result != nil {
		if err := xml.Unmarshal(data, result); err != nil {
			return fmt.Errorf("xml decode: %w", err)
		}
	}
	return nil
}
