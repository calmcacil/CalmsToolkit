// Package httputil provides shared HTTP client utilities for API fetchers.
package httputil

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Client is a reusable HTTP client wrapper around http.Client.
type Client struct {
	*http.Client
	MaxBodySize int64
}

// ResponseError describes a non-success HTTP response without exposing request
// credentials. Body is bounded and redacted before it is stored.
type ResponseError struct {
	StatusCode        int
	Method, URL, Body string
}

func (e *ResponseError) Error() string {
	return fmt.Sprintf("%s %s: unexpected status %d: %s", e.Method, e.URL, e.StatusCode, e.Body)
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
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 20
	transport.IdleConnTimeout = 30 * time.Second
	return &Client{
		Client: &http.Client{
			Timeout:   timeout,
			Transport: transport,
		},
		MaxBodySize: 10 * 1024 * 1024,
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
func (c *Client) DoRequest(ctx context.Context, method, url string, headers map[string]string, body io.Reader) ([]byte, int, error) {
	data, status, _, err := c.doRequest(ctx, method, url, headers, body)
	return data, status, err
}

func (c *Client) doRequest(ctx context.Context, method, requestURL string, headers map[string]string, body io.Reader) ([]byte, int, http.Header, error) {
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
	if err != nil {
		return nil, 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()
	maxSize := c.MaxBodySize
	if maxSize <= 0 {
		maxSize = 10 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return nil, resp.StatusCode, resp.Header.Clone(), err
	}
	if int64(len(data)) > maxSize {
		return nil, resp.StatusCode, resp.Header.Clone(), fmt.Errorf("response body too large (exceeds %d bytes)", maxSize)
	}
	return data, resp.StatusCode, resp.Header.Clone(), nil
}

// DoJSON performs an HTTP request and decodes the JSON response into result.
func (c *Client) DoJSON(ctx context.Context, method, url string, headers map[string]string, body io.Reader, result interface{}) error {
	data, status, err := c.DoRequest(ctx, method, url, headers, body)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return responseError(method, url, status, data)
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
		return responseError(method, url, status, data)
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
			return responseError(method, url, status, data)
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
			return responseError(method, url, status, data)
		}
		if result == nil {
			return nil
		}
		return xml.Unmarshal(data, result)
	}, rc)
}

type responseHandler func([]byte, int) error

func (c *Client) doWithRetry(ctx context.Context, method, url string, headers map[string]string, body io.Reader, fn responseHandler, rc RetryConfig) error {
	var bodyData []byte
	var err error
	if body != nil {
		bodyData, err = io.ReadAll(body)
		if err != nil {
			return fmt.Errorf("reading request body: %w", err)
		}
	}
	if !retryableMethod(method) {
		rc.MaxAttempts = min(rc.MaxAttempts, 1)
	}
	if rc.MaxAttempts < 1 {
		rc.MaxAttempts = 1
	}
	backoff := rc.InitialBackoff
	for attempt := 1; attempt <= rc.MaxAttempts; attempt++ {
		var requestBody io.Reader
		if bodyData != nil {
			requestBody = bytes.NewReader(bodyData)
		}
		data, status, responseHeaders, err := c.doRequest(ctx, method, url, headers, requestBody)
		if err == nil {
			if rc.shouldRetry(status) && attempt < rc.MaxAttempts {
				if rc.RespectRetryAfter {
					ra := parseRetryAfterHeader(responseHeaders.Get("Retry-After"), time.Now())
					if ra <= 0 {
						ra = parseRetryAfter(string(data))
					}
					if ra > 0 && (rc.MaxBackoff <= 0 || ra <= rc.MaxBackoff) {
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
	s = redact(s)
	if len(s) > 512 {
		return s[:512] + "..."
	}
	return s
}

func responseError(method, requestURL string, status int, data []byte) error {
	return &ResponseError{StatusCode: status, Method: method, URL: safeURL(requestURL), Body: truncateBody(strings.TrimSpace(string(data)))}
}

func safeURL(value string) string {
	u, err := url.Parse(value)
	if err != nil {
		return "<invalid-url>"
	}
	query := u.Query()
	for key := range query {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "key") || strings.Contains(lower, "auth") {
			query.Set(key, "REDACTED")
		}
	}
	u.RawQuery = query.Encode()
	return u.String()
}

var secretPattern = regexp.MustCompile(`(?i)(api[_-]?key|token|authorization)(["']?\s*[:=]\s*["']?)[^"'\s&,}]+`)

func redact(value string) string { return secretPattern.ReplaceAllString(value, `${1}${2}REDACTED`) }

func retryableMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

func parseRetryAfterHeader(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil && when.After(now) {
		return when.Sub(now)
	}
	return 0
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
