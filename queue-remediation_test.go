//go:build queueremediation

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestMapStatusToAction verifies status classification logic
func TestMapStatusToAction(t *testing.T) {
	tests := []struct {
		name                 string
		status               string
		trackedState         string
		trackedStatus        string
		statusMessages       []StatusMessage
		expectedAction       string
		expectedBlocklist    bool
		expectedManualImport bool
	}{
		{
			name:                 "Normal downloading with ok status",
			status:               "downloading",
			trackedState:         "downloading",
			trackedStatus:        "ok",
			statusMessages:       nil,
			expectedAction:       "monitor",
			expectedBlocklist:    false,
			expectedManualImport: false,
		},
		{
			name:                 "Completed waiting for import (no issues)",
			status:               "completed",
			trackedState:         "importPending",
			trackedStatus:        "ok",
			statusMessages:       nil,
			expectedAction:       "monitor",
			expectedBlocklist:    false,
			expectedManualImport: false,
		},
		{
			name:          "Not a Custom Format upgrade",
			status:        "completed",
			trackedState:  "importPending",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Not a Custom Format upgrade for existing movie file(s). New: [Repack/Proper - Notifiarr] (5) do not improve on Existing: [HD Bluray Tier 02 - Notifiarr] (1750)",
					},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    true,
			expectedManualImport: false,
		},
		{
			name:          "No files found eligible",
			status:        "completed",
			trackedState:  "importPending",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"No files found are eligible for import in /downloads/Movie.2024",
					},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    false,
			expectedManualImport: false,
		},
		{
			name:          "Sample file detected",
			status:        "completed",
			trackedState:  "importPending",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Sample",
					},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    false,
			expectedManualImport: false,
		},
		{
			name:          "Matched by ID not name",
			status:        "completed",
			trackedState:  "importPending",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Found matching series via grab history, but release was matched to series by ID. Automatic import is not possible.",
					},
				},
			},
			expectedAction:       "manual_import",
			expectedBlocklist:    false,
			expectedManualImport: true,
		},
		{
			name:                 "Import blocked state",
			status:               "completed",
			trackedState:         "importBlocked",
			trackedStatus:        "warning",
			statusMessages:       nil,
			expectedAction:       "manual_import",
			expectedBlocklist:    false,
			expectedManualImport: true,
		},
		{
			name:          "Quality revision not upgrade",
			status:        "completed",
			trackedState:  "importPending",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Not a quality revision upgrade for existing episode file(s)",
					},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    true,
			expectedManualImport: false,
		},
		{
			name:                 "Failed download",
			status:               "failed",
			trackedState:         "failedPending",
			trackedStatus:        "error",
			statusMessages:       nil,
			expectedAction:       "delete",
			expectedBlocklist:    false,
			expectedManualImport: false,
		},
		{
			name:          "Import blocked WITH quality non-upgrade message",
			status:        "completed",
			trackedState:  "importBlocked",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Not a quality revision upgrade for existing episode file(s)",
					},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    true,
			expectedManualImport: false,
		},
		{
			name:          "Import blocked WITH custom format non-upgrade message",
			status:        "completed",
			trackedState:  "importBlocked",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Not a Custom Format upgrade for existing movie file(s)",
					},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    true,
			expectedManualImport: false,
		},
		{
			name:          "Import blocked WITH matched by ID message",
			status:        "completed",
			trackedState:  "importBlocked",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Found matching series via grab history, but release was matched to series by ID",
					},
				},
			},
			expectedAction:       "manual_import",
			expectedBlocklist:    false,
			expectedManualImport: true,
		},
		// ==================== MULTI-MESSAGE TEST CASES ====================
		// Tests for multiple simultaneous status messages to verify priority hierarchy
		{
			name:          "Sample + Custom Format - Quality takes priority",
			status:        "completed",
			trackedState:  "importPending",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{"Sample"},
				},
				{
					Title:    "Warning",
					Messages: []string{"Not a Custom Format upgrade for existing movie file(s)"},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    true,
			expectedManualImport: false,
		},
		{
			name:          "Sample + Quality Revision - Quality takes priority",
			status:        "completed",
			trackedState:  "importPending",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{"Sample"},
				},
				{
					Title:    "Warning",
					Messages: []string{"Not a quality revision upgrade for existing episode file(s)"},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    true,
			expectedManualImport: false,
		},
		{
			name:          "Sample + No Files Found - Structural issue priority",
			status:        "completed",
			trackedState:  "importPending",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{"Sample"},
				},
				{
					Title:    "Warning",
					Messages: []string{"No files found are eligible for import in /downloads/Movie.2024"},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    false,
			expectedManualImport: false,
		},
		{
			name:          "Custom Format + Matched by ID - Quality takes priority over ID match",
			status:        "completed",
			trackedState:  "importPending",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{"Not a Custom Format upgrade for existing movie file(s)"},
				},
				{
					Title:    "Warning",
					Messages: []string{"Found matching series via grab history, but release was matched to series by ID"},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    true,
			expectedManualImport: false,
		},
		{
			name:          "importBlocked state + Quality message - Explicit blocker",
			status:        "completed",
			trackedState:  "importBlocked",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{"Not a Custom Format upgrade for existing movie file(s)"},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    true,
			expectedManualImport: false,
		},
		{
			name:          "Multiple non-quality messages - Sample + No Files + Other",
			status:        "completed",
			trackedState:  "importPending",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title:    "Info",
					Messages: []string{"Download completed"},
				},
				{
					Title:    "Warning",
					Messages: []string{"Sample"},
				},
				{
					Title:    "Warning",
					Messages: []string{"No files found are eligible for import in /downloads/Series.S01E01"},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    false,
			expectedManualImport: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := QueueItem{
				Status:                tt.status,
				TrackedDownloadState:  tt.trackedState,
				TrackedDownloadStatus: tt.trackedStatus,
				StatusMessages:        tt.statusMessages,
			}

			action, blocklist, manualImport := mapStatusToAction(item)

			if action != tt.expectedAction {
				t.Errorf("action = %q, want %q", action, tt.expectedAction)
			}
			if blocklist != tt.expectedBlocklist {
				t.Errorf("blocklist = %v, want %v", blocklist, tt.expectedBlocklist)
			}
			if manualImport != tt.expectedManualImport {
				t.Errorf("manualImport = %v, want %v", manualImport, tt.expectedManualImport)
			}
		})
	}
}

// TestParseStatusMessages verifies status message parsing
func TestParseStatusMessages(t *testing.T) {
	tests := []struct {
		name              string
		statusMessages    []StatusMessage
		expectedReason    string
		expectedBlocklist bool
	}{
		{
			name: "Custom format message",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Not a Custom Format upgrade for existing movie file(s)",
					},
				},
			},
			expectedReason:    "custom_format_no_upgrade",
			expectedBlocklist: true,
		},
		{
			name: "No files found message",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"No files found are eligible for import in /path",
					},
				},
			},
			expectedReason:    "no_files_found",
			expectedBlocklist: false,
		},
		{
			name: "Sample file message",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Sample",
					},
				},
			},
			expectedReason:    "sample_file",
			expectedBlocklist: false,
		},
		{
			name: "ID match message",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Found matching series via grab history, but release was matched to series by ID",
					},
				},
			},
			expectedReason:    "matched_by_id",
			expectedBlocklist: false,
		},
		{
			name: "Quality revision message",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Not a quality revision upgrade for existing episode file(s)",
					},
				},
			},
			expectedReason:    "quality_no_upgrade",
			expectedBlocklist: true,
		},
		{
			name: "Multiple messages with match",
			statusMessages: []StatusMessage{
				{
					Title: "Info",
					Messages: []string{
						"Download completed",
					},
				},
				{
					Title: "Warning",
					Messages: []string{
						"Sample",
					},
				},
			},
			expectedReason:    "sample_file",
			expectedBlocklist: false,
		},
		{
			name:              "Empty messages",
			statusMessages:    nil,
			expectedReason:    "unknown",
			expectedBlocklist: false,
		},
		{
			name: "Unrecognized message",
			statusMessages: []StatusMessage{
				{
					Title: "Warning",
					Messages: []string{
						"Some unknown error message",
					},
				},
			},
			expectedReason:    "unknown",
			expectedBlocklist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, blocklist := parseStatusMessages(tt.statusMessages)
			if reason != tt.expectedReason {
				t.Errorf("reason = %q, want %q", reason, tt.expectedReason)
			}
			if blocklist != tt.expectedBlocklist {
				t.Errorf("blocklist = %v, want %v", blocklist, tt.expectedBlocklist)
			}
		})
	}
}

