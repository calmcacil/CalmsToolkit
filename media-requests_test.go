//go:build mediarequests
// +build mediarequests

package main

import (
	"encoding/json"
	"fmt"
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

// TestCheckUserPermissions verifies permission checking
func TestCheckUserPermissions(t *testing.T) {
	const (
		MANAGE_REQUESTS = 16
		ADMIN           = 2
	)

	tests := []struct {
		name              string
		mockAuthMe        AuthMe
		mockStatusCode    int
		expectError       bool
		expectManagePerms bool
		expectAdminPerms  bool
	}{
		{
			name: "User with MANAGE_REQUESTS permission",
			mockAuthMe: AuthMe{
				ID:          1,
				Email:       "admin@example.com",
				Permissions: MANAGE_REQUESTS,
			},
			mockStatusCode:    http.StatusOK,
			expectError:       false,
			expectManagePerms: true,
			expectAdminPerms:  false,
		},
		{
			name: "User with ADMIN permission",
			mockAuthMe: AuthMe{
				ID:          1,
				Email:       "admin@example.com",
				Permissions: ADMIN,
			},
			mockStatusCode:    http.StatusOK,
			expectError:       false,
			expectManagePerms: false,
			expectAdminPerms:  true,
		},
		{
			name: "User with both MANAGE_REQUESTS and ADMIN",
			mockAuthMe: AuthMe{
				ID:          1,
				Email:       "superadmin@example.com",
				Permissions: MANAGE_REQUESTS | ADMIN,
			},
			mockStatusCode:    http.StatusOK,
			expectError:       false,
			expectManagePerms: true,
			expectAdminPerms:  true,
		},
		{
			name: "User with no permissions",
			mockAuthMe: AuthMe{
				ID:          1,
				Email:       "user@example.com",
				Permissions: 0,
			},
			mockStatusCode:    http.StatusOK,
			expectError:       false,
			expectManagePerms: false,
			expectAdminPerms:  false,
		},
		{
			name:           "Unauthorized",
			mockAuthMe:     AuthMe{},
			mockStatusCode: http.StatusUnauthorized,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/auth/me" {
					t.Errorf("Expected /api/v1/auth/me, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockStatusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.mockAuthMe)
				}
			}))
			defer server.Close()

			config := Config{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			authMe, err := checkUserPermissions(config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if authMe.Permissions != tt.mockAuthMe.Permissions {
					t.Errorf("Permissions = %d, want %d", authMe.Permissions, tt.mockAuthMe.Permissions)
				}

				hasManage := (authMe.Permissions & MANAGE_REQUESTS) != 0
				if hasManage != tt.expectManagePerms {
					t.Errorf("Has MANAGE_REQUESTS = %v, want %v", hasManage, tt.expectManagePerms)
				}

				hasAdmin := (authMe.Permissions & ADMIN) != 0
				if hasAdmin != tt.expectAdminPerms {
					t.Errorf("Has ADMIN = %v, want %v", hasAdmin, tt.expectAdminPerms)
				}
			}
		})
	}
}

// TestGetRequestCount verifies request count fetching
func TestGetRequestCount(t *testing.T) {
	tests := []struct {
		name           string
		mockCount      RequestCount
		mockStatusCode int
		expectError    bool
	}{
		{
			name: "Valid request count",
			mockCount: RequestCount{
				Pending:  5,
				Approved: 10,
				Total:    15,
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name: "Zero pending requests",
			mockCount: RequestCount{
				Pending:  0,
				Approved: 8,
				Total:    8,
			},
			mockStatusCode: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "Server error",
			mockCount:      RequestCount{},
			mockStatusCode: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/request/count" {
					t.Errorf("Expected /api/v1/request/count, got %s", r.URL.Path)
				}
				w.WriteHeader(tt.mockStatusCode)
				if tt.mockStatusCode == http.StatusOK {
					json.NewEncoder(w).Encode(tt.mockCount)
				}
			}))
			defer server.Close()

			config := Config{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			count, err := getRequestCount(config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if count.Pending != tt.mockCount.Pending {
					t.Errorf("Pending = %d, want %d", count.Pending, tt.mockCount.Pending)
				}
				if count.Approved != tt.mockCount.Approved {
					t.Errorf("Approved = %d, want %d", count.Approved, tt.mockCount.Approved)
				}
				if count.Total != tt.mockCount.Total {
					t.Errorf("Total = %d, want %d", count.Total, tt.mockCount.Total)
				}
			}
		})
	}
}

