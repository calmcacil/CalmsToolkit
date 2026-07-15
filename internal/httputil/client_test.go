package httputil

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRetryReplaysBodyAndReadsRetryAfterHeader(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"value":1}` {
			t.Errorf("attempt %d body=%q", attempts, body)
		}
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"ok"}`))
	}))
	defer server.Close()
	client := NewClient(time.Second)
	var result testJSON
	rc := DefaultRetry()
	rc.InitialBackoff = time.Millisecond
	rc.MaxBackoff = 2 * time.Millisecond
	if err := client.DoJSONWithRetry(context.Background(), http.MethodPut, server.URL, nil, strings.NewReader(`{"value":1}`), &result, rc); err != nil {
		t.Fatal(err)
	}
	if attempts != 2 || result.Message != "ok" {
		t.Fatalf("attempts=%d result=%+v", attempts, result)
	}
}

func TestResponseErrorRedactsSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"api_key":"supersecret"}`))
	}))
	defer server.Close()
	err := NewClient(time.Second).DoJSON(context.Background(), http.MethodGet, server.URL+"?token=querysecret", nil, nil, nil)
	var responseErr *ResponseError
	if !errors.As(err, &responseErr) {
		t.Fatalf("error type=%T", err)
	}
	if strings.Contains(err.Error(), "supersecret") || strings.Contains(err.Error(), "querysecret") {
		t.Fatalf("secret leaked: %v", err)
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient(5 * time.Second)
	if c.Timeout != 5*time.Second {
		t.Errorf("Timeout = %v, want 5s", c.Timeout)
	}
}

func TestNewTransportClient(t *testing.T) {
	c := NewTransportClient(10 * time.Second)
	if c.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", c.Timeout)
	}
}

func TestDoRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))
	defer server.Close()

	ctx := context.Background()
	client := NewClient(5 * time.Second)

	body, status, err := client.DoRequest(ctx, "GET", server.URL, nil, nil)
	if err != nil {
		t.Fatalf("DoRequest error: %v", err)
	}
	if status != http.StatusOK {
		t.Errorf("Status = %d, want %d", status, http.StatusOK)
	}
	if string(body) != "hello" {
		t.Errorf("Body = %q, want %q", string(body), "hello")
	}
}

type testJSON struct {
	Message string `json:"message"`
}

func TestDoJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"ok"}`))
	}))
	defer server.Close()

	ctx := context.Background()
	client := NewClient(5 * time.Second)

	var result testJSON
	err := client.DoJSON(ctx, "GET", server.URL, nil, nil, &result)
	if err != nil {
		t.Fatalf("DoJSON error: %v", err)
	}
	if result.Message != "ok" {
		t.Errorf("Message = %q, want %q", result.Message, "ok")
	}
}

func TestDoJSONNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	ctx := context.Background()
	client := NewClient(5 * time.Second)

	err := client.DoJSON(ctx, "GET", server.URL, nil, nil, nil)
	if err == nil {
		t.Fatal("Expected error for non-OK status, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("Error should mention status code, got: %v", err)
	}
}

type testXML struct {
	Message string `xml:"message"`
}

func TestDoXML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.Write([]byte(`<root><message>ok</message></root>`))
	}))
	defer server.Close()

	ctx := context.Background()
	client := NewClient(5 * time.Second)

	var result testXML
	err := client.DoXML(ctx, "GET", server.URL, nil, nil, &result)
	if err != nil {
		t.Fatalf("DoXML error: %v", err)
	}
	if result.Message != "ok" {
		t.Errorf("Message = %q, want %q", result.Message, "ok")
	}
}

func TestLargeBodyRejection(t *testing.T) {
	largeBody := make([]byte, 12*1024*1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(largeBody)
	}))
	defer server.Close()

	ctx := context.Background()
	client := NewClient(5 * time.Second)

	_, _, err := client.DoRequest(ctx, "GET", server.URL, nil, nil)
	if err == nil {
		t.Fatal("Expected error for large body, got nil")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("Error should mention too large, got: %v", err)
	}
}