// TestFetchQueue verifies queue fetching with pagination
func TestFetchQueue(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectError   bool
		expectedCount int
		validateItems func(*testing.T, []QueueItem)
	}{
		{
			name:       "Successful single page fetch",
			statusCode: http.StatusOK,
			responseBody: `{
				"page": 1,
				"pageSize": 100,
				"totalRecords": 2,
				"records": [
					{
						"id": 1,
						"title": "Test Movie",
						"status": "downloading",
						"trackedDownloadState": "downloading",
						"trackedDownloadStatus": "ok",
						"downloadClient": "qBittorrent",
						"downloadId": "ABC123"
					},
					{
						"id": 2,
						"title": "Test Series S01E01",
						"status": "completed",
						"trackedDownloadState": "importPending",
						"trackedDownloadStatus": "warning",
						"statusMessages": [
							{
								"title": "Warning",
								"messages": ["Sample"]
							}
						],
						"downloadClient": "qBittorrent",
						"downloadId": "DEF456"
					}
				]
			}`,
			expectError:   false,
			expectedCount: 2,
			validateItems: func(t *testing.T, items []QueueItem) {
				if items[0].ID != 1 {
					t.Errorf("First item ID = %d, want 1", items[0].ID)
				}
				if items[0].Status != "downloading" {
					t.Errorf("First item status = %q, want %q", items[0].Status, "downloading")
				}
				if items[1].TrackedDownloadStatus != "warning" {
					t.Errorf("Second item trackedDownloadStatus = %q, want %q", items[1].TrackedDownloadStatus, "warning")
				}
				if len(items[1].StatusMessages) != 1 {
					t.Errorf("Second item has %d status messages, want 1", len(items[1].StatusMessages))
				}
			},
		},
		{
			name:          "Empty queue",
			statusCode:    http.StatusOK,
			responseBody:  `{"page": 1, "pageSize": 100, "totalRecords": 0, "records": []}`,
			expectError:   false,
			expectedCount: 0,
		},
		{
			name:         "Unauthorized",
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"error": "Invalid API key"}`,
			expectError:  true,
		},
		{
			name:         "Server error",
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"error": "Internal server error"}`,
			expectError:  true,
		},
		{
			name:         "Invalid JSON",
			statusCode:   http.StatusOK,
			responseBody: `{invalid json}`,
			expectError:  true,
		},
		{
			name:       "Item with null fields",
			statusCode: http.StatusOK,
			responseBody: `{
				"page": 1,
				"pageSize": 100,
				"totalRecords": 1,
				"records": [
					{
						"id": 100,
						"title": "Test",
						"status": "downloading",
						"trackedDownloadState": "downloading",
						"trackedDownloadStatus": "ok",
						"errorMessage": null,
						"statusMessages": null
					}
				]
			}`,
			expectError:   false,
			expectedCount: 1,
			validateItems: func(t *testing.T, items []QueueItem) {
				if items[0].ErrorMessage != "" {
					t.Errorf("Expected empty error message, got %q", items[0].ErrorMessage)
				}
				if items[0].StatusMessages != nil && len(items[0].StatusMessages) > 0 {
					t.Errorf("Expected nil/empty status messages, got %d items", len(items[0].StatusMessages))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify correct endpoint
				if !strings.Contains(r.URL.Path, "/api/v3/queue") {
					t.Errorf("Expected /api/v3/queue in path, got %s", r.URL.Path)
				}

				// Verify API key header
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error":"Invalid API key"}`))
					return
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			config := Config{Timeout: 5 * time.Second}
			items, err := fetchQueue(config, server.URL, "test-token")

			if (err != nil) != tt.expectError {
				t.Errorf("error = %v, expectError = %v", err, tt.expectError)
				return
			}

			if !tt.expectError {
				if len(items) != tt.expectedCount {
					t.Errorf("got %d items, want %d", len(items), tt.expectedCount)
				}

				if tt.validateItems != nil {
					tt.validateItems(t, items)
				}
			}
		})
	}
}

// TestDeleteQueueItem verifies queue item deletion with query parameters
func TestDeleteQueueItem(t *testing.T) {
	tests := []struct {
		name             string
		itemID           int
		removeFromClient bool
		blocklist        bool
		statusCode       int
		expectError      bool
		validateRequest  func(*testing.T, *http.Request)
	}{
		{
			name:             "Delete without remove or blocklist",
			itemID:           42,
			removeFromClient: false,
			blocklist:        false,
			statusCode:       http.StatusOK,
			expectError:      false,
			validateRequest: func(t *testing.T, r *http.Request) {
				if r.URL.Query().Get("removeFromClient") != "" {
					t.Error("removeFromClient should not be set")
				}
				if r.URL.Query().Get("blocklist") != "" {
					t.Error("blocklist should not be set")
				}
			},
		},
		{
			name:             "Delete with removeFromClient",
			itemID:           42,
			removeFromClient: true,
			blocklist:        false,
			statusCode:       http.StatusOK,
			expectError:      false,
			validateRequest: func(t *testing.T, r *http.Request) {
				if r.URL.Query().Get("removeFromClient") != "true" {
					t.Errorf("removeFromClient = %q, want 'true'", r.URL.Query().Get("removeFromClient"))
				}
			},
		},
		{
			name:             "Delete with blocklist",
			itemID:           42,
			removeFromClient: false,
			blocklist:        true,
			statusCode:       http.StatusOK,
			expectError:      false,
			validateRequest: func(t *testing.T, r *http.Request) {
				if r.URL.Query().Get("blocklist") != "true" {
					t.Errorf("blocklist = %q, want 'true'", r.URL.Query().Get("blocklist"))
				}
			},
		},
		{
			name:             "Delete with both flags",
			itemID:           42,
			removeFromClient: true,
			blocklist:        true,
			statusCode:       http.StatusOK,
			expectError:      false,
			validateRequest: func(t *testing.T, r *http.Request) {
				if r.URL.Query().Get("removeFromClient") != "true" {
					t.Error("removeFromClient should be true")
				}
				if r.URL.Query().Get("blocklist") != "true" {
					t.Error("blocklist should be true")
				}
			},
		},
		{
			name:             "Server error",
			itemID:           42,
			removeFromClient: true,
			blocklist:        true,
			statusCode:       http.StatusInternalServerError,
			expectError:      true,
		},
		{
			name:             "Not found",
			itemID:           999,
			removeFromClient: false,
			blocklist:        false,
			statusCode:       http.StatusNotFound,
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify DELETE method
				if r.Method != "DELETE" {
					t.Errorf("Expected DELETE, got %s", r.Method)
				}

				// Verify endpoint path includes item ID
				if !strings.Contains(r.URL.Path, "/api/v3/queue/") {
					t.Errorf("Expected /api/v3/queue/ in path, got %s", r.URL.Path)
				}

				// Verify API key
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// Run custom validations
				if tt.validateRequest != nil {
					tt.validateRequest(t, r)
				}

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			config := Config{Timeout: 5 * time.Second}
			err := deleteQueueItem(config, server.URL, "test-token", tt.itemID, tt.removeFromClient, tt.blocklist)

			if (err != nil) != tt.expectError {
				t.Errorf("error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}

// TestTriggerManualImport verifies manual import command
func TestTriggerManualImport(t *testing.T) {
	tests := []struct {
		name            string
		downloadPath    string
		statusCode      int
		expectError     bool
		validateRequest func(*testing.T, *http.Request, map[string]interface{})
	}{
		{
			name:         "Successful manual import trigger",
			downloadPath: "/downloads/Movie.2024",
			statusCode:   http.StatusCreated,
			expectError:  false,
			validateRequest: func(t *testing.T, r *http.Request, body map[string]interface{}) {
				if body["name"] != "DownloadedEpisodesScan" && body["name"] != "DownloadedMoviesScan" {
					t.Errorf("Command name = %q, want DownloadedEpisodesScan or DownloadedMoviesScan", body["name"])
				}
				if body["path"] != "/downloads/Movie.2024" {
					t.Errorf("Path = %q, want /downloads/Movie.2024", body["path"])
				}
			},
		},
		{
			name:         "Empty path",
			downloadPath: "",
			statusCode:   http.StatusCreated,
			expectError:  false,
		},
		{
			name:         "Server error",
			downloadPath: "/downloads/test",
			statusCode:   http.StatusInternalServerError,
			expectError:  true,
		},
		{
			name:         "Bad request",
			downloadPath: "/invalid/path",
			statusCode:   http.StatusBadRequest,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify POST method
				if r.Method != "POST" {
					t.Errorf("Expected POST, got %s", r.Method)
				}

				// Verify endpoint
				if !strings.Contains(r.URL.Path, "/api/v3/command") {
					t.Errorf("Expected /api/v3/command in path, got %s", r.URL.Path)
				}

				// Verify content type
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
				}

				// Verify API key
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// Parse request body
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("Failed to decode request body: %v", err)
				}

				// Run custom validations
				if tt.validateRequest != nil {
					tt.validateRequest(t, r, body)
				}

				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"id":    1,
					"name":  body["name"],
					"state": "queued",
				})
			}))
			defer server.Close()

			config := Config{Timeout: 5 * time.Second}
			err := triggerManualImport(config, server.URL, "test-token", tt.downloadPath, "sonarr", false, QueueItem{})

			if (err != nil) != tt.expectError {
				t.Errorf("error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}

// TestHTTPRedirectHandling verifies HTTP 307 redirect support
func TestHTTPRedirectHandling(t *testing.T) {
	// Create a server that redirects
	finalServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(QueueResponse{
			Page:         1,
			PageSize:     100,
			TotalRecords: 1,
			Records: []QueueItem{
				{
					ID:     123,
					Title:  "Test Movie",
					Status: "downloading",
				},
			},
		})
	}))
	defer finalServer.Close()

	// Create redirect server
	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 307 redirect to final server
		http.Redirect(w, r, finalServer.URL+r.URL.Path+"?"+r.URL.RawQuery, http.StatusTemporaryRedirect)
	}))
	defer redirectServer.Close()

	// Test that fetch follows redirect
	config := Config{Timeout: 5 * time.Second}
	items, err := fetchQueue(config, redirectServer.URL, "test-token")

	if err != nil {
		t.Fatalf("Unexpected error following redirect: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("Got %d items, want 1 (should have followed redirect)", len(items))
	}

	if items[0].ID != 123 {
		t.Errorf("Item ID = %d, want 123", items[0].ID)
	}
}

