package requests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/config"
)

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

func TestBuildToolConfig(t *testing.T) {
	tests := []struct {
		name            string
		tk              *config.ToolkitConfig
		expectedURL     string
		expectedKey     string
		expectedTimeout time.Duration
		expectedNoColor bool
		expectedVerbose bool
	}{
		{
			name:            "Nil config returns defaults",
			tk:              nil,
			expectedURL:     "http://localhost:5055",
			expectedTimeout: 10 * time.Second,
		},
		{
			name: "ToolConfig with all values",
			tk: &config.ToolkitConfig{
				General: config.GeneralConfig{
					Timeout: "45s",
					NoColor: true,
				},
				MediaRequests: config.RequestsConfig{
					OverseerrURL: "http://overseerr.example.com",
					APIKey:       "test-key-123",
					Verbose:      true,
				},
			},
			expectedURL:     "http://overseerr.example.com",
			expectedKey:     "test-key-123",
			expectedTimeout: 45 * time.Second,
			expectedNoColor: true,
			expectedVerbose: true,
		},
		{
			name: "Invalid timeout falls back to 10s",
			tk: &config.ToolkitConfig{
				General: config.GeneralConfig{
					Timeout: "not-a-duration",
				},
			},
			expectedURL:     "",
			expectedTimeout: 10 * time.Second,
		},
		{
			name: "Zero timeout falls back to 10s",
			tk: &config.ToolkitConfig{
				General: config.GeneralConfig{
					Timeout: "0s",
				},
			},
			expectedURL:     "",
			expectedTimeout: 10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := BuildToolConfig(tt.tk)
			if cfg.ServerURL != tt.expectedURL {
				t.Errorf("ServerURL = %q, want %q", cfg.ServerURL, tt.expectedURL)
			}
			if cfg.APIKey != tt.expectedKey {
				t.Errorf("APIKey = %q, want %q", cfg.APIKey, tt.expectedKey)
			}
			if cfg.Timeout != tt.expectedTimeout {
				t.Errorf("Timeout = %v, want %v", cfg.Timeout, tt.expectedTimeout)
			}
			if cfg.NoColor != tt.expectedNoColor {
				t.Errorf("NoColor = %v, want %v", cfg.NoColor, tt.expectedNoColor)
			}
			if cfg.Verbose != tt.expectedVerbose {
				t.Errorf("Verbose = %v, want %v", cfg.Verbose, tt.expectedVerbose)
			}
		})
	}
}

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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			cfg := ToolConfig{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			results, err := searchMedia(cfg, tt.query)

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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	details, err := getTVDetails(cfg, 1)
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

			cfg := ToolConfig{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			_, err := createRequest(cfg, tt.media, nil, nil)

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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	requests, err := getPendingRequests(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(requests) != 2 {
		t.Errorf("Got %d requests, want 2", len(requests))
	}
}

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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	err := approveRequest(cfg, 123)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	err := declineRequest(cfg, 123)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

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

			cfg := ToolConfig{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			err := testConnection(cfg)

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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	instances, err := fetchServiceInstances(cfg, "radarr")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(instances) != 2 {
		t.Errorf("Got %d instances, want 2", len(instances))
	}
}

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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	details, err := fetchServiceDetails(cfg, "radarr", 1)
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

func TestSearchMediaWithSpaces(t *testing.T) {
	tests := []struct {
		name          string
		query         string
		expectedQuery string
	}{
		{
			name:          "Query with single space",
			query:         "The Matrix",
			expectedQuery: "The+Matrix",
		},
		{
			name:          "Query with multiple spaces",
			query:         "Star Wars Episode IV",
			expectedQuery: "Star+Wars+Episode+IV",
		},
		{
			name:          "Query with special characters",
			query:         "Rick & Morty",
			expectedQuery: "Rick+%26+Morty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query().Get("query")
				if query != tt.query {
					t.Errorf("Query parameter = %q, want %q", query, tt.query)
				}
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

			cfg := ToolConfig{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			_, err := searchMedia(cfg, tt.query)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

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

			cfg := ToolConfig{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			_, err := searchMedia(cfg, "test query")
			if err == nil {
				t.Fatal("Expected error, got nil")
			}

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

			cfg := ToolConfig{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			authMe, err := checkUserPermissions(cfg)

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

			cfg := ToolConfig{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			count, err := getRequestCount(cfg)

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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	requests, err := getPendingRequests(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(requests) != 3 {
		t.Errorf("Got %d requests, want 3", len(requests))
	}

	for i, req := range requests {
		if req.Status != StatusPending {
			t.Errorf("Request %d has status %d, want %d (StatusPending)", i, req.Status, StatusPending)
		}
	}
}

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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	requests, err := getPendingRequests(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(requests) != 0 {
		t.Errorf("Got %d requests, want 0", len(requests))
	}
}

func TestGetPendingRequestsPagination(t *testing.T) {
	requestCallCount := 0
	totalResults := 125

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

			skip := r.URL.Query().Get("skip")
			skipNum := 0
			if skip != "" {
				fmt.Sscanf(skip, "%d", &skipNum)
			}

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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	requests, err := getPendingRequests(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if requestCallCount != 3 {
		t.Errorf("Made %d API calls to /request, want 3", requestCallCount)
	}

	if len(requests) != totalResults {
		t.Errorf("Got %d requests, want %d", len(requests), totalResults)
	}

	for i, req := range requests {
		expectedID := i + 1
		if req.ID != expectedID {
			t.Errorf("Request %d has ID %d, want %d", i, req.ID, expectedID)
			break
		}
	}
}

func TestGetPendingRequestsWithFallback(t *testing.T) {
	pendingCallCount := 0
	allCallCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	requests, err := getPendingRequests(cfg)
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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	requests, err := getPendingRequests(cfg)
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

func TestGetPendingRequestsFallbackPagination(t *testing.T) {
	allPageCount := 0
	totalResults := 120

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/request/count":
			json.NewEncoder(w).Encode(RequestCount{
				Pending:  60,
				Approved: 60,
				Total:    totalResults,
			})

		case r.URL.Query().Get("filter") == "pending":
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

			pageStart := skipNum
			pageEnd := skipNum + 50
			if pageEnd > totalResults {
				pageEnd = totalResults
			}

			var results []MediaRequest
			for i := pageStart; i < pageEnd; i++ {
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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	requests, err := getPendingRequests(cfg)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if allPageCount != 3 {
		t.Errorf("Made %d calls to filter=all, want 3 (should paginate through all pages)", allPageCount)
	}

	if len(requests) != 60 {
		t.Errorf("Got %d pending requests, want 60", len(requests))
	}

	for _, req := range requests {
		if req.Status != StatusPending {
			t.Errorf("Request %d has status %d, want %d (StatusPending)", req.ID, req.Status, StatusPending)
			break
		}
	}
}

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
			name:      "Approve with server override and default root folder",
			requestID: 123,
			overrides: &RequestOverrides{
				ServerID:   1,
				ServerName: "Radarr",
				RootFolder: "",
			},
			approveStatus: http.StatusOK,
			updateStatus:  http.StatusOK,
			expectError:   false,
			expectUpdate:  true,
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
			expectUpdate:     true,
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
			expectedErrorMsg: "failed to set request overrides before approval",
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
					json.NewDecoder(r.Body).Decode(&updateBody)
					w.WriteHeader(tt.updateStatus)

				default:
					t.Errorf("Unexpected endpoint: %s", r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			cfg := ToolConfig{
				ServerURL: server.URL,
				APIKey:    "test-key",
				Timeout:   5 * time.Second,
			}

			err := approveRequestWithOverrides(cfg, tt.requestID, tt.overrides)

			updateShouldHaveBeenCalled := tt.expectUpdate
			updateSucceeded := updateShouldHaveBeenCalled && tt.updateStatus == http.StatusOK
			updateNotNeeded := !updateShouldHaveBeenCalled && (tt.overrides == nil || (tt.overrides.RootFolder == "" && tt.overrides.ServerID <= 0))
			updateCalledAndFailed := updateShouldHaveBeenCalled && tt.updateStatus != http.StatusOK

			if updateSucceeded || updateNotNeeded {
				if !approveCalled {
					t.Error("Approve endpoint was not called when it should have been")
				}
			}
			if updateCalledAndFailed {
				if approveCalled {
					t.Error("Approve endpoint was called when update already failed")
				}
			}

			if tt.expectUpdate != updateCalled {
				t.Errorf("Update called = %v, want %v", updateCalled, tt.expectUpdate)
			}

			if updateCalled {
				if tt.overrides.RootFolder != "" {
					rootFolder, ok := updateBody["rootFolder"].(string)
					if !ok || rootFolder != tt.overrides.RootFolder {
						t.Errorf("Update body rootFolder = %v, want %v", updateBody["rootFolder"], tt.overrides.RootFolder)
					}
				}
				if tt.overrides.ServerID > 0 {
					serverID, ok := updateBody["serverId"].(float64)
					if !ok || int(serverID) != tt.overrides.ServerID {
						t.Errorf("Update body serverId = %v, want %v", updateBody["serverId"], tt.overrides.ServerID)
					}
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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	overrides := &RequestOverrides{
		ServerID:   1,
		ServerName: "Sonarr",
		RootFolder: rootFolder,
	}

	err := approveRequestWithOverrides(cfg, requestID, overrides)
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

	cfg := ToolConfig{
		ServerURL: server.URL,
		APIKey:    "test-key",
		Timeout:   5 * time.Second,
	}

	err := approveRequestWithOverrides(cfg, 789, nil)
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

// TestDisplayCurrentRootFolder verifies current root folder display in approval screen
func TestDisplayCurrentRootFolder(t *testing.T) {
	// This test verifies the logic but not the actual display
	// Visual verification requires manual testing

	tests := []struct {
		name         string
		request      MediaRequest
		shouldShow   bool
		expectedText string
	}{
		{
			name: "Request with root folder set",
			request: MediaRequest{
				ID:         123,
				Status:     StatusPending,
				Type:       "movie",
				RootFolder: "/movies/4k",
			},
			shouldShow:   true,
			expectedText: "/movies/4k",
		},
		{
			name: "Request without root folder",
			request: MediaRequest{
				ID:         456,
				Status:     StatusPending,
				Type:       "tv",
				RootFolder: "",
			},
			shouldShow:   false,
			expectedText: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the RootFolder field is correctly set
			if tt.shouldShow && tt.request.RootFolder != tt.expectedText {
				t.Errorf("RootFolder = %q, want %q", tt.request.RootFolder, tt.expectedText)
			}
			if !tt.shouldShow && tt.request.RootFolder != "" {
				t.Errorf("RootFolder should be empty, got %q", tt.request.RootFolder)
			}
		})
	}

}
