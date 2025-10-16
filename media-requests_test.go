//go:build mediarequests
// +build mediarequests

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestGetYear verifies year extraction from search results
func TestGetYear(t *testing.T) {
	tests := []struct {
		name     string
		result   SearchResult
		expected string
	}{
		{
			name: "Movie with release date",
			result: SearchResult{
				MediaType:   "movie",
				ReleaseDate: "2024-03-15",
			},
			expected: "2024",
		},
		{
			name: "TV show with first air date",
			result: SearchResult{
				MediaType:    "tv",
				FirstAirDate: "2023-01-10",
			},
			expected: "2023",
		},
		{
			name: "Movie with empty date",
			result: SearchResult{
				MediaType:   "movie",
				ReleaseDate: "",
			},
			expected: "",
		},
		{
			name: "TV show with empty date",
			result: SearchResult{
				MediaType:    "tv",
				FirstAirDate: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getYear(tt.result)
			if got != tt.expected {
				t.Errorf("getYear() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestGetStatusText verifies REQUEST status code to text mapping
func TestGetStatusText(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected string
	}{
		{"Pending status", StatusPending, "Pending Approval"},
		{"Approved status", StatusApproved, "Approved"},
		{"Declined status", StatusDeclined, "Declined"},
		{"Invalid status", 999, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStatusText(tt.status)
			if got != tt.expected {
				t.Errorf("getStatusText(%d) = %v, want %v", tt.status, got, tt.expected)
			}
		})
	}
}

// TestFormatDate verifies date formatting
func TestFormatDate(t *testing.T) {
	tests := []struct {
		name     string
		dateStr  string
		expected string
	}{
		{
			name:     "Valid ISO date",
			dateStr:  "2024-03-15T10:30:00.000Z",
			expected: "2024-03-15 10:30",
		},
		{
			name:     "Invalid date",
			dateStr:  "invalid",
			expected: "invalid",
		},
		{
			name:     "Empty date",
			dateStr:  "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDate(tt.dateStr)
			if got != tt.expected {
				t.Errorf("formatDate(%q) = %v, want %v", tt.dateStr, got, tt.expected)
			}
		})
	}
}

// TestLoadEnvFile verifies environment file loading
func TestLoadEnvFile(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Create test .env file
	content := `OVERSEERR_URL=http://test.example.com
OVERSEERR_TOKEN=test-api-key-123
# This is a comment
SOME_OTHER_VAR=ignored
`
	err := os.WriteFile(envFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test .env file: %v", err)
	}

	// Test loading
	config := &Config{}
	loadEnvFile(envFile, config)

	// Verify values were loaded
	if config.ServerURL != "http://test.example.com" {
		t.Errorf("ServerURL = %v, want %v", config.ServerURL, "http://test.example.com")
	}
	if config.APIKey != "test-api-key-123" {
		t.Errorf("APIKey = %v, want %v", config.APIKey, "test-api-key-123")
	}
}

// TestLoadEnvFileMissing verifies behavior when .env file doesn't exist
func TestLoadEnvFileMissing(t *testing.T) {
	config := &Config{
		ServerURL: "http://original.example.com",
		APIKey:    "original-key",
	}

	// Should not panic when file doesn't exist
	loadEnvFile("/nonexistent/path/.env", config)

	// Config should remain unchanged
	if config.ServerURL != "http://original.example.com" {
		t.Errorf("ServerURL changed when it shouldn't: %v", config.ServerURL)
	}
}

// TestLoadConfig verifies configuration loading with precedence
func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name            string
		serverURL       string
		apiKey          string
		timeout         time.Duration
		noColor         bool
		expectedURL     string
		expectedKey     string
		expectedTimeout time.Duration
		expectedNoColor bool
	}{
		{
			name:            "All parameters provided",
			serverURL:       "http://cli.example.com",
			apiKey:          "cli-key",
			timeout:         45 * time.Second,
			noColor:         true,
			expectedURL:     "http://cli.example.com",
			expectedKey:     "cli-key",
			expectedTimeout: 45 * time.Second,
			expectedNoColor: true,
		},
		{
			name:            "Zero timeout stays zero",
			serverURL:       "http://test.example.com",
			apiKey:          "test-key",
			timeout:         0,
			noColor:         false,
			expectedURL:     "http://test.example.com",
			expectedKey:     "test-key",
			expectedTimeout: 0,
			expectedNoColor: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := loadConfig(tt.serverURL, tt.apiKey, tt.timeout, tt.noColor)

			if config.ServerURL != tt.expectedURL {
				t.Errorf("ServerURL = %v, want %v", config.ServerURL, tt.expectedURL)
			}
			if config.APIKey != tt.expectedKey {
				t.Errorf("APIKey = %v, want %v", config.APIKey, tt.expectedKey)
			}
			if config.Timeout != tt.expectedTimeout {
				t.Errorf("Timeout = %v, want %v", config.Timeout, tt.expectedTimeout)
			}
			if config.NoColor != tt.expectedNoColor {
				t.Errorf("NoColor = %v, want %v", config.NoColor, tt.expectedNoColor)
			}
		})
	}
}