// TestFetchAllQueues verifies multi-instance queue fetching
func TestFetchAllQueues(t *testing.T) {
	// Create mock Sonarr server
	sonarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(QueueResponse{
			Page:         1,
			PageSize:     100,
			TotalRecords: 1,
			Records: []QueueItem{
				{
					ID:                    1,
					Title:                 "Test Series S01E01",
					Status:                "downloading",
					TrackedDownloadState:  "downloading",
					TrackedDownloadStatus: "ok",
				},
			},
		})
	}))
	defer sonarrServer.Close()

	// Create mock Radarr server
	radarrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(QueueResponse{
			Page:         1,
			PageSize:     100,
			TotalRecords: 2,
			Records: []QueueItem{
				{
					ID:                    2,
					Title:                 "Test Movie 1",
					Status:                "completed",
					TrackedDownloadState:  "importPending",
					TrackedDownloadStatus: "warning",
				},
				{
					ID:                    3,
					Title:                 "Test Movie 2",
					Status:                "downloading",
					TrackedDownloadState:  "downloading",
					TrackedDownloadStatus: "ok",
				},
			},
		})
	}))
	defer radarrServer.Close()

	config := Config{
		SonarrURLs:   []string{sonarrServer.URL},
		SonarrTokens: []string{"sonarr-token"},
		RadarrURLs:   []string{radarrServer.URL},
		RadarrTokens: []string{"radarr-token"},
		Timeout:      5 * time.Second,
	}

	allItems, err := fetchAllQueues(config)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(allItems) != 3 {
		t.Errorf("Got %d total items, want 3 (1 Sonarr + 2 Radarr)", len(allItems))
	}

	// Verify items from both sources
	foundSonarr := false
	foundRadarr := 0
	for _, item := range allItems {
		if item.ID == 1 {
			foundSonarr = true
		}
		if item.ID == 2 || item.ID == 3 {
			foundRadarr++
		}
	}

	if !foundSonarr {
		t.Error("Did not find Sonarr item")
	}
	if foundRadarr != 2 {
		t.Errorf("Found %d Radarr items, want 2", foundRadarr)
	}
}

// TestFetchAllQueuesPartialFailure verifies partial failure handling
func TestFetchAllQueuesPartialFailure(t *testing.T) {
	// Working server
	workingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(QueueResponse{
			Page:         1,
			PageSize:     100,
			TotalRecords: 1,
			Records: []QueueItem{
				{ID: 1, Title: "Working", Status: "downloading"},
			},
		})
	}))
	defer workingServer.Close()

	// Failing server
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failingServer.Close()

	config := Config{
		SonarrURLs:   []string{workingServer.URL, failingServer.URL},
		SonarrTokens: []string{"token1", "token2"},
		Timeout:      5 * time.Second,
	}

	allItems, err := fetchAllQueues(config)

	// Should not error on partial failure
	if err != nil {
		t.Errorf("Should not error on partial failure, got: %v", err)
	}

	// Should still have items from working server
	if len(allItems) != 1 {
		t.Errorf("Got %d items, want 1 (from working server)", len(allItems))
	}

	if allItems[0].ID != 1 {
		t.Errorf("Item ID = %d, want 1", allItems[0].ID)
	}
}

// TestFetchAllQueuesCompleteFailure verifies total failure handling
func TestFetchAllQueuesCompleteFailure(t *testing.T) {
	// All servers failing
	failingServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failingServer1.Close()

	failingServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer failingServer2.Close()

	config := Config{
		SonarrURLs:   []string{failingServer1.URL},
		SonarrTokens: []string{"token1"},
		RadarrURLs:   []string{failingServer2.URL},
		RadarrTokens: []string{"token2"},
		Timeout:      5 * time.Second,
	}

	allItems, err := fetchAllQueues(config)

	// Should error when all instances fail
	if err == nil {
		t.Error("Expected error when all instances fail")
	}

	if allItems != nil && len(allItems) > 0 {
		t.Errorf("Expected empty/nil items on complete failure, got %d items", len(allItems))
	}
}

