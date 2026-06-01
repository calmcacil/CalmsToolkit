// Package httputil provides shared HTTP client utilities for API fetchers.
package httputil

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Client is a reusable HTTP client wrapper around http.Client.
type Client struct {
	*http.Client
	MaxBodySize int64
}

// NewClient creates a new Client with the given timeout.
func NewClient(timeout time.Duration) *Client {
	return &Client{
		Client: &http.Client{
			Timeout: timeout,
		},
		MaxBodySize: 10 * 1024 * 1024,
	}
}

// NewTransportClient creates a new Client with a tuned transport (connection pooling).
func NewTransportClient(timeout time.Duration) *Client {
	return &Client{
		Client: &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:       20,
				IdleConnTimeout:    30 * time.Second,
				DisableCompression: false,
			},
		},
	}
}

// RetryConfig controls retry behaviour for DoJSONWithRetry and DoXMLWithRetry.
type RetryConfig struct {
	MaxAttempts       int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	RespectRetryAfter bool
	RetryStatuses     []int
}

func DefaultRetry() RetryConfig {
	return RetryConfig{
		MaxAttempts:       3,
		InitialBackoff:    500 * time.Millisecond,
		MaxBackoff:        5 * time.Second,
		RespectRetryAfter: true,
		RetryStatuses:     []int{http.StatusTooManyRequests, 502, 503, 504},
	}
}

func (rc RetryConfig) shouldRetry(status int) bool {
	for _, s := range rc.RetryStatuses {
		if status == s {
			return true
		}
	}
	return false
}

func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	return d + time.Duration(rand.Int63n(int64(d/4)))
}

// DoRequest performs an HTTP request and returns the raw body bytes and HTTP status code.
func (c *Client) DoRequest(ctx context.Context, method, url string, headers map[string]string, body io.Reader) ([]byte, int, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, 0, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, 0, 0, err
	}
	defer resp.Body.Close()

	retryAfter := parseRetryAfterHeader(resp)

	maxSize := c.MaxBodySize
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return nil, resp.StatusCode, retryAfter, err
	}
	if int64(len(data)) > maxSize {
		return nil, resp.StatusCode, retryAfter, fmt.Errorf("response body too large (exceeds %d bytes)", maxSize)
	}
	return data, resp.StatusCode, retryAfter, nil
}

func parseRetryAfterHeader(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	h := resp.Header.Get("Retry-After")
	if h == "" {
		return 0
	}
	if s, err := strconv.Atoi(h); err == nil && s > 0 {
		return time.Duration(s) * time.Second
	}
	if t, err := time.Parse(time.RFC1123, h); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// DoJSON performs an HTTP request and decodes the JSON response into result.
func (c *Client) DoJSON(ctx context.Context, method, url string, headers map[string]string, body io.Reader, result interface{}) error {
	data, status, _, err := c.DoRequest(ctx, method, url, headers, body)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("unexpected status %d: %s", status, truncateBody(strings.TrimSpace(string(data))))
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
	data, status, _, err := c.DoRequest(ctx, method, url, headers, body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("unexpected status %d: %s", status, truncateBody(strings.TrimSpace(string(data))))
	}
	if result != nil {
		if err := xml.Unmarshal(data, result); err != nil {
			return fmt.Errorf("xml decode: %w", err)
		}
	}
	return nil
}

// DoJSONWithRetry calls DoJSON with retry logic for transient failures.
func (c *Client) DoJSONWithRetry(ctx context.Context, method, url string, headers map[string]string, body io.Reader, result interface{}, rc RetryConfig) error {
	return c.doWithRetry(ctx, method, url, headers, body, func(data []byte, status int) error {
		if status != http.StatusOK && status != http.StatusCreated {
			return fmt.Errorf("unexpected status %d: %s", status, truncateBody(string(data)))
		}
		if result == nil {
			return nil
		}
		return json.Unmarshal(data, result)
	}, rc)
}

// DoXMLWithRetry calls DoXML with retry logic for transient failures.
func (c *Client) DoXMLWithRetry(ctx context.Context, method, url string, headers map[string]string, body io.Reader, result interface{}, rc RetryConfig) error {
	return c.doWithRetry(ctx, method, url, headers, body, func(data []byte, status int) error {
		if status != http.StatusOK {
			return fmt.Errorf("unexpected status %d: %s", status, truncateBody(string(data)))
		}
		if result == nil {
			return nil
		}
		return xml.Unmarshal(data, result)
	}, rc)
}

type responseHandler func([]byte, int) error

func (c *Client) doWithRetry(ctx context.Context, method, url string, headers map[string]string, body io.Reader, fn responseHandler, rc RetryConfig) error {
	backoff := rc.InitialBackoff
	for attempt := 1; attempt <= rc.MaxAttempts; attempt++ {
		data, status, retryAfter, err := c.DoRequest(ctx, method, url, headers, body)
		if err == nil {
			if rc.shouldRetry(status) && attempt < rc.MaxAttempts {
				if rc.RespectRetryAfter {
					if retryAfter > 0 && retryAfter < rc.MaxBackoff {
						backoff = retryAfter
					} else if ra := parseRetryAfter(string(data)); ra > 0 && ra < rc.MaxBackoff {
						backoff = ra
					}
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(jitter(backoff)):
				}
				backoff = min(backoff*2, rc.MaxBackoff)
				continue
			}
			return fn(data, status)
		}
		if attempt == rc.MaxAttempts {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jitter(backoff)):
		}
		backoff = min(backoff*2, rc.MaxBackoff)
	}
	return nil
}

func truncateBody(s string) string {
	if len(s) > 512 {
		return s[:512] + "..."
	}
	return s
}

func parseRetryAfter(body string) time.Duration {
	var h struct {
		RetryAfter string `json:"retryAfter"`
	}
	if json.Unmarshal([]byte(body), &h) == nil && h.RetryAfter != "" {
		if d, err := time.ParseDuration(h.RetryAfter); err == nil {
			return d
		}
		if s, err := strconv.Atoi(h.RetryAfter); err == nil {
			return time.Duration(s) * time.Second
		}
	}
	return 0
}
