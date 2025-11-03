package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	baseURL := "http://test.example.com"
	token := "test-token"
	timeout := 10 * time.Second

	client := NewClient(baseURL, token, timeout)

	if client.GetBaseURL() != baseURL {
		t.Errorf("Expected base URL %q, got %q", baseURL, client.GetBaseURL())
	}
	if client.GetToken() != token {
		t.Errorf("Expected token %q, got %q", token, client.GetToken())
	}
	if client.GetTimeout() != timeout {
		t.Errorf("Expected timeout %v, got %v", timeout, client.GetTimeout())
	}
}

func TestNewClientWithDefaultTimeout(t *testing.T) {
	client := NewClient("http://test.example.com", "test-token", 0)

	if client.GetTimeout() != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", client.GetTimeout())
	}
}

func TestClientSetters(t *testing.T) {
	client := NewClient("http://test.example.com", "test-token", 30*time.Second)

	client.SetUserAgent("TestAgent/1.0")
	client.SetTimeout(60 * time.Second)

	// Note: We can't directly test the user agent since it's not exposed
	// but we can test the timeout
	if client.GetTimeout() != 60*time.Second {
		t.Errorf("Expected timeout 60s, got %v", client.GetTimeout())
	}
}

func TestClientGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET method, got %s", r.Method)
		}
		if r.Header.Get("X-Api-Key") != "test-token" {
			t.Errorf("Expected X-Api-Key header, got %q", r.Header.Get("X-Api-Key"))
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("User-Agent") != "CalmsToolkit/1.0" {
			t.Errorf("Expected User-Agent header, got %q", r.Header.Get("User-Agent"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "success"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", 30*time.Second)
	resp, err := client.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if !resp.IsSuccess() {
		t.Errorf("Expected successful response, got status %d", resp.StatusCode)
	}

	var result map[string]string
	if err := resp.JSON(&result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result["message"] != "success" {
		t.Errorf("Expected message 'success', got %q", result["message"])
	}
}

func TestClientPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type header, got %q", r.Header.Get("Content-Type"))
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}
		defer r.Body.Close()

		var data map[string]string
		if err := json.Unmarshal(body, &data); err != nil {
			t.Fatalf("Failed to parse request body: %v", err)
		}

		if data["test"] != "value" {
			t.Errorf("Expected test value 'value', got %q", data["test"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"result": "created"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", 30*time.Second)
	resp, err := client.Post(context.Background(), "/test", map[string]string{"test": "value"})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if !resp.IsSuccess() {
		t.Errorf("Expected successful response, got status %d", resp.StatusCode)
	}

	var result map[string]string
	if err := resp.JSON(&result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result["result"] != "created" {
		t.Errorf("Expected result 'created', got %q", result["result"])
	}
}

func TestClientPut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("Expected PUT method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", 30*time.Second)
	resp, err := client.Put(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if !resp.IsSuccess() {
		t.Errorf("Expected successful response, got status %d", resp.StatusCode)
	}
}

func TestClientDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("Expected DELETE method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", 30*time.Second)
	resp, err := client.Delete(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if !resp.IsSuccess() {
		t.Errorf("Expected successful response, got status %d", resp.StatusCode)
	}
}

func TestResponseMethods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "test"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", 30*time.Second)

	// Test JSON method
	resp, err := client.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp.Body.Close()

	var result map[string]string
	if err := resp.JSON(&result); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}
	if result["message"] != "test" {
		t.Errorf("Expected message 'test', got %q", result["message"])
	}

	// Test Text method - need a new response since body was consumed
	resp2, err := client.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp2.Body.Close()

	text, err := resp2.Text()
	if err != nil {
		t.Fatalf("Failed to get text: %v", err)
	}
	if !strings.Contains(text, "test") {
		t.Errorf("Expected text to contain 'test', got %q", text)
	}

	// Test Bytes method - need another new response
	resp3, err := client.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp3.Body.Close()

	bytes, err := resp3.Bytes()
	if err != nil {
		t.Fatalf("Failed to get bytes: %v", err)
	}
	if len(bytes) == 0 {
		t.Error("Expected non-empty bytes")
	}
}

func TestResponseStatusMethods(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		isSuccess     bool
		isClientError bool
		isServerError bool
	}{
		{"Success", http.StatusOK, true, false, false},
		{"Created", http.StatusCreated, true, false, false},
		{"BadRequest", http.StatusBadRequest, false, true, false},
		{"Unauthorized", http.StatusUnauthorized, false, true, false},
		{"NotFound", http.StatusNotFound, false, true, false},
		{"InternalServerError", http.StatusInternalServerError, false, false, true},
		{"BadGateway", http.StatusBadGateway, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token", 30*time.Second)
			resp, err := client.Get(context.Background(), "/test")
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}
			defer resp.Body.Close()

			if resp.IsSuccess() != tt.isSuccess {
				t.Errorf("Expected IsSuccess() to be %v, got %v", tt.isSuccess, resp.IsSuccess())
			}
			if resp.IsClientError() != tt.isClientError {
				t.Errorf("Expected IsClientError() to be %v, got %v", tt.isClientError, resp.IsClientError())
			}
			if resp.IsServerError() != tt.isServerError {
				t.Errorf("Expected IsServerError() to be %v, got %v", tt.isServerError, resp.IsServerError())
			}
		})
	}
}

func TestEnsureSuccess(t *testing.T) {
	// Test successful response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", 30*time.Second)
	resp, err := client.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if err := resp.EnsureSuccess(); err != nil {
		t.Errorf("Expected no error for successful response, got %v", err)
	}

	// Test error response
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "bad request"})
	}))
	defer errorServer.Close()

	errorClient := NewClient(errorServer.URL, "test-token", 30*time.Second)
	errorResp, err := errorClient.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer errorResp.Body.Close()

	if err := errorResp.EnsureSuccess(); err == nil {
		t.Error("Expected error for error response, got nil")
	}
}

func TestTestConnection(t *testing.T) {
	// Test successful connection
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/me" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", 30*time.Second)
	if err := client.TestConnection(context.Background()); err != nil {
		t.Errorf("Expected successful connection, got %v", err)
	}

	// Test failed connection
	client = NewClient("http://invalid.example.com", "test-token", 1*time.Second)
	if err := client.TestConnection(context.Background()); err == nil {
		t.Error("Expected connection error, got nil")
	}
}

func TestClientWithoutToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "" {
			t.Errorf("Expected no X-Api-Key header, got %q", r.Header.Get("X-Api-Key"))
		}
		if r.Header.Get("Authorization") != "" {
			t.Errorf("Expected no Authorization header, got %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", 30*time.Second)
	resp, err := client.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer resp.Body.Close()

	if !resp.IsSuccess() {
		t.Errorf("Expected successful response, got status %d", resp.StatusCode)
	}
}