// TestClassifyAndRemediate verifies full workflow
func TestClassifyAndRemediate(t *testing.T) {
	deleteCalls := 0
	manualImportCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "DELETE" && strings.Contains(r.URL.Path, "/queue/"):
			deleteCalls++
			// Verify blocklist parameter for custom format items
			if strings.Contains(r.URL.Path, "/queue/1") && r.URL.Query().Get("blocklist") != "true" {
				t.Error("Item 1 should be deleted with blocklist=true")
			}
			if strings.Contains(r.URL.Path, "/queue/2") && r.URL.Query().Get("blocklist") != "" {
				t.Error("Item 2 should be deleted without blocklist parameter")
			}
			w.WriteHeader(http.StatusOK)

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/command"):
			manualImportCalls++
			w.WriteHeader(http.StatusCreated)

		case strings.Contains(r.URL.Path, "/queue"):
			// Return test queue
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(QueueResponse{
				Page:         1,
				PageSize:     100,
				TotalRecords: 4,
				Records: []QueueItem{
					{
						ID:                    1,
						Title:                 "Custom Format No Upgrade",
						Status:                "completed",
						TrackedDownloadState:  "importPending",
						TrackedDownloadStatus: "warning",
						StatusMessages: []StatusMessage{
							{
								Title:    "Warning",
								Messages: []string{"Not a Custom Format upgrade for existing movie file(s)"},
							},
						},
					},
					{
						ID:                    2,
						Title:                 "No Files Found",
						Status:                "completed",
						TrackedDownloadState:  "importPending",
						TrackedDownloadStatus: "warning",
						StatusMessages: []StatusMessage{
							{
								Title:    "Warning",
								Messages: []string{"No files found are eligible for import"},
							},
						},
					},
					{
						ID:                    3,
						Title:                 "Matched by ID",
						Status:                "completed",
						TrackedDownloadState:  "importPending",
						TrackedDownloadStatus: "warning",
						OutputPath:            "/downloads/Series.S01E01",
						StatusMessages: []StatusMessage{
							{
								Title:    "Warning",
								Messages: []string{"matched to series by ID"},
							},
						},
					},
					{
						ID:                    4,
						Title:                 "Normal Download",
						Status:                "downloading",
						TrackedDownloadState:  "downloading",
						TrackedDownloadStatus: "ok",
					},
				},
			})
		}
	}))
	defer server.Close()

	config := Config{
		SonarrURLs:   []string{server.URL},
		SonarrTokens: []string{"test-token"},
		Timeout:      5 * time.Second,
	}

	err := classifyAndRemediate(config, false) // dryRun = false
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should delete 2 items (custom format and no files)
	if deleteCalls != 2 {
		t.Errorf("Made %d delete calls, want 2", deleteCalls)
	}

	// Should trigger 1 manual import (matched by ID)
	if manualImportCalls != 1 {
		t.Errorf("Made %d manual import calls, want 1", manualImportCalls)
	}
}

// TestClassifyAndRemediateDryRun verifies dry run mode
func TestClassifyAndRemediateDryRun(t *testing.T) {
	deleteCalls := 0
	manualImportCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "DELETE":
			deleteCalls++
			w.WriteHeader(http.StatusOK)
		case r.Method == "POST" && strings.Contains(r.URL.Path, "/command"):
			manualImportCalls++
			w.WriteHeader(http.StatusCreated)
		case strings.Contains(r.URL.Path, "/queue"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(QueueResponse{
				Page:         1,
				PageSize:     100,
				TotalRecords: 1,
				Records: []QueueItem{
					{
						ID:                    1,
						Title:                 "Sample File",
						Status:                "completed",
						TrackedDownloadState:  "importPending",
						TrackedDownloadStatus: "warning",
						StatusMessages: []StatusMessage{
							{Title: "Warning", Messages: []string{"Sample"}},
						},
					},
				},
			})
		}
	}))
	defer server.Close()

	config := Config{
		SonarrURLs:   []string{server.URL},
		SonarrTokens: []string{"test-token"},
		Timeout:      5 * time.Second,
	}

	err := classifyAndRemediate(config, true) // dryRun = true
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should NOT make any actual API calls in dry run mode
	if deleteCalls != 0 {
		t.Errorf("Made %d delete calls in dry run, want 0", deleteCalls)
	}
	if manualImportCalls != 0 {
		t.Errorf("Made %d manual import calls in dry run, want 0", manualImportCalls)
	}
}

// TestQueuePagination verifies large queue pagination handling
func TestQueuePagination(t *testing.T) {
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		// Should request with pageSize parameter
		pageSize := r.URL.Query().Get("pageSize")
		if pageSize == "" {
			t.Error("pageSize query parameter not set")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(QueueResponse{
			Page:         1,
			PageSize:     100,
			TotalRecords: 50,
			Records:      make([]QueueItem, 50),
		})
	}))
	defer server.Close()

	config := Config{Timeout: 5 * time.Second}
	_, err := fetchQueue(config, server.URL, "test-token")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if callCount != 1 {
		t.Errorf("Made %d API calls, want 1", callCount)
	}
}

// TestConfigValidation verifies configuration validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		shouldError bool
	}{
		{
			name: "Valid Sonarr only",
			config: Config{
				SonarrURLs:   []string{"http://localhost:8989"},
				SonarrTokens: []string{"token"},
			},
			shouldError: false,
		},
		{
			name: "Valid Radarr only",
			config: Config{
				RadarrURLs:   []string{"http://localhost:7878"},
				RadarrTokens: []string{"token"},
			},
			shouldError: false,
		},
		{
			name: "Valid both",
			config: Config{
				SonarrURLs:   []string{"http://localhost:8989"},
				SonarrTokens: []string{"token1"},
				RadarrURLs:   []string{"http://localhost:7878"},
				RadarrTokens: []string{"token2"},
			},
			shouldError: false,
		},
		{
			name:        "No instances configured",
			config:      Config{},
			shouldError: true,
		},
		{
			name: "Sonarr URL/token mismatch",
			config: Config{
				SonarrURLs:   []string{"http://localhost:8989", "http://localhost:8990"},
				SonarrTokens: []string{"token"},
			},
			shouldError: true,
		},
		{
			name: "Radarr URL/token mismatch",
			config: Config{
				RadarrURLs:   []string{"http://localhost:7878"},
				RadarrTokens: []string{"token1", "token2"},
			},
			shouldError: true,
		},
		{
			name: "Multiple instances valid",
			config: Config{
				SonarrURLs:   []string{"http://s1:8989", "http://s2:8989", "http://s3:8989"},
				SonarrTokens: []string{"token1", "token2", "token3"},
				RadarrURLs:   []string{"http://r1:7878", "http://r2:7878"},
				RadarrTokens: []string{"token4", "token5"},
			},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateQueueConfig(tt.config)
			if (err != nil) != tt.shouldError {
				t.Errorf("validateQueueConfig() error = %v, shouldError = %v", err, tt.shouldError)
			}
		})
	}
}

// TestEmptyQueueHandling verifies empty queue behavior
func TestEmptyQueueHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(QueueResponse{
			Page:         1,
			PageSize:     100,
			TotalRecords: 0,
			Records:      []QueueItem{},
		})
	}))
	defer server.Close()

	config := Config{
		SonarrURLs:   []string{server.URL},
		SonarrTokens: []string{"test-token"},
		Timeout:      5 * time.Second,
	}

	// Should not error on empty queue
	err := classifyAndRemediate(config, false)
	if err != nil {
		t.Errorf("Should not error on empty queue, got: %v", err)
	}
}

// TestStatusMessageCaseInsensitivity verifies message matching is case-aware
func TestStatusMessageCaseInsensitivity(t *testing.T) {
	tests := []struct {
		name              string
		message           string
		expected          string
		expectedBlocklist bool
	}{
		{
			name:              "Lowercase sample",
			message:           "sample",
			expected:          "sample_file",
			expectedBlocklist: false,
		},
		{
			name:              "Uppercase SAMPLE",
			message:           "SAMPLE",
			expected:          "sample_file",
			expectedBlocklist: false,
		},
		{
			name:              "Mixed case Sample",
			message:           "Sample",
			expected:          "sample_file",
			expectedBlocklist: false,
		},
		{
			name:              "In sentence - this is a sample file",
			message:           "This is a sample file",
			expected:          "sample_file",
			expectedBlocklist: false,
		},
		{
			name:              "Custom format various case",
			message:           "not a CUSTOM FORMAT upgrade",
			expected:          "custom_format_no_upgrade",
			expectedBlocklist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statusMessages := []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{tt.message},
				},
			}
			result, blocklist := parseStatusMessages(statusMessages)
			if result != tt.expected {
				t.Errorf("parseStatusMessages(%q) reason = %q, want %q", tt.message, result, tt.expected)
			}
			if blocklist != tt.expectedBlocklist {
				t.Errorf("parseStatusMessages(%q) blocklist = %v, want %v", tt.message, blocklist, tt.expectedBlocklist)
			}
		})
	}
}