// TestSearchMedia verifies media search API interaction
func TestSearchMedia(t *testing.T) {
	tests := []struct {
		name           string
		query          string
		mockResponse   SearchResponse
		mockStatusCode int
		expectError    bool
	}{
		{
			name:  "Successful search",
			query: "Inception",
			mockResponse: SearchResponse{
				Page:         1,
				TotalPages:   1,
				TotalResults: 2,
				Results: []SearchResult{
					{
						ID:          27205,
						MediaType:   "movie",
						Title:       "Inception",
						ReleaseDate: "2010-07-16",
						VoteAverage: 8.4,
					},
				},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Server error",
			query:          "test",
			mockResponse:   SearchResponse{},
			mockStatusCode: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name:  "Empty results",
			query: "NonexistentMovie12345",
			mockResponse: SearchResponse{
				Page:         1,
				TotalPages:   0,
				TotalResults: 0,
				Results:      []SearchResult{},
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request
				if r.Method != "GET" {
					t.Errorf("Expected GET request, got %s", r.Method)
				}
				if r.Header.Get("X-Api-Key") == "" {
					t.Error("Expected X-Api-Key header")
				}

				w.WriteHeader(tt.mockStatusCode)
				if tt.mockStatusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.mockResponse)
				}
			}))
			defer server.Close()

			config := Config{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			results, err := searchMedia(config, tt.query)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if len(results) != len(tt.mockResponse.Results) {
					t.Errorf("Got %d results, want %d", len(results), len(tt.mockResponse.Results))
				}
			}
		})
	}
}