// TestGetPendingRequestsHappyPath verifies the normal case where filter=pending works
func TestGetPendingRequestsHappyPath(t *testing.T) {
	mockCount := RequestCount{
		Pending:  3,
		Approved: 5,
		Total:    8,
	}

	mockRequests := RequestsResponse{
		PageInfo: PageInfo{
			Pages:    1,
			PageSize: 50,
			Results:  3,
			Page:     1,
		},
		Results: []MediaRequest{
			{
				ID:     101,
				Status: StatusPending,
				Type:   "movie",
				Media: MediaInfo{
					TmdbID: 550,
				},
				RequestedBy: User{
					ID:    1,
					Email: "user1@example.com",
				},
			},
			{
				ID:     102,
				Status: StatusPending,
				Type:   "tv",
				Media: MediaInfo{
					TmdbID: 1396,
				},
				RequestedBy: User{
					ID:    2,
					Email: "user2@example.com",
				},
			},
			{
				ID:     103,
				Status: StatusPending,
				Type:   "movie",
				Media: MediaInfo{
					TmdbID: 27205,
				},
				RequestedBy: User{
					ID:    1,
					Email: "user1@example.com",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/request/count"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(mockCount)
		case strings.Contains(r.URL.Path, "/request"):
			// Verify the request uses filter=pending
			if !strings.Contains(r.URL.RawQuery, "filter=pending") {
				t.Errorf("Expected filter=pending in query, got %s", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(mockRequests)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
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

	if len(requests) != 3 {
		t.Errorf("Got %d requests, want 3", len(requests))
	}

	// Verify all returned requests have pending status
	for i, req := range requests {
		if req.Status != StatusPending {
			t.Errorf("Request %d has status %d, want %d (StatusPending)", i, req.Status, StatusPending)
		}
	}
}

// TestGetPendingRequestsNoPending verifies behavior when no pending requests exist
func TestGetPendingRequestsNoPending(t *testing.T) {
	mockCountResponse := RequestCount{
		Pending:  0,
		Approved: 10,
		Total:    10,
	}

	mockRequestsResponse := RequestsResponse{
		PageInfo: PageInfo{
			Pages:    0,
			PageSize: 50,
			Results:  0,
			Page:     1,
		},
		Results: []MediaRequest{},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/request/count") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(mockCountResponse)
		} else if strings.Contains(r.URL.Path, "/request") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(mockRequestsResponse)
		}
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

	if len(requests) != 0 {
		t.Errorf("Got %d requests, want 0", len(requests))
	}
}

// TestGetPendingRequestsPagination verifies multi-page request fetching
func TestGetPendingRequestsPagination(t *testing.T) {
	requestCallCount := 0
	totalResults := 125 // More than 50 to trigger pagination

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		switch {
		case strings.Contains(r.URL.Path, "/request/count"):
			json.NewEncoder(w).Encode(RequestCount{
				Pending:  totalResults,
				Approved: 10,
				Total:    totalResults + 10,
			})

		case strings.Contains(r.URL.Path, "/request"):
			requestCallCount++

			// Parse skip parameter
			skip := r.URL.Query().Get("skip")
			skipNum := 0
			if skip != "" {
				fmt.Sscanf(skip, "%d", &skipNum)
			}

			// Calculate what to return for this page
			pageStart := skipNum
			pageEnd := skipNum + 50
			if pageEnd > totalResults {
				pageEnd = totalResults
			}

			var results []MediaRequest
			for i := pageStart; i < pageEnd; i++ {
				results = append(results, MediaRequest{
					ID:     i + 1,
					Status: StatusPending,
					Media:  MediaInfo{TmdbID: 1000 + i},
				})
			}

			response := RequestsResponse{
				PageInfo: PageInfo{
					Pages:    3,
					PageSize: 50,
					Results:  totalResults,
					Page:     (skipNum / 50) + 1,
				},
				Results: results,
			}

			json.NewEncoder(w).Encode(response)
		}
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

	// Should make 3 calls: skip=0 (50 results), skip=50 (50 results), skip=100 (25 results)
	if requestCallCount != 3 {
		t.Errorf("Made %d API calls to /request, want 3", requestCallCount)
	}

	if len(requests) != totalResults {
		t.Errorf("Got %d requests, want %d", len(requests), totalResults)
	}

	// Verify request IDs are sequential
	for i, req := range requests {
		expectedID := i + 1
		if req.ID != expectedID {
			t.Errorf("Request %d has ID %d, want %d", i, req.ID, expectedID)
			break // Only show first mismatch
		}
	}
}

// TestGetPendingRequestsWithFallback verifies the fallback logic when filter=pending returns 0 results
func TestGetPendingRequestsWithFallback(t *testing.T) {
	callCount := 0
	pendingCallCount := 0
	allCallCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		endpoint := r.URL.Path + "?" + r.URL.RawQuery

		switch {
		case r.URL.Path == "/api/v1/request/count":
			json.NewEncoder(w).Encode(RequestCount{
				Pending:  2,
				Approved: 3,
				Total:    5,
			})

		case r.URL.Query().Get("filter") == "pending":
			pendingCallCount++
			json.NewEncoder(w).Encode(RequestsResponse{
				PageInfo: PageInfo{
					Pages:    0,
					PageSize: 50,
					Results:  0,
					Page:     1,
				},
				Results: []MediaRequest{},
			})

		case r.URL.Query().Get("filter") == "all":
			allCallCount++
			json.NewEncoder(w).Encode(RequestsResponse{
				PageInfo: PageInfo{
					Pages:    1,
					PageSize: 50,
					Results:  5,
					Page:     1,
				},
				Results: []MediaRequest{
					{ID: 1, Status: StatusPending, Media: MediaInfo{TmdbID: 100}},
					{ID: 2, Status: StatusApproved, Media: MediaInfo{TmdbID: 101}},
					{ID: 3, Status: StatusPending, Media: MediaInfo{TmdbID: 102}},
					{ID: 4, Status: StatusApproved, Media: MediaInfo{TmdbID: 103}},
					{ID: 5, Status: StatusDeclined, Media: MediaInfo{TmdbID: 104}},
				},
			})

		default:
			t.Errorf("Unexpected endpoint called: %s", endpoint)
			w.WriteHeader(http.StatusNotFound)
		}
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

	if pendingCallCount != 1 {
		t.Errorf("Made %d calls to filter=pending, want 1", pendingCallCount)
	}

	if allCallCount != 1 {
		t.Errorf("Made %d calls to filter=all, want 1 (fallback should have triggered)", allCallCount)
	}

	if len(requests) != 2 {
		t.Errorf("Got %d pending requests, want 2", len(requests))
	}

	for _, req := range requests {
		if req.Status != StatusPending {
			t.Errorf("Request %d has status %d, want %d (StatusPending)", req.ID, req.Status, StatusPending)
		}
	}

	expectedPendingIDs := []int{1, 3}
	for i, req := range requests {
		if req.ID != expectedPendingIDs[i] {
			t.Errorf("Request %d has ID %d, want %d", i, req.ID, expectedPendingIDs[i])
		}
	}
}

// TestGetPendingRequestsNoFallbackNeeded verifies fallback is not triggered when primary fetch works
func TestGetPendingRequestsNoFallbackNeeded(t *testing.T) {
	pendingCallCount := 0
	allCallCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/request/count":
			json.NewEncoder(w).Encode(RequestCount{
				Pending:  2,
				Approved: 3,
				Total:    5,
			})

		case r.URL.Query().Get("filter") == "pending":
			pendingCallCount++
			json.NewEncoder(w).Encode(RequestsResponse{
				PageInfo: PageInfo{
					Pages:    1,
					PageSize: 50,
					Results:  2,
					Page:     1,
				},
				Results: []MediaRequest{
					{ID: 1, Status: StatusPending, Media: MediaInfo{TmdbID: 100}},
					{ID: 2, Status: StatusPending, Media: MediaInfo{TmdbID: 101}},
				},
			})

		case r.URL.Query().Get("filter") == "all":
			allCallCount++
			t.Error("filter=all should not be called when filter=pending works correctly")

		default:
			w.WriteHeader(http.StatusNotFound)
		}
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

	if pendingCallCount != 1 {
		t.Errorf("Made %d calls to filter=pending, want 1", pendingCallCount)
	}

	if allCallCount != 0 {
		t.Errorf("Made %d calls to filter=all, want 0 (fallback should not trigger)", allCallCount)
	}

	if len(requests) != 2 {
		t.Errorf("Got %d pending requests, want 2", len(requests))
	}
}

// TestGetPendingRequestsFallbackPagination verifies fallback handles pagination correctly
func TestGetPendingRequestsFallbackPagination(t *testing.T) {
	allPageCount := 0
	totalResults := 120 // More than 50 to ensure pagination

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/request/count":
			json.NewEncoder(w).Encode(RequestCount{
				Pending:  60, // Half will be pending
				Approved: 60,
				Total:    totalResults,
			})

		case r.URL.Query().Get("filter") == "pending":
			// Simulate the bug: filter=pending returns 0 even though count shows 60
			json.NewEncoder(w).Encode(RequestsResponse{
				PageInfo: PageInfo{
					Pages:    0,
					PageSize: 50,
					Results:  0,
					Page:     1,
				},
				Results: []MediaRequest{},
			})

		case r.URL.Query().Get("filter") == "all":
			allPageCount++
			skip := r.URL.Query().Get("skip")
			skipNum := 0
			if skip != "" {
				fmt.Sscanf(skip, "%d", &skipNum)
			}

			// Generate results for this page
			pageStart := skipNum
			pageEnd := skipNum + 50
			if pageEnd > totalResults {
				pageEnd = totalResults
			}

			var results []MediaRequest
			for i := pageStart; i < pageEnd; i++ {
				// Alternate between pending and approved
				status := StatusApproved
				if i%2 == 0 {
					status = StatusPending
				}
				results = append(results, MediaRequest{
					ID:     i + 1,
					Status: status,
					Media:  MediaInfo{TmdbID: 1000 + i},
				})
			}

			response := RequestsResponse{
				PageInfo: PageInfo{
					Pages:    3,
					PageSize: 50,
					Results:  totalResults,
					Page:     (skipNum / 50) + 1,
				},
				Results: results,
			}

			json.NewEncoder(w).Encode(response)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
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

	// Should make 3 calls to filter=all to get all 120 results
	if allPageCount != 3 {
		t.Errorf("Made %d calls to filter=all, want 3 (should paginate through all pages)", allPageCount)
	}

	// Should have 60 pending requests (every even-indexed item)
	if len(requests) != 60 {
		t.Errorf("Got %d pending requests, want 60", len(requests))
	}

	// Verify all returned requests are pending
	for _, req := range requests {
		if req.Status != StatusPending {
			t.Errorf("Request %d has status %d, want %d (StatusPending)", req.ID, req.Status, StatusPending)
			break
		}
	}
}

// TestApproveRequestWithOverrides verifies approval with root folder override
func TestApproveRequestWithOverrides(t *testing.T) {
	tests := []struct {
		name             string
		requestID        int
		overrides        *RequestOverrides
		approveStatus    int
		updateStatus     int
		expectError      bool
		expectUpdate     bool
		expectedErrorMsg string
	}{
		{
			name:          "Approve without overrides",
			requestID:     123,
			overrides:     nil,
			approveStatus: http.StatusOK,
			updateStatus:  http.StatusOK,
			expectError:   false,
			expectUpdate:  false,
		},
		{
			name:      "Approve with empty root folder",
			requestID: 123,
			overrides: &RequestOverrides{
				ServerID:   1,
				ServerName: "Radarr",
				RootFolder: "",
			},
			approveStatus: http.StatusOK,
			updateStatus:  http.StatusOK,
			expectError:   false,
			expectUpdate:  false,
		},
		{
			name:      "Approve with root folder override",
			requestID: 123,
			overrides: &RequestOverrides{
				ServerID:   1,
				ServerName: "Radarr",
				RootFolder: "/movies/4k",
			},
			approveStatus: http.StatusOK,
			updateStatus:  http.StatusOK,
			expectError:   false,
			expectUpdate:  true,
		},
		{
			name:      "Approval fails",
			requestID: 123,
			overrides: &RequestOverrides{
				ServerID:   1,
				ServerName: "Radarr",
				RootFolder: "/movies/4k",
			},
			approveStatus:    http.StatusInternalServerError,
			updateStatus:     http.StatusOK,
			expectError:      true,
			expectUpdate:     false,
			expectedErrorMsg: "approval failed",
		},
		{
			name:      "Approval succeeds but update fails",
			requestID: 123,
			overrides: &RequestOverrides{
				ServerID:   1,
				ServerName: "Radarr",
				RootFolder: "/movies/4k",
			},
			approveStatus:    http.StatusOK,
			updateStatus:     http.StatusInternalServerError,
			expectError:      true,
			expectUpdate:     true,
			expectedErrorMsg: "approved but root folder update failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			approveCalled := false
			updateCalled := false
			var updateBody map[string]interface{}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "/approve"):
					approveCalled = true
					if r.Method != "POST" {
						t.Errorf("Expected POST for approve, got %s", r.Method)
					}
					w.WriteHeader(tt.approveStatus)

				case strings.Contains(r.URL.Path, "/request/"):
					updateCalled = true
					if r.Method != "PUT" {
						t.Errorf("Expected PUT for update, got %s", r.Method)
					}
					// Decode body to verify rootFolder is set
					json.NewDecoder(r.Body).Decode(&updateBody)
					w.WriteHeader(tt.updateStatus)

				default:
					t.Errorf("Unexpected endpoint: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			config := Config{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			err := approveRequestWithOverrides(config, tt.requestID, tt.overrides)

			if !approveCalled {
				t.Error("Approve endpoint was not called")
			}

			if tt.expectUpdate != updateCalled {
				t.Errorf("Update called = %v, want %v", updateCalled, tt.expectUpdate)
			}

			if updateCalled {
				if rootFolder, ok := updateBody["rootFolder"].(string); !ok || rootFolder != tt.overrides.RootFolder {
					t.Errorf("Update body rootFolder = %v, want %v", updateBody["rootFolder"], tt.overrides.RootFolder)
				}
			}

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.expectedErrorMsg != "" && !strings.Contains(err.Error(), tt.expectedErrorMsg) {
					t.Errorf("Error message = %q, should contain %q", err.Error(), tt.expectedErrorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

// TestApproveRequestWithOverridesEndpoint verifies the correct API endpoints are called
func TestApproveRequestWithOverridesEndpoint(t *testing.T) {
	requestID := 456
	rootFolder := "/tv/4k"

	approvePath := ""
	updatePath := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/approve") {
			approvePath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		} else if r.Method == "PUT" {
			updatePath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	config := Config{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	overrides := &RequestOverrides{
		ServerID:   1,
		ServerName: "Sonarr",
		RootFolder: rootFolder,
	}

	err := approveRequestWithOverrides(config, requestID, overrides)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expectedApprovePath := fmt.Sprintf("/api/v1/request/%d/approve", requestID)
	if approvePath != expectedApprovePath {
		t.Errorf("Approve path = %q, want %q", approvePath, expectedApprovePath)
	}

	expectedUpdatePath := fmt.Sprintf("/api/v1/request/%d", requestID)
	if updatePath != expectedUpdatePath {
		t.Errorf("Update path = %q, want %q", updatePath, expectedUpdatePath)
	}
}

// TestApproveRequestWithOverridesNilOverrides verifies nil overrides only calls approve
func TestApproveRequestWithOverridesNilOverrides(t *testing.T) {
	approveCalled := false
	updateCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/approve") {
			approveCalled = true
			w.WriteHeader(http.StatusOK)
		} else if r.Method == "PUT" {
			updateCalled = true
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	config := Config{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	err := approveRequestWithOverrides(config, 789, nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !approveCalled {
		t.Error("Approve should have been called")
	}

	if updateCalled {
		t.Error("Update should not have been called with nil overrides")
	}
}