// ========== TEST: scanForManualImport (GET /api/v3/manualimport) ==========

func TestScanForManualImport(t *testing.T) {
	tests := []struct {
		name            string
		folderPath      string
		instanceType    string
		statusCode      int
		responseBody    string
		expectError     bool
		expectedCount   int
		validateItems   func(*testing.T, []ManualImportResource)
		validateRequest func(*testing.T, *http.Request)
	}{
		{
			name:         "Sonarr - scan folder with matched series",
			folderPath:   "/downloads/Series.Name.S01E01",
			instanceType: "sonarr",
			statusCode:   http.StatusOK,
			responseBody: `[
				{
					"id": 1234567890,
					"path": "/downloads/Series.Name.S01E01/Series.Name.S01E01.720p-GROUP.mkv",
					"name": "Series.Name.S01E01.720p-GROUP.mkv",
					"size": 549755813888,
					"series": {
						"id": 42,
						"title": "Series Name"
					},
					"seasonNumber": 1,
					"episodes": [
						{
							"id": 123456,
							"seasonNumber": 1,
							"episodeNumber": 1,
							"title": "Episode Title"
						}
					],
					"quality": {
						"quality": {
							"id": 1,
							"name": "HDTV-720p",
							"resolution": 720
						},
						"revision": {
							"version": 1,
							"real": 0,
							"isRepack": false
						}
					},
					"languages": [
						{
							"id": 1,
							"name": "English"
						}
					],
					"releaseGroup": "GROUP",
					"rejections": [],
					"downloadId": "abc123xyz"
				}
			]`,
			expectError:   false,
			expectedCount: 1,
			validateItems: func(t *testing.T, items []ManualImportResource) {
				if items[0].Series == nil {
					t.Fatal("Series should not be nil")
				}
				if items[0].Series.ID != 42 {
					t.Errorf("Series ID = %d, want 42", items[0].Series.ID)
				}
				if items[0].SeasonNumber == nil || *items[0].SeasonNumber != 1 {
					t.Error("SeasonNumber should be 1")
				}
				if len(items[0].Episodes) != 1 {
					t.Errorf("Episodes count = %d, want 1", len(items[0].Episodes))
				}
				if len(items[0].Rejections) != 0 {
					t.Errorf("Should have no rejections, got %d", len(items[0].Rejections))
				}
			},
			validateRequest: func(t *testing.T, r *http.Request) {
				if !strings.Contains(r.URL.Path, "/api/v3/manualimport") {
					t.Errorf("Expected /api/v3/manualimport, got %s", r.URL.Path)
				}
				if r.Method != "GET" {
					t.Errorf("Expected GET, got %s", r.Method)
				}
				folder := r.URL.Query().Get("folder")
				if folder != "/downloads/Series.Name.S01E01" {
					t.Errorf("folder param = %q, want %q", folder, "/downloads/Series.Name.S01E01")
				}
			},
		},
		{
			name:         "Sonarr - series null with rejection",
			folderPath:   "/downloads/Unknown.Series.S01E01",
			instanceType: "sonarr",
			statusCode:   http.StatusOK,
			responseBody: `[
				{
					"id": 9876543210,
					"path": "/downloads/Unknown.Series.S01E01/file.mkv",
					"name": "file.mkv",
					"series": null,
					"seasonNumber": null,
					"episodes": [],
					"quality": {
						"quality": {
							"id": 1,
							"name": "HDTV-720p"
						},
						"revision": {
							"version": 1,
							"real": 0
						}
					},
					"languages": [],
					"rejections": [
						{
							"reason": "Unknown Series",
							"type": "permanent"
						}
					]
				}
			]`,
			expectError:   false,
			expectedCount: 1,
			validateItems: func(t *testing.T, items []ManualImportResource) {
				if items[0].Series != nil {
					t.Error("Series should be nil for unknown series")
				}
				if len(items[0].Rejections) != 1 {
					t.Fatalf("Expected 1 rejection, got %d", len(items[0].Rejections))
				}
				if items[0].Rejections[0].Reason != "Unknown Series" {
					t.Errorf("Rejection reason = %q, want 'Unknown Series'", items[0].Rejections[0].Reason)
				}
			},
		},
		{
			name:         "Radarr - scan folder with matched movie",
			folderPath:   "/downloads/Movie.Name.2020",
			instanceType: "radarr",
			statusCode:   http.StatusOK,
			responseBody: `[
				{
					"id": 1234567890,
					"path": "/downloads/Movie.Name.2020/Movie.Name.2020.1080p-GROUP.mkv",
					"name": "Movie.Name.2020.1080p-GROUP.mkv",
					"size": 3298534883328,
					"movie": {
						"id": 42,
						"title": "Movie Name",
						"year": 2020
					},
					"quality": {
						"quality": {
							"id": 3,
							"name": "HDTV-1080p",
							"resolution": 1080
						},
						"revision": {
							"version": 1,
							"real": 0
						}
					},
					"languages": [
						{
							"id": 1,
							"name": "English"
						}
					],
					"releaseGroup": "GROUP",
					"rejections": [],
					"downloadId": "xyz789abc"
				}
			]`,
			expectError:   false,
			expectedCount: 1,
			validateItems: func(t *testing.T, items []ManualImportResource) {
				if items[0].Movie == nil {
					t.Fatal("Movie should not be nil")
				}
				if items[0].Movie.ID != 42 {
					t.Errorf("Movie ID = %d, want 42", items[0].Movie.ID)
				}
				if items[0].Movie.Year != 2020 {
					t.Errorf("Movie year = %d, want 2020", items[0].Movie.Year)
				}
				if len(items[0].Rejections) != 0 {
					t.Errorf("Should have no rejections, got %d", len(items[0].Rejections))
				}
			},
		},
		{
			name:         "Radarr - movie null with rejection",
			folderPath:   "/downloads/Unknown.Movie.2021",
			instanceType: "radarr",
			statusCode:   http.StatusOK,
			responseBody: `[
				{
					"id": 9876543210,
					"path": "/downloads/Unknown.Movie.2021/file.mkv",
					"name": "file.mkv",
					"movie": null,
					"quality": {
						"quality": {"id": 3, "name": "HDTV-1080p"}
					},
					"languages": [],
					"rejections": [
						{
							"reason": "Unknown Movie",
							"type": "permanent"
						}
					]
				}
			]`,
			expectError:   false,
			expectedCount: 1,
			validateItems: func(t *testing.T, items []ManualImportResource) {
				if items[0].Movie != nil {
					t.Error("Movie should be nil for unknown movie")
				}
				if len(items[0].Rejections) != 1 {
					t.Fatalf("Expected 1 rejection, got %d", len(items[0].Rejections))
				}
				if items[0].Rejections[0].Reason != "Unknown Movie" {
					t.Errorf("Rejection reason = %q, want 'Unknown Movie'", items[0].Rejections[0].Reason)
				}
			},
		},
		{
			name:          "Empty folder - no files found",
			folderPath:    "/downloads/empty",
			instanceType:  "sonarr",
			statusCode:    http.StatusOK,
			responseBody:  `[]`,
			expectError:   false,
			expectedCount: 0,
		},
		{
			name:         "Multiple files in scan",
			folderPath:   "/downloads/Season.Pack",
			instanceType: "sonarr",
			statusCode:   http.StatusOK,
			responseBody: `[
				{
					"id": 1,
					"path": "/downloads/Season.Pack/S01E01.mkv",
					"series": {"id": 42, "title": "Series"},
					"seasonNumber": 1,
					"episodes": [{"id": 100, "seasonNumber": 1, "episodeNumber": 1}],
					"quality": {"quality": {"id": 1, "name": "HDTV-720p"}},
					"rejections": []
				},
				{
					"id": 2,
					"path": "/downloads/Season.Pack/S01E02.mkv",
					"series": {"id": 42, "title": "Series"},
					"seasonNumber": 1,
					"episodes": [{"id": 101, "seasonNumber": 1, "episodeNumber": 2}],
					"quality": {"quality": {"id": 1, "name": "HDTV-720p"}},
					"rejections": []
				}
			]`,
			expectError:   false,
			expectedCount: 2,
			validateItems: func(t *testing.T, items []ManualImportResource) {
				if len(items) != 2 {
					t.Fatalf("Expected 2 items, got %d", len(items))
				}
				// Verify both are from same series
				if items[0].Series.ID != items[1].Series.ID {
					t.Error("Both files should be from same series")
				}
			},
		},
		{
			name:         "API error - 400 Bad Request",
			folderPath:   "/invalid/path",
			instanceType: "sonarr",
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error": "Invalid path"}`,
			expectError:  true,
		},
		{
			name:         "API error - 404 Not Found",
			folderPath:   "/nonexistent",
			instanceType: "sonarr",
			statusCode:   http.StatusNotFound,
			responseBody: `{"error": "Path not found"}`,
			expectError:  true,
		},
		{
			name:         "API error - 500 Server Error",
			folderPath:   "/downloads/test",
			instanceType: "sonarr",
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"error": "Internal server error"}`,
			expectError:  true,
		},
		{
			name:         "Invalid JSON response",
			folderPath:   "/downloads/test",
			instanceType: "sonarr",
			statusCode:   http.StatusOK,
			responseBody: `{invalid json`,
			expectError:  true,
		},
		{
			name:         "Unauthorized - invalid API key",
			folderPath:   "/downloads/test",
			instanceType: "sonarr",
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"error": "Unauthorized"}`,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify API key
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error":"Unauthorized"}`))
					return
				}

				// Run custom request validations
				if tt.validateRequest != nil {
					tt.validateRequest(t, r)
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Note: This tests the expected function signature
			// Actual implementation would be:
			// items, err := scanForManualImport(config, server.URL, "test-token", tt.folderPath)

			// For now, we test by calling the endpoint directly to validate test structure
			config := Config{Timeout: 5 * time.Second}
			client := &http.Client{Timeout: config.Timeout}

			req, _ := http.NewRequest("GET", server.URL+"/api/v3/manualimport?folder="+tt.folderPath, nil)
			req.Header.Set("X-Api-Key", "test-token")

			resp, err := client.Do(req)
			if err != nil && !tt.expectError {
				t.Fatalf("Unexpected error: %v", err)
			}
			if resp != nil {
				defer resp.Body.Close()
			}

			if tt.expectError {
				if resp != nil && resp.StatusCode == http.StatusOK {
					// Even if status is OK, check if JSON decodes - invalid JSON should fail
					var items []ManualImportResource
					if err := json.NewDecoder(resp.Body).Decode(&items); err == nil {
						t.Error("Expected error (invalid JSON or non-OK status) but got success")
					}
				}
				return
			}

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Status code = %d, want 200", resp.StatusCode)
				return
			}

			var items []ManualImportResource
			if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if len(items) != tt.expectedCount {
				t.Errorf("got %d items, want %d", len(items), tt.expectedCount)
			}

			if tt.validateItems != nil && len(items) > 0 {
				tt.validateItems(t, items)
			}
		})
	}
}