// TestGetTVDetails verifies TV show details fetching
func TestGetTVDetails(t *testing.T) {
	mockTV := TVDetails{
		ID:   1,
		Name: "Test Show",
		Seasons: []Season{
			{SeasonNumber: 1, EpisodeCount: 10},
			{SeasonNumber: 2, EpisodeCount: 12},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockTV)
	}))
	defer server.Close()

	config := Config{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	details, err := getTVDetails(config, 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if details.Name != mockTV.Name {
		t.Errorf("Name = %v, want %v", details.Name, mockTV.Name)
	}
	if len(details.Seasons) != 2 {
		t.Errorf("Got %d seasons, want 2", len(details.Seasons))
	}
}

// TestCreateRequest verifies media request creation
func TestCreateRequest(t *testing.T) {
	tests := []struct {
		name           string
		media          SearchResult
		mockStatusCode int
		expectError    bool
	}{
		{
			name: "Successful movie request",
			media: SearchResult{
				ID:        27205,
				MediaType: "movie",
				Title:     "Inception",
			},
			mockStatusCode: http.StatusCreated,
			expectError:    false,
		},
		{
			name: "Request conflict (already exists)",
			media: SearchResult{
				ID:        100,
				MediaType: "movie",
				Title:     "Test Movie",
			},
			mockStatusCode: http.StatusConflict,
			expectError:    true,
		},
		{
			name: "Server error",
			media: SearchResult{
				ID:        200,
				MediaType: "movie",
				Title:     "Error Movie",
			},
			mockStatusCode: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" {
					t.Errorf("Expected POST request, got %s", r.Method)
				}
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockStatusCode == http.StatusCreated {
					json.NewEncoder(w).Encode(MediaRequest{ID: 1})
				}
			}))
			defer server.Close()

			config := Config{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			_, err := createRequest(config, tt.media, nil, nil)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestGetPendingRequests verifies pending requests fetching
func TestGetPendingRequests(t *testing.T) {
	mockRequests := RequestsResponse{
		PageInfo: PageInfo{
			Pages:   1,
			Results: 2,
		},
		Results: []MediaRequest{
			{
				ID:     1,
				Status: StatusPending,
				Media: MediaInfo{
					TmdbID: 27205,
				},
			},
			{
				ID:     2,
				Status: StatusPending,
				Media: MediaInfo{
					TmdbID: 1234,
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockRequests)
	}))
	defer server.Close()

	config := Config{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	requests, err := getPendingRequests(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(requests) != 2 {
		t.Errorf("Got %d requests, want 2", len(requests))
	}
}

// TestApproveRequest verifies request approval
func TestApproveRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/approve") {
			t.Errorf("Expected /approve in path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := Config{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	err := approveRequest(config, 123)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestDeclineRequest verifies request decline
func TestDeclineRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/decline") {
			t.Errorf("Expected /decline in path, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	config := Config{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	err := declineRequest(config, 123)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestTestConnection verifies connection testing
func TestTestConnection(t *testing.T) {
	tests := []struct {
		name           string
		mockStatusCode int
		expectError    bool
	}{
		{
			name:           "Successful connection",
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Unauthorized",
			mockStatusCode: http.StatusUnauthorized,
			expectError:    true,
		},
		{
			name:           "Not Found",
			mockStatusCode: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatusCode)
				json.NewEncoder(w).Encode(map[string]string{"version": "1.0.0"})
			}))
			defer server.Close()

			config := Config{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			err := testConnection(config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestFetchServiceInstances verifies service instance fetching
func TestFetchServiceInstances(t *testing.T) {
	mockInstances := []ServiceInstance{
		{ID: 1, Name: "Radarr 4K", Is4k: true},
		{ID: 2, Name: "Sonarr HD", Is4k: false},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockInstances)
	}))
	defer server.Close()

	config := Config{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	instances, err := fetchServiceInstances(config, "radarr")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(instances) != 2 {
		t.Errorf("Got %d instances, want 2", len(instances))
	}
}

// TestFetchServiceDetails verifies service detail fetching
func TestFetchServiceDetails(t *testing.T) {
	mockDetails := ServiceDetails{
		RootFolders: []ServiceRootFolder{
			{ID: 1, Path: "/movies/4k"},
			{ID: 2, Path: "/movies/hd"},
		},
		Profiles: []ServiceProfile{
			{ID: 1, Name: "Ultra HD"},
			{ID: 2, Name: "HD 1080p"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(mockDetails)
	}))
	defer server.Close()

	config := Config{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	details, err := fetchServiceDetails(config, "radarr", 1)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(details.RootFolders) != 2 {
		t.Errorf("Got %d root folders, want 2", len(details.RootFolders))
	}
	if len(details.Profiles) != 2 {
		t.Errorf("Got %d profiles, want 2", len(details.Profiles))
	}
}

// TestSearchMediaWithSpaces verifies that search queries with spaces are properly URL encoded
func TestSearchMediaWithSpaces(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		expectedQuery string // What we expect to see in the URL
	}{
		{
			name:          "Query with single space",
			query:         "The Matrix",
			expectedQuery: "The+Matrix", // URL encoded space
		},
		{
			name:          "Query with multiple spaces",
			query:         "Star Wars Episode IV",
			expectedQuery: "Star+Wars+Episode+IV",
		},
		{
			name:          "Query with special characters",
			query:         "Rick & Morty",
			expectedQuery: "Rick+%26+Morty", // & becomes %26
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify the query parameter is properly encoded
				query := r.URL.Query().Get("query")
				if query != tt.query {
					t.Errorf("Query parameter = %q, want %q", query, tt.query)
				}

				// Verify URL encoding in raw query
				if !strings.Contains(r.URL.RawQuery, tt.expectedQuery) {
					t.Errorf("RawQuery = %q, should contain %q", r.URL.RawQuery, tt.expectedQuery)
				}

				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(SearchResponse{
					Page:         1,
					TotalPages:   1,
					TotalResults: 1,
					Results:      []SearchResult{},
				})
			}))
			defer server.Close()

			config := Config{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			_, err := searchMedia(config, tt.query)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestSearchMediaErrorDiagnostics verifies that error messages include response body
func TestSearchMediaErrorDiagnostics(t *testing.T) {
	tests := []struct {
		name           string
		mockStatusCode int
		mockBody       string
		expectErrorMsg string
	}{
		{
			name:           "Bad Request with body",
			mockStatusCode: http.StatusBadRequest,
			mockBody:       `{"message":"Invalid query parameter"}`,
			expectErrorMsg: "Invalid query parameter",
		},
		{
			name:           "Unauthorized with body",
			mockStatusCode: http.StatusUnauthorized,
			mockBody:       `{"error":"Invalid API key"}`,
			expectErrorMsg: "Invalid API key",
		},
		{
			name:           "Server error with body",
			mockStatusCode: http.StatusInternalServerError,
			mockBody:       `{"error":"Database connection failed"}`,
			expectErrorMsg: "Database connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.mockStatusCode)
				w.Write([]byte(tt.mockBody))
			}))
			defer server.Close()

			config := Config{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			_, err := searchMedia(config, "test query")
			if err == nil {
				t.Fatal("Expected error, got nil")
			}

			// Verify error message includes both status code and response body
			errMsg := err.Error()
			if !strings.Contains(errMsg, tt.expectErrorMsg) {
				t.Errorf("Error message = %q, should contain %q", errMsg, tt.expectErrorMsg)
			}
			if !strings.Contains(errMsg, "status") {
				t.Errorf("Error message = %q, should contain status code", errMsg)
			}
		})
	}
}