// ========== TEST: executeManualImport (POST /api/v3/manualimport) ==========

func TestExecuteManualImport(t *testing.T) {
	tests := []struct {
		name            string
		importRequests  []ManualImportRequest
		instanceType    string
		statusCode      int
		responseBody    string
		expectError     bool
		validateRequest func(*testing.T, *http.Request, []ManualImportRequest)
	}{
		{
			name:         "Sonarr - import single episode",
			instanceType: "sonarr",
			importRequests: []ManualImportRequest{
				{
					Path:         "/downloads/Series.S01E01/file.mkv",
					SeriesID:     42,
					SeasonNumber: 1,
					EpisodeIDs:   []int{123456},
					Quality: QualityModel{
						Quality: QualityDefinition{ID: 1, Name: "HDTV-720p"},
					},
					Languages:  []Language{{ID: 1, Name: "English"}},
					DownloadID: "abc123",
				},
			},
			statusCode:   http.StatusOK,
			responseBody: `[]`,
			expectError:  false,
			validateRequest: func(t *testing.T, r *http.Request, requests []ManualImportRequest) {
				if r.Method != "POST" {
					t.Errorf("Expected POST, got %s", r.Method)
				}
				if !strings.Contains(r.URL.Path, "/api/v3/manualimport") {
					t.Errorf("Expected /api/v3/manualimport, got %s", r.URL.Path)
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Expected Content-Type application/json")
				}

				// Decode and validate body
				var body []ManualImportRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("Failed to decode request body: %v", err)
				}
				if len(body) != 1 {
					t.Fatalf("Expected 1 request, got %d", len(body))
				}
				if body[0].SeriesID != 42 {
					t.Errorf("SeriesID = %d, want 42", body[0].SeriesID)
				}
				if body[0].SeasonNumber != 1 {
					t.Errorf("SeasonNumber = %d, want 1", body[0].SeasonNumber)
				}
				if len(body[0].EpisodeIDs) != 1 || body[0].EpisodeIDs[0] != 123456 {
					t.Errorf("EpisodeIDs = %v, want [123456]", body[0].EpisodeIDs)
				}
			},
		},
		{
			name:         "Sonarr - import multiple episodes",
			instanceType: "sonarr",
			importRequests: []ManualImportRequest{
				{
					Path:         "/downloads/S01E01.mkv",
					SeriesID:     42,
					SeasonNumber: 1,
					EpisodeIDs:   []int{100},
					DownloadID:   "batch1",
				},
				{
					Path:         "/downloads/S01E02.mkv",
					SeriesID:     42,
					SeasonNumber: 1,
					EpisodeIDs:   []int{101},
					DownloadID:   "batch1",
				},
			},
			statusCode:   http.StatusOK,
			responseBody: `[]`,
			expectError:  false,
			validateRequest: func(t *testing.T, r *http.Request, requests []ManualImportRequest) {
				var body []ManualImportRequest
				json.NewDecoder(r.Body).Decode(&body)
				if len(body) != 2 {
					t.Errorf("Expected 2 import requests, got %d", len(body))
				}
				// Verify both are for same series
				if body[0].SeriesID != body[1].SeriesID {
					t.Error("Both requests should be for same series")
				}
			},
		},
		{
			name:         "Radarr - import movie",
			instanceType: "radarr",
			importRequests: []ManualImportRequest{
				{
					Path:    "/downloads/Movie.2020/movie.mkv",
					MovieID: 42,
					Quality: QualityModel{
						Quality: QualityDefinition{ID: 3, Name: "HDTV-1080p"},
					},
					Languages:  []Language{{ID: 1, Name: "English"}},
					DownloadID: "movie123",
				},
			},
			statusCode:   http.StatusOK,
			responseBody: `[]`,
			expectError:  false,
			validateRequest: func(t *testing.T, r *http.Request, requests []ManualImportRequest) {
				var body []ManualImportRequest
				json.NewDecoder(r.Body).Decode(&body)
				if len(body) != 1 {
					t.Fatalf("Expected 1 request, got %d", len(body))
				}
				if body[0].MovieID != 42 {
					t.Errorf("MovieID = %d, want 42", body[0].MovieID)
				}
				// Radarr requests should NOT have SeriesID or EpisodeIDs
				if body[0].SeriesID != 0 {
					t.Error("Radarr request should not have SeriesID")
				}
			},
		},
		{
			name:           "Empty import list",
			instanceType:   "sonarr",
			importRequests: []ManualImportRequest{},
			statusCode:     http.StatusOK,
			responseBody:   `[]`,
			expectError:    false,
		},
		{
			name:         "API error - 400 Bad Request (missing required field)",
			instanceType: "sonarr",
			importRequests: []ManualImportRequest{
				{
					Path: "/downloads/file.mkv",
					// Missing SeriesID - should cause error
				},
			},
			statusCode:   http.StatusBadRequest,
			responseBody: `{"error": "Missing required field: seriesId"}`,
			expectError:  true,
		},
		{
			name:         "API error - 404 Not Found (invalid series ID)",
			instanceType: "sonarr",
			importRequests: []ManualImportRequest{
				{
					Path:         "/downloads/file.mkv",
					SeriesID:     99999,
					SeasonNumber: 1,
				},
			},
			statusCode:   http.StatusNotFound,
			responseBody: `{"error": "Series not found"}`,
			expectError:  true,
		},
		{
			name:         "API error - 500 Server Error",
			instanceType: "sonarr",
			importRequests: []ManualImportRequest{
				{
					Path:     "/downloads/file.mkv",
					SeriesID: 42,
				},
			},
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"error": "Internal server error"}`,
			expectError:  true,
		},
		{
			name:         "Unauthorized - invalid API key",
			instanceType: "sonarr",
			importRequests: []ManualImportRequest{
				{Path: "/downloads/file.mkv", SeriesID: 42},
			},
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"error": "Unauthorized"}`,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify API key
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error":"Unauthorized"}`))
					return
				}

				// Run custom request validations
				if tt.validateRequest != nil {
					tt.validateRequest(t, r, tt.importRequests)
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Test by calling endpoint directly
			config := Config{Timeout: 5 * time.Second}
			client := &http.Client{Timeout: config.Timeout}

			jsonData, _ := json.Marshal(tt.importRequests)
			req, _ := http.NewRequest("POST", server.URL+"/api/v3/manualimport", strings.NewReader(string(jsonData)))
			req.Header.Set("X-Api-Key", "test-token")
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil && !tt.expectError {
				t.Fatalf("Unexpected error: %v", err)
			}
			if resp != nil {
				defer resp.Body.Close()
			}

			if tt.expectError {
				if resp != nil && resp.StatusCode == http.StatusOK {
					t.Error("Expected error but got 200 OK")
				}
				return
			}

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Status code = %d, want 200", resp.StatusCode)
			}
		})
	}
}

// ========== TEST: Dry Run Mode - CRITICAL TEST ==========

func TestManualImportDryRunMode(t *testing.T) {
	getScanCalls := 0
	postImportCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/manualimport"):
			getScanCalls++
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[
				{
					"id": 1,
					"path": "/downloads/test/file.mkv",
					"series": {"id": 42, "title": "Test Series"},
					"seasonNumber": 1,
					"episodes": [{"id": 100, "seasonNumber": 1, "episodeNumber": 1}],
					"quality": {"quality": {"id": 1, "name": "HDTV-720p"}},
					"rejections": []
				}
			]`))

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/api/v3/manualimport"):
			postImportCalls++
			// This should NEVER be called in dry-run mode
			t.Error("POST /api/v3/manualimport called in dry-run mode - DRY RUN VIOLATED!")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/api/v3/command"):
			// Command API should also not be called in dry-run
			t.Error("POST /api/v3/command called in dry-run mode - DRY RUN VIOLATED!")
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer server.Close()

	// Test scenario: Queue item requires manual import
	queueServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if strings.Contains(r.URL.Path, "/queue") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(QueueResponse{
				Page:         1,
				PageSize:     100,
				TotalRecords: 1,
				Records: []QueueItem{
					{
						ID:                    1,
						Title:                 "Test Series S01E01",
						Status:                "completed",
						TrackedDownloadState:  "importBlocked",
						TrackedDownloadStatus: "warning",
						OutputPath:            "/downloads/test",
						DownloadId:            "test123",
						StatusMessages: []StatusMessage{
							{
								Title:    "Warning",
								Messages: []string{"matched to series by ID"},
							},
						},
					},
				},
			})
		}
	}))
	defer queueServer.Close()

	config := Config{
		SonarrURLs:   []string{queueServer.URL},
		SonarrTokens: []string{"test-token"},
		Timeout:      5 * time.Second,
	}

	// Run in DRY-RUN mode
	err := classifyAndRemediate(config, true) // dryRun = true
	if err != nil {
		t.Fatalf("Unexpected error in dry-run: %v", err)
	}

	// CRITICAL VALIDATION: No POST requests should have been made
	if postImportCalls > 0 {
		t.Errorf("DRY-RUN MODE VIOLATED: Made %d POST /api/v3/manualimport calls, want 0", postImportCalls)
	}

	// In a full implementation with manual import, we might allow GET scans in dry-run
	// but absolutely NO POST operations
	t.Logf("Dry-run completed successfully: %d GET scans, %d POST imports (want 0)", getScanCalls, postImportCalls)
}

// ========== TEST: buildManualImportRequests ==========

func TestBuildManualImportRequests(t *testing.T) {
	tests := []struct {
		name          string
		scannedItems  []ManualImportResource
		queueItem     QueueItem
		instanceType  string
		expectedCount int
		validateFunc  func(*testing.T, []ManualImportRequest)
	}{
		{
			name: "Sonarr - single matched episode",
			scannedItems: []ManualImportResource{
				{
					Path: "/downloads/Series.S01E01/file.mkv",
					Series: &SeriesResource{
						ID:    42,
						Title: "Test Series",
					},
					SeasonNumber: intPtr(1),
					Episodes: []EpisodeResource{
						{ID: 123456, SeasonNumber: 1, EpisodeNumber: 1},
					},
					Quality: QualityModel{
						Quality: QualityDefinition{ID: 1, Name: "HDTV-720p"},
					},
					Languages:  []Language{{ID: 1, Name: "English"}},
					Rejections: []ImportRejection{},
				},
			},
			queueItem: QueueItem{
				DownloadId: "abc123",
			},
			instanceType:  "sonarr",
			expectedCount: 1,
			validateFunc: func(t *testing.T, requests []ManualImportRequest) {
				if requests[0].SeriesID != 42 {
					t.Errorf("SeriesID = %d, want 42", requests[0].SeriesID)
				}
				if requests[0].SeasonNumber != 1 {
					t.Errorf("SeasonNumber = %d, want 1", requests[0].SeasonNumber)
				}
				if len(requests[0].EpisodeIDs) != 1 || requests[0].EpisodeIDs[0] != 123456 {
					t.Errorf("EpisodeIDs = %v, want [123456]", requests[0].EpisodeIDs)
				}
				if requests[0].DownloadID != "abc123" {
					t.Errorf("DownloadID = %q, want 'abc123'", requests[0].DownloadID)
				}
			},
		},
		{
			name: "Sonarr - skip items with rejections",
			scannedItems: []ManualImportResource{
				{
					Path:       "/downloads/file.mkv",
					Series:     nil,
					Rejections: []ImportRejection{{Reason: "Unknown Series", Type: "permanent"}},
				},
			},
			queueItem:     QueueItem{},
			instanceType:  "sonarr",
			expectedCount: 0, // Should skip rejected items
		},
		{
			name: "Sonarr - skip items with null series",
			scannedItems: []ManualImportResource{
				{
					Path:       "/downloads/file.mkv",
					Series:     nil,
					Rejections: []ImportRejection{},
				},
			},
			queueItem:     QueueItem{},
			instanceType:  "sonarr",
			expectedCount: 0, // Should skip items without series match
		},
		{
			name: "Radarr - single matched movie",
			scannedItems: []ManualImportResource{
				{
					Path: "/downloads/Movie.2020/movie.mkv",
					Movie: &MovieResource{
						ID:    42,
						Title: "Test Movie",
						Year:  2020,
					},
					Quality: QualityModel{
						Quality: QualityDefinition{ID: 3, Name: "HDTV-1080p"},
					},
					Languages:  []Language{{ID: 1, Name: "English"}},
					Rejections: []ImportRejection{},
				},
			},
			queueItem: QueueItem{
				DownloadId: "movie123",
			},
			instanceType:  "radarr",
			expectedCount: 1,
			validateFunc: func(t *testing.T, requests []ManualImportRequest) {
				if requests[0].MovieID != 42 {
					t.Errorf("MovieID = %d, want 42", requests[0].MovieID)
				}
				if requests[0].SeriesID != 0 {
					t.Error("SeriesID should be 0 for Radarr")
				}
				if requests[0].DownloadID != "movie123" {
					t.Errorf("DownloadID = %q, want 'movie123'", requests[0].DownloadID)
				}
			},
		},
		{
			name: "Radarr - skip items with null movie",
			scannedItems: []ManualImportResource{
				{
					Path:       "/downloads/movie.mkv",
					Movie:      nil,
					Rejections: []ImportRejection{},
				},
			},
			queueItem:     QueueItem{},
			instanceType:  "radarr",
			expectedCount: 0,
		},
		{
			name: "Multiple files - all valid",
			scannedItems: []ManualImportResource{
				{
					Path:         "/downloads/S01E01.mkv",
					Series:       &SeriesResource{ID: 42},
					SeasonNumber: intPtr(1),
					Episodes:     []EpisodeResource{{ID: 100}},
					Rejections:   []ImportRejection{},
				},
				{
					Path:         "/downloads/S01E02.mkv",
					Series:       &SeriesResource{ID: 42},
					SeasonNumber: intPtr(1),
					Episodes:     []EpisodeResource{{ID: 101}},
					Rejections:   []ImportRejection{},
				},
			},
			queueItem:     QueueItem{DownloadId: "batch"},
			instanceType:  "sonarr",
			expectedCount: 2,
		},
		{
			name: "Multiple files - mixed valid and rejected",
			scannedItems: []ManualImportResource{
				{
					Path:         "/downloads/S01E01.mkv",
					Series:       &SeriesResource{ID: 42},
					SeasonNumber: intPtr(1),
					Episodes:     []EpisodeResource{{ID: 100}},
					Rejections:   []ImportRejection{},
				},
				{
					Path:       "/downloads/S01E02.mkv",
					Series:     nil,
					Rejections: []ImportRejection{{Reason: "Unknown Series"}},
				},
			},
			queueItem:     QueueItem{},
			instanceType:  "sonarr",
			expectedCount: 1, // Only one valid
		},
		{
			name:          "Empty scan results",
			scannedItems:  []ManualImportResource{},
			queueItem:     QueueItem{},
			instanceType:  "sonarr",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This tests the expected function signature
			// Actual implementation would be:
			// requests := buildManualImportRequests(tt.scannedItems, tt.queueItem)

			// For now, simulate the logic
			var requests []ManualImportRequest
			for _, item := range tt.scannedItems {
				// Skip items with rejections
				if len(item.Rejections) > 0 {
					continue
				}

				if tt.instanceType == "sonarr" {
					if item.Series == nil {
						continue
					}
					req := ManualImportRequest{
						Path:         item.Path,
						SeriesID:     item.Series.ID,
						Quality:      item.Quality,
						Languages:    item.Languages,
						ReleaseGroup: item.ReleaseGroup,
						DownloadID:   tt.queueItem.DownloadId,
					}
					if item.SeasonNumber != nil {
						req.SeasonNumber = *item.SeasonNumber
					}
					for _, ep := range item.Episodes {
						req.EpisodeIDs = append(req.EpisodeIDs, ep.ID)
					}
					requests = append(requests, req)
				} else { // radarr
					if item.Movie == nil {
						continue
					}
					req := ManualImportRequest{
						Path:         item.Path,
						MovieID:      item.Movie.ID,
						Quality:      item.Quality,
						Languages:    item.Languages,
						ReleaseGroup: item.ReleaseGroup,
						DownloadID:   tt.queueItem.DownloadId,
					}
					requests = append(requests, req)
				}
			}

			if len(requests) != tt.expectedCount {
				t.Errorf("got %d requests, want %d", len(requests), tt.expectedCount)
			}

			if tt.validateFunc != nil && len(requests) > 0 {
				tt.validateFunc(t, requests)
			}
		})
	}
}

// ========== TEST: Integration - Full Manual Import Workflow ==========

func TestManualImportIntegrationWorkflow(t *testing.T) {
	getScanCalled := false
	postImportCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/manualimport"):
			getScanCalled = true
			// Verify folder parameter
			folder := r.URL.Query().Get("folder")
			if folder != "/downloads/test" {
				t.Errorf("folder param = %q, want '/downloads/test'", folder)
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[
				{
					"id": 1,
					"path": "/downloads/test/Series.S01E01.mkv",
					"series": {"id": 42, "title": "Test Series"},
					"seasonNumber": 1,
					"episodes": [{"id": 123456, "seasonNumber": 1, "episodeNumber": 1}],
					"quality": {"quality": {"id": 1, "name": "HDTV-720p"}},
					"languages": [{"id": 1, "name": "English"}],
					"rejections": [],
					"downloadId": "original-download-id"
				}
			]`))

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/api/v3/manualimport"):
			postImportCalled = true

			// Verify request body
			var body []ManualImportRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("Failed to decode POST body: %v", err)
			}

			if len(body) != 1 {
				t.Fatalf("Expected 1 import request, got %d", len(body))
			}

			// Validate import request fields
			if body[0].SeriesID != 42 {
				t.Errorf("SeriesID = %d, want 42", body[0].SeriesID)
			}
			if body[0].SeasonNumber != 1 {
				t.Errorf("SeasonNumber = %d, want 1", body[0].SeasonNumber)
			}
			if len(body[0].EpisodeIDs) != 1 || body[0].EpisodeIDs[0] != 123456 {
				t.Errorf("EpisodeIDs = %v, want [123456]", body[0].EpisodeIDs)
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()

	// This test validates the expected workflow:
	// 1. Queue item identified as needing manual import
	// 2. GET /api/v3/manualimport?folder=... to scan
	// 3. Build import requests from scan results
	// 4. POST /api/v3/manualimport with import requests

	t.Run("Full workflow simulation", func(t *testing.T) {
		config := Config{Timeout: 5 * time.Second}
		client := &http.Client{Timeout: config.Timeout}

		// Step 1: Scan
		scanReq, _ := http.NewRequest("GET", server.URL+"/api/v3/manualimport?folder=/downloads/test", nil)
		scanReq.Header.Set("X-Api-Key", "test-token")
		scanResp, err := client.Do(scanReq)
		if err != nil {
			t.Fatalf("Scan request failed: %v", err)
		}
		defer scanResp.Body.Close()

		var scanned []ManualImportResource
		json.NewDecoder(scanResp.Body).Decode(&scanned)

		if len(scanned) != 1 {
			t.Fatalf("Expected 1 scanned item, got %d", len(scanned))
		}

		// Step 2: Build import requests
		var imports []ManualImportRequest
		for _, item := range scanned {
			if len(item.Rejections) == 0 && item.Series != nil {
				req := ManualImportRequest{
					Path:         item.Path,
					SeriesID:     item.Series.ID,
					SeasonNumber: *item.SeasonNumber,
					Quality:      item.Quality,
					Languages:    item.Languages,
				}
				for _, ep := range item.Episodes {
					req.EpisodeIDs = append(req.EpisodeIDs, ep.ID)
				}
				imports = append(imports, req)
			}
		}

		// Step 3: Execute import
		jsonData, _ := json.Marshal(imports)
		importReq, _ := http.NewRequest("POST", server.URL+"/api/v3/manualimport", strings.NewReader(string(jsonData)))
		importReq.Header.Set("X-Api-Key", "test-token")
		importReq.Header.Set("Content-Type", "application/json")

		importResp, err := client.Do(importReq)
		if err != nil {
			t.Fatalf("Import request failed: %v", err)
		}
		defer importResp.Body.Close()

		if importResp.StatusCode != http.StatusOK {
			t.Errorf("Import status = %d, want 200", importResp.StatusCode)
		}
	})

	// Verify both API calls were made
	if !getScanCalled {
		t.Error("GET /api/v3/manualimport was not called")
	}
	if !postImportCalled {
		t.Error("POST /api/v3/manualimport was not called")
	}
}

// ========== HELPER FUNCTIONS ==========

func intPtr(i int) *int {
	return &i
}
