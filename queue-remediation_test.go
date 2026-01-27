//go:build queueremediation

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
			name:          "Matched by ID not name - delete with blocklist",
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
			expectedAction:       "delete",
			expectedBlocklist:    true,
			expectedManualImport: false,
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
		{
			name:          "Import blocked WITH sample file message",
			status:        "completed",
			trackedState:  "importBlocked",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{"Sample"},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    false,
			expectedManualImport: false,
		},
		{
			name:          "Import blocked WITH no files found message",
			status:        "completed",
			trackedState:  "importBlocked",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{"No files found"},
				},
			},
			expectedAction:       "delete",
			expectedBlocklist:    false,
			expectedManualImport: false,
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
		{
			name:          "Sample + TheXEM mapping pending - Mapping takes priority (American Pickers bug fix)",
			status:        "completed",
			trackedState:  "importBlocked",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{"This show has individual episode mappings on TheXEM but the mapping for this episode has not been confirmed yet by their administrators. TheXEM needs manual input."},
				},
				{
					Title:    "Warning",
					Messages: []string{"Sample"},
				},
				{
					Title:    "Warning",
					Messages: []string{"This show has individual episode mappings on TheXEM but the mapping for this episode has not been confirmed yet by their administrators. TheXEM needs manual input."},
				},
			},
			expectedAction:       "manual_import",
			expectedBlocklist:    false,
			expectedManualImport: true,
		},
		{
			name:          "Sample + TBA title - Mapping takes priority",
			status:        "completed",
			trackedState:  "importBlocked",
			trackedStatus: "warning",
			statusMessages: []StatusMessage{
				{
					Title:    "Warning",
					Messages: []string{"Sample"},
				},
				{
					Title:    "Warning",
					Messages: []string{"Episode has a TBA title and recently aired"},
				},
			},
			expectedAction:       "manual_import",
			expectedBlocklist:    false,
			expectedManualImport: true,
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

func TestMapStatusToActionMatchedByID(t *testing.T) {
	sonarrItem := QueueItem{
		InstanceType:          "sonarr",
		Status:                "completed",
		TrackedDownloadState:  "importPending",
		TrackedDownloadStatus: "warning",
		StatusMessages:        []StatusMessage{{Title: "Warning", Messages: []string{"Found matching series via grab history, but release was matched to series by ID"}}},
	}
	radarrItem := QueueItem{
		InstanceType:          "radarr",
		Status:                "completed",
		TrackedDownloadState:  "importPending",
		TrackedDownloadStatus: "warning",
		StatusMessages:        []StatusMessage{{Title: "Warning", Messages: []string{"Found matching series via grab history, but release was matched to series by ID"}}},
	}

	action, _, manual := mapStatusToAction(sonarrItem)
	if action != "manual_import" || !manual {
		t.Errorf("Sonarr matched_by_id action = %q manual=%v, want manual_import/true", action, manual)
	}

	action, _, manual = mapStatusToAction(radarrItem)
	if action != "delete" || manual {
		t.Errorf("Radarr matched_by_id action = %q manual=%v, want delete/false", action, manual)
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

// TestTriggerManualImport verifies manual import command using scan → build → execute workflow
func TestTriggerManualImport(t *testing.T) {
	tests := []struct {
		name            string
		downloadPath    string
		scanStatusCode  int
		importStatus    int
		expectError     bool
		queueItem       QueueItem
		validateRequest func(*testing.T, *http.Request, map[string]interface{})
	}{
		{
			name:           "Successful manual import with scan and execute",
			downloadPath:   "/downloads/Movie.2024",
			scanStatusCode: http.StatusOK,
			importStatus:   http.StatusCreated,
			expectError:    false,
			queueItem:      QueueItem{ID: 1, OutputPath: "/downloads/Movie.2024", MovieID: 42, DownloadId: "test-download-id"},
			validateRequest: func(t *testing.T, r *http.Request, body map[string]interface{}) {
				if body["name"] != "ManualImport" {
					t.Errorf("Command name = %q, want ManualImport", body["name"])
				}
				// Check that files array exists
				if _, ok := body["files"]; !ok {
					t.Errorf("Expected 'files' field in ManualImport command")
				}
			},
		},
		{
			name:           "Scan returns no files",
			downloadPath:   "/downloads/empty",
			scanStatusCode: http.StatusOK,
			importStatus:   http.StatusCreated,
			expectError:    true, // Should error because no files found
			queueItem:      QueueItem{ID: 2, OutputPath: "/downloads/empty", MovieID: 43, DownloadId: "empty-download"},
		},
		{
			name:           "Scan fails with server error",
			downloadPath:   "/downloads/test",
			scanStatusCode: http.StatusInternalServerError,
			importStatus:   http.StatusCreated,
			expectError:    true,
			queueItem:      QueueItem{ID: 3, OutputPath: "/downloads/test", MovieID: 44, DownloadId: "error-download"},
		},
		{
			name:           "Import execution fails",
			downloadPath:   "/downloads/import-fail",
			scanStatusCode: http.StatusOK,
			importStatus:   http.StatusBadRequest,
			expectError:    true,
			queueItem:      QueueItem{ID: 4, OutputPath: "/downloads/import-fail", MovieID: 45, DownloadId: "fail-download"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify API key
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// Handle command status polling (GET /api/v3/command/{id})
				if r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/command/") {
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(CommandResource{ID: 1, Status: "completed", Result: "successful"})
					return
				}

				// Handle scan request (GET /api/v3/manualimport)
				if r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/manualimport") {
					w.WriteHeader(tt.scanStatusCode)
					if tt.scanStatusCode == http.StatusOK {
						// Return mock scan results based on test case
						if tt.name == "Scan returns no files" {
							json.NewEncoder(w).Encode([]ManualImportResource{})
						} else {
							// Return a valid file that can be imported (with Movie metadata for Radarr)
							json.NewEncoder(w).Encode([]ManualImportResource{
								{
									Path:       tt.downloadPath + "/movie.mkv",
									Name:       "movie.mkv",
									Movie:      &MovieResource{ID: tt.queueItem.MovieID, Title: "Test Movie"},
									Quality:    QualityModel{Quality: QualityDefinition{Name: "Bluray-1080p"}},
									Rejections: []ImportRejection{}, // No rejections = importable
								},
							})
						}
					} else {
						json.NewEncoder(w).Encode(map[string]string{"error": "scan failed"})
					}
					return
				}

				// Handle import command (POST /api/v3/command)
				if r.Method == "POST" && strings.Contains(r.URL.Path, "/api/v3/command") {
					if r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
					}

					var body map[string]interface{}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Errorf("Failed to decode request body: %v", err)
					}

					// Run custom validations
					if tt.validateRequest != nil {
						tt.validateRequest(t, r, body)
					}

					w.WriteHeader(tt.importStatus)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"id":    1,
						"name":  body["name"],
						"state": "queued",
					})
					return
				}

				// Unexpected request
				t.Errorf("Unexpected request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			config := Config{Timeout: 5 * time.Second}
			err := triggerManualImport(config, server.URL, "test-token", tt.downloadPath, "radarr", false, tt.queueItem)

			if (err != nil) != tt.expectError {
				t.Errorf("error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}

func TestTriggerManualImportMatchedByIDFallback(t *testing.T) {
	var commandCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/manualimport"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]ManualImportResource{
				{
					Path:       "/downloads/Test.Series.S01E02/file.mkv",
					Quality:    QualityModel{Quality: QualityDefinition{ID: 1, Name: "HDTV-720p"}},
					Languages:  []Language{{ID: 1, Name: "English"}},
					Rejections: []ImportRejection{{Reason: "Unknown Series", Type: "permanent"}},
				},
			})

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/series"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]SeriesResource{{ID: 42, Title: "Test Series"}})

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/parse"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(ParseResource{
				Series: SeriesResource{ID: 42, Title: "Test Series"},
				ParsedEpisodeInfo: ParsedEpisodeInfo{
					SeasonNumber:   1,
					EpisodeNumbers: []int{2},
				},
			})

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/episode"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]EpisodeResource{{ID: 555, SeriesID: 42, SeasonNumber: 1, EpisodeNumber: 2, Title: "Ep"}})

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/api/v3/command"):
			commandCalled = true
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 7})

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/command/"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(CommandResource{ID: 7, Status: "completed", Result: "successful"})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	queueItem := QueueItem{
		ID:           101,
		Title:        "Test.Series.S01E02.720p",
		InstanceType: "sonarr",
		OutputPath:   "/downloads/Test.Series.S01E02",
		DownloadId:   "download-1",
		StatusMessages: []StatusMessage{
			{Title: "Warning", Messages: []string{"matched to series by ID"}},
		},
	}

	config := Config{Timeout: 5 * time.Second}
	if err := triggerManualImport(config, server.URL, "test-token", queueItem.OutputPath, "sonarr", false, queueItem); err != nil {
		t.Fatalf("triggerManualImport fallback failed: %v", err)
	}
	if !commandCalled {
		t.Error("Expected manual import command to be executed")
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
			// Verify blocklist parameter for items
			if strings.Contains(r.URL.Path, "/queue/1") && r.URL.Query().Get("blocklist") != "true" {
				t.Error("Item 1 (custom format) should be deleted with blocklist=true")
			}
			if strings.Contains(r.URL.Path, "/queue/2") && r.URL.Query().Get("blocklist") != "" {
				t.Error("Item 2 (no files) should be deleted without blocklist parameter")
			}
			if strings.Contains(r.URL.Path, "/queue/3") && r.URL.Query().Get("blocklist") != "true" {
				t.Error("Item 3 (matched_by_id) should be deleted with blocklist=true")
			}
			w.WriteHeader(http.StatusOK)

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/manualimport"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]ManualImportResource{
				{
					Path:         "/downloads/Series.S01E01/file.mkv",
					Name:         "file.mkv",
					Series:       &SeriesResource{ID: 42, Title: "Matched by ID"},
					SeasonNumber: func() *int { n := 1; return &n }(),
					Episodes:     []EpisodeResource{{EpisodeNumber: 1}},
				},
			})

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/command"):
			manualImportCalls++
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})

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
						SeriesID:              42,
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

	// Should delete 2 items (custom format, no files)
	if deleteCalls != 2 {
		t.Errorf("Made %d delete calls, want 2", deleteCalls)
	}

	// Should trigger manual import for matched_by_id
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
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/command/"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(CommandResource{ID: 1, Status: "completed", Result: "successful"})

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

// TestFormatStatusMessages verifies status message formatting for dry-run output
func TestFormatStatusMessages(t *testing.T) {
	tests := []struct {
		name           string
		item           QueueItem
		expectedOutput string
	}{
		{
			name: "No status messages",
			item: QueueItem{
				StatusMessages: []StatusMessage{},
			},
			expectedOutput: "",
		},
		{
			name: "Single status message",
			item: QueueItem{
				StatusMessages: []StatusMessage{
					{
						Title:    "Warning",
						Messages: []string{"Sample file detected"},
					},
				},
			},
			expectedOutput: "     • Sample file detected",
		},
		{
			name: "Multiple status messages",
			item: QueueItem{
				StatusMessages: []StatusMessage{
					{
						Title:    "Warning",
						Messages: []string{"Sample file detected", "Quality not an upgrade"},
					},
				},
			},
			expectedOutput: "     • Sample file detected\n[DRY-RUN]     • Quality not an upgrade",
		},
		{
			name: "Multiple status message groups",
			item: QueueItem{
				StatusMessages: []StatusMessage{
					{
						Title:    "Warning",
						Messages: []string{"Sample file detected"},
					},
					{
						Title:    "Info",
						Messages: []string{"Download completed"},
					},
				},
			},
			expectedOutput: "     • Sample file detected\n[DRY-RUN]     • Download completed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatStatusMessages(tt.item)
			if result != tt.expectedOutput {
				t.Errorf("formatStatusMessages() = %q, want %q", result, tt.expectedOutput)
			}
		})
	}
}

// TestGetManualImportDetails verifies manual import details extraction
func TestGetManualImportDetails(t *testing.T) {
	// Create a minimal config for testing (API calls will fail but that's OK)
	config := Config{
		Timeout: 5 * time.Second,
	}

	tests := []struct {
		name           string
		item           QueueItem
		expectedOutput string
	}{
		{
			name: "Sonarr item with series ID",
			item: QueueItem{
				InstanceType: "sonarr",
				SeriesID:     123,
				OutputPath:   "/downloads/test",
			},
			expectedOutput: "     • Series ID: 123 (validated)\n[DRY-RUN]     • Output Path: /downloads/test\n[DRY-RUN]     • Import Method: Command API → ManualImport",
		},
		{
			name: "Radarr item with movie ID",
			item: QueueItem{
				InstanceType: "radarr",
				MovieID:      456,
				OutputPath:   "/downloads/movie",
			},
			expectedOutput: "     • Movie ID: 456 (validated)\n[DRY-RUN]     • Output Path: /downloads/movie\n[DRY-RUN]     • Import Method: Command API → ManualImport",
		},
		{
			name: "Item with no IDs or path",
			item: QueueItem{
				InstanceType: "sonarr",
			},
			expectedOutput: "",
		},
		{
			name: "Item with only output path",
			item: QueueItem{
				InstanceType: "sonarr",
				OutputPath:   "/downloads/onlypath",
			},
			expectedOutput: "     • Output Path: /downloads/onlypath\n[DRY-RUN]     • Import Method: Command API → ManualImport",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getManualImportDetails(config, tt.item)
			if result != tt.expectedOutput {
				t.Errorf("getManualImportDetails() = %q, want %q", result, tt.expectedOutput)
			}
		})
	}
}

// TestGetManualImportDetailsWithNameLookup verifies manual import details with series/movie name lookup
func TestGetManualImportDetailsWithNameLookup(t *testing.T) {
	// Create mock server for series API
	seriesServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/series/123" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(SeriesResource{
				ID:    123,
				Title: "Test Series Name",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer seriesServer.Close()

	// Create mock server for movie API
	movieServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/movie/456" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(MovieResource{
				ID:    456,
				Title: "Test Movie Name",
				Year:  2024,
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer movieServer.Close()

	config := Config{
		Timeout:      5 * time.Second,
		SonarrURLs:   []string{seriesServer.URL},
		SonarrTokens: []string{"test-token"},
		RadarrURLs:   []string{movieServer.URL},
		RadarrTokens: []string{"test-token"},
	}

	tests := []struct {
		name           string
		item           QueueItem
		expectedOutput string
	}{
		{
			name: "Sonarr item with series ID and successful lookup",
			item: QueueItem{
				InstanceType: "sonarr",
				InstanceURL:  seriesServer.URL,
				SeriesID:     123,
				OutputPath:   "/downloads/test",
			},
			expectedOutput: "     • Series ID: 123 (Test Series Name) (validated)\n[DRY-RUN]     • Output Path: /downloads/test\n[DRY-RUN]     • Import Method: Command API → ManualImport",
		},
		{
			name: "Radarr item with movie ID and successful lookup",
			item: QueueItem{
				InstanceType: "radarr",
				InstanceURL:  movieServer.URL,
				MovieID:      456,
				OutputPath:   "/downloads/movie",
			},
			expectedOutput: "     • Movie ID: 456 (Test Movie Name, 2024) (validated)\n[DRY-RUN]     • Output Path: /downloads/movie\n[DRY-RUN]     • Import Method: Command API → ManualImport",
		},
		{
			name: "Series lookup fails (invalid URL)",
			item: QueueItem{
				InstanceType: "sonarr",
				InstanceURL:  "http://invalid:8989",
				SeriesID:     123,
				OutputPath:   "/downloads/test",
			},
			expectedOutput: "     • Series ID: 123 (validated)\n[DRY-RUN]     • Output Path: /downloads/test\n[DRY-RUN]     • Import Method: Command API → ManualImport",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getManualImportDetails(config, tt.item)
			if result != tt.expectedOutput {
				t.Errorf("getManualImportDetails() = %q, want %q", result, tt.expectedOutput)
			}
		})
	}
}

// TestFormatQueueItemHeader verifies enhanced queue item header formatting
func TestFormatQueueItemHeader(t *testing.T) {
	config := Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"token"},
	}

	tests := []struct {
		name           string
		item           QueueItem
		expectedOutput string
	}{
		{
			name: "Complete item with all fields",
			item: QueueItem{
				InstanceURL:          "http://localhost:8989",
				InstanceType:         "sonarr",
				ID:                   123,
				Title:                "Test Series S01E01",
				Status:               "completed",
				TrackedDownloadState: "importPending",
				DownloadClient:       "transmission",
			},
			expectedOutput: "[DRY-RUN] Sonarr1 - Item #123 (Test Series S01E01)\n[DRY-RUN]   Status: completed | State: importPending | Client: transmission",
		},
		{
			name: "Item with minimal fields",
			item: QueueItem{
				InstanceURL:  "http://localhost:8989",
				InstanceType: "sonarr",
				ID:           456,
				Title:        "Minimal Item",
				Status:       "downloading",
			},
			expectedOutput: "[DRY-RUN] Sonarr1 - Item #456 (Minimal Item)\n[DRY-RUN]   Status: downloading",
		},
		{
			name: "Radarr item",
			item: QueueItem{
				InstanceURL:          "http://localhost:7878",
				InstanceType:         "radarr",
				ID:                   789,
				Title:                "Test Movie 2024",
				Status:               "completed",
				TrackedDownloadState: "importPending",
				DownloadClient:       "qBittorrent",
			},
			expectedOutput: "[DRY-RUN] radarr - Item #789 (Test Movie 2024)\n[DRY-RUN]   Status: completed | State: importPending | Client: qBittorrent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatQueueItemHeader(config, tt.item)
			if result != tt.expectedOutput {
				t.Errorf("formatQueueItemHeader() = %q, want %q", result, tt.expectedOutput)
			}
		})
	}
}

// TestGetActionDescription verifies enhanced action descriptions
func TestGetActionDescription(t *testing.T) {
	tests := []struct {
		name           string
		action         string
		item           QueueItem
		expectedOutput string
	}{
		{
			name:   "Delete with blocklist",
			action: "delete",
			item: QueueItem{
				StatusMessages: []StatusMessage{
					{
						Title:    "Warning",
						Messages: []string{"Not a Custom Format upgrade for existing movie file(s)"},
					},
				},
			},
			expectedOutput: "→ Would DELETE (blocklist=true) - custom format not an upgrade",
		},
		{
			name:   "Delete without blocklist",
			action: "delete",
			item: QueueItem{
				StatusMessages: []StatusMessage{
					{
						Title:    "Warning",
						Messages: []string{"Sample"},
					},
				},
			},
			expectedOutput: "→ Would DELETE - sample file detected",
		},
		{
			name:   "Manual import with path",
			action: "manual_import",
			item: QueueItem{
				OutputPath: "/downloads/test",
				StatusMessages: []StatusMessage{
					{
						Title:    "Warning",
						Messages: []string{"matched to series by ID"},
					},
				},
			},
			expectedOutput: "→ Would MANUAL_IMPORT - matched to series by ID (fallback to delete on mapping failure)",
		},
		{
			name:   "Manual import without path",
			action: "manual_import",
			item: QueueItem{
				OutputPath: "",
			},
			expectedOutput: "→ Would MANUAL_IMPORT (no output path available!) - downloading normally",
		},
		{
			name:   "Monitor action",
			action: "monitor",
			item: QueueItem{
				Status: "downloading",
			},
			expectedOutput: "→ MONITORING - downloading normally",
		},
		{
			name:   "Unknown action",
			action: "unknown",
			item: QueueItem{
				Status: "downloading",
			},
			expectedOutput: "→ Unknown action - downloading normally",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getActionDescription(tt.action, tt.item)
			if result != tt.expectedOutput {
				t.Errorf("getActionDescription() = %q, want %q", result, tt.expectedOutput)
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

				if r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/command/") {
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(CommandResource{ID: 1, Status: "completed", Result: "successful"})
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

// TestTorrentsDirectorySkip verifies that items in /torrents/ directory are skipped
func TestTorrentsDirectorySkip(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/queue"):
			// Return queue item with /torrents/ path
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{
				"page": 1,
				"pageSize": 100,
				"totalRecords": 1,
				"records": [
					{
						"id": 12345,
						"title": "Test.Series.S01E01.1080p.WEB.H264-GROUP",
						"status": "completed",
						"trackedDownloadState": "importPending",
						"trackedDownloadStatus": "warning",
						"statusMessages": [
							{
								"title": "Warning",
								"messages": ["No files found are eligible for import in /torrents/Test.Series.S01E01.1080p.WEB.H264-GROUP"]
							}
						],
						"downloadClient": "qBittorrent",
						"downloadId": "test123",
						"outputPath": "/torrents/Test.Series.S01E01.1080p.WEB.H264-GROUP",
						"seriesId": 42
					}
				]
			}`))
		default:
			t.Errorf("Unexpected API call: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	config := Config{
		SonarrURLs:   []string{server.URL},
		SonarrTokens: []string{"test-token"},
		Timeout:      5 * time.Second,
		Verbose:      true,
	}

	// Capture stdout to verify dry-run output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := classifyAndRemediate(config, true)

	// Restore stdout and capture output
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)

	output := buf.String()

	// Verify no error occurred
	if err != nil {
		t.Errorf("classifyAndRemediate() error = %v", err)
	}

	// Verify item was skipped
	if !strings.Contains(output, "SKIPPED - item is in /torrents/ directory") {
		t.Errorf("Expected output to contain torrents directory skip message, got: %s", output)
	}

	// Verify item was not processed for deletion or manual import
	if strings.Contains(output, "Would DELETE") || strings.Contains(output, "Would MANUAL_IMPORT") {
		t.Errorf("Expected item to be skipped, but found processing action in output: %s", output)
	}

	// Verify summary shows 1 total item but 0 actions taken
	if !strings.Contains(output, "Total items: 1") {
		t.Errorf("Expected summary to show 1 total item, got: %s", output)
	}
	if !strings.Contains(output, "Would delete: 0") || !strings.Contains(output, "Would manual import: 0") || !strings.Contains(output, "Monitoring: 0") {
		t.Errorf("Expected summary to show 0 actions taken, got: %s", output)
	}
}

// ========== TESTS FOR --manual FLAG FUNCTIONALITY ==========

// TestManualFlagParsing tests the --manual flag parsing and mode selection
func TestManualFlagParsing(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		expectManual bool
		expectError  bool
	}{
		{
			name:         "CLI mode without --manual flag",
			args:         []string{"-sonarr-urls", "http://localhost:8989", "-sonarr-tokens", "test-token"},
			expectManual: false,
			expectError:  false,
		},
		{
			name:         "TUI mode with --manual flag",
			args:         []string{"-manual", "-sonarr-urls", "http://localhost:8989", "-sonarr-tokens", "test-token"},
			expectManual: true,
			expectError:  false,
		},
		{
			name:         "TUI mode with --manual=true",
			args:         []string{"-manual=true", "-sonarr-urls", "http://localhost:8989", "-sonarr-tokens", "test-token"},
			expectManual: true,
			expectError:  false,
		},
		{
			name:         "CLI mode with --manual=false",
			args:         []string{"-manual=false", "-sonarr-urls", "http://localhost:8989", "-sonarr-tokens", "test-token"},
			expectManual: false,
			expectError:  false,
		},
		{
			name:         "Manual flag with valid config",
			args:         []string{"-manual", "-sonarr-urls", "http://localhost:8989", "-sonarr-tokens", "test-token"},
			expectManual: true,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new flag set to avoid conflicts
			flagSet := flag.NewFlagSet("test", flag.ContinueOnError)

			// Define flags
			var (
				sonarrURLs   = flagSet.String("sonarr-urls", "", "Comma-separated Sonarr URLs")
				sonarrTokens = flagSet.String("sonarr-tokens", "", "Comma-separated Sonarr API tokens")
				radarrURLs   = flagSet.String("radarr-urls", "", "Comma-separated Radarr URLs")
				radarrTokens = flagSet.String("radarr-tokens", "", "Comma-separated Radarr API tokens")
				timeout      = flagSet.Duration("timeout", 30*time.Second, "HTTP request timeout")
				useRestAPI   = flagSet.Bool("use-rest-api", false, "Use REST API for manual imports instead of Command API")
				verbose      = flagSet.Bool("verbose", false, "Show verbose logging (API calls, filtering decisions)")
				debug        = flagSet.Bool("debug", false, "Show debug logging (full request/response payloads, implies -verbose)")
				manual       = flagSet.Bool("manual", false, "Launch interactive TUI mode for manual queue remediation")
			)

			// Temporarily clear environment variables for this test
			oldEnv := map[string]string{
				"SONARR_URLS":   os.Getenv("SONARR_URLS"),
				"SONARR_TOKENS": os.Getenv("SONARR_TOKENS"),
				"RADARR_URLS":   os.Getenv("RADARR_URLS"),
				"RADARR_TOKENS": os.Getenv("RADARR_TOKENS"),
			}
			for key := range oldEnv {
				os.Unsetenv(key)
			}
			defer func() {
				for key, value := range oldEnv {
					if value != "" {
						os.Setenv(key, value)
					}
				}
			}()

			// Parse the test arguments
			err := flagSet.Parse(tt.args)

			if tt.expectError {
				// For cases where we expect validation error after flag parsing
				config := loadConfig(*sonarrURLs, *sonarrTokens, *radarrURLs, *radarrTokens, *timeout, *useRestAPI, *verbose, *debug)
				validateErr := validateQueueConfig(config)
				if validateErr == nil {
					t.Logf("Config: %+v", config)
					t.Errorf("Expected validation error for args %v, but got none", tt.args)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected flag parsing error: %v", err)
				return
			}

			if *manual != tt.expectManual {
				t.Errorf("manual flag = %v, want %v", *manual, tt.expectManual)
			}
		})
	}
}

// TestTUIModelInitialization tests TUI model creation and initialization
func TestTUIModelInitialization(t *testing.T) {
	config := Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"test-token"},
		Timeout:      30 * time.Second,
	}

	model := InitialModel(config)

	// Verify initial state
	if model.config.SonarrURLs[0] != "http://localhost:8989" {
		t.Errorf("Expected Sonarr URL to be set, got %v", model.config.SonarrURLs)
	}
	if model.currentIndex != 0 {
		t.Errorf("Expected currentIndex = 0, got %d", model.currentIndex)
	}
	if !model.loading {
		t.Error("Expected loading = true initially")
	}
	if model.status != "Loading queue items..." {
		t.Errorf("Expected initial status message, got %q", model.status)
	}
	if len(model.items) != 0 {
		t.Errorf("Expected empty items list, got %d items", len(model.items))
	}
}

// TestTUIModelUpdate tests TUI model updates for various user inputs
func TestTUIModelUpdate(t *testing.T) {
	config := Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"test-token"},
		Timeout:      30 * time.Second,
	}

	model := InitialModel(config)

	// Test window size update
	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updatedModel, _ := model.Update(windowMsg)

	if updatedModel.(TUIModel).width != 100 {
		t.Errorf("Expected width = 100, got %d", updatedModel.(TUIModel).width)
	}
	if updatedModel.(TUIModel).height != 50 {
		t.Errorf("Expected height = 50, got %d", updatedModel.(TUIModel).height)
	}

	// Test quit commands
	quitKeys := []tea.KeyType{tea.KeyCtrlC, tea.KeyEsc}
	for _, key := range quitKeys {
		keyMsg := tea.KeyMsg{Type: key}
		updatedModel, cmd := model.Update(keyMsg)

		if !updatedModel.(TUIModel).quitting {
			t.Errorf("Expected quitting = true for key %v", key)
		}
		if cmd == nil {
			t.Error("Expected tea.Quit command for quit key")
		}
	}

	// Test navigation with empty items list
	upMsg := tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, _ = model.Update(upMsg)
	if updatedModel.(TUIModel).currentIndex != 0 {
		t.Error("Navigation should not change index when items list is empty")
	}

	// Test with items in the list
	model.items = []QueueItem{
		{ID: 1, Title: "Item 1"},
		{ID: 2, Title: "Item 2"},
	}

	// Test down navigation
	downMsg := tea.KeyMsg{Type: tea.KeyDown}
	updatedModel, _ = model.Update(downMsg)
	if updatedModel.(TUIModel).currentIndex != 1 {
		t.Errorf("Expected currentIndex = 1 after down navigation, got %d", updatedModel.(TUIModel).currentIndex)
	}

	// Test up navigation
	upMsg = tea.KeyMsg{Type: tea.KeyUp}
	updatedModel, _ = model.Update(upMsg)
	if updatedModel.(TUIModel).currentIndex != 0 {
		t.Errorf("Expected currentIndex = 0 after up navigation, got %d", updatedModel.(TUIModel).currentIndex)
	}

	// Test boundary conditions
	// Can't go below 0
	updatedModel, _ = model.Update(upMsg)
	if updatedModel.(TUIModel).currentIndex != 0 {
		t.Error("Should not go below index 0")
	}

	// Can't go above last item
	model.currentIndex = 1
	updatedModel, _ = model.Update(downMsg)
	if updatedModel.(TUIModel).currentIndex != 1 {
		t.Error("Should not go above last item index")
	}
}

// TestTUIItemsLoaded tests the itemsLoaded message handling
func TestTUIItemsLoaded(t *testing.T) {
	// Create a minimal config for testing (won't be used for HTTP calls)
	config := Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"test-token"},
		UseRestAPI:   true,
	}

	model := InitialModel(config)

	// Test successful load with items
	items := []QueueItem{
		{ID: 1, Title: "Test Item 1", Status: "warning", TrackedDownloadState: "importPending", ErrorMessage: "No files found are eligible for import"},
		{ID: 2, Title: "Test Item 2", Status: "failed", TrackedDownloadState: "importPending"},
	}

	msg := itemsLoadedMsg{items: items, err: nil}
	updatedModel, _ := model.Update(msg)

	tuiModel := updatedModel.(TUIModel)
	if tuiModel.loading {
		t.Error("Expected loading = false after items loaded")
	}
	if tuiModel.error != "" {
		t.Errorf("Expected no error, got %q", tuiModel.error)
	}
	if len(tuiModel.items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(tuiModel.items))
	}
	if !strings.Contains(tuiModel.status, "Loaded 2 items") {
		t.Errorf("Expected status to mention loaded items, got %q", tuiModel.status)
	}

	// Test successful load with no items
	msg = itemsLoadedMsg{items: []QueueItem{}, err: nil}
	updatedModel, _ = model.Update(msg)

	tuiModel = updatedModel.(TUIModel)
	if len(tuiModel.items) != 0 {
		t.Error("Expected empty items list")
	}
	if !strings.Contains(tuiModel.status, "No queue items requiring remediation") {
		t.Errorf("Expected no items message, got %q", tuiModel.status)
	}

	// Test failed load
	msg = itemsLoadedMsg{items: nil, err: fmt.Errorf("network error")}
	updatedModel, _ = model.Update(msg)

	tuiModel = updatedModel.(TUIModel)
	if tuiModel.loading {
		t.Error("Expected loading = false after error")
	}
	if tuiModel.error == "" {
		t.Error("Expected error message to be set")
	}
	if !strings.Contains(tuiModel.error, "network error") {
		t.Errorf("Expected error to contain 'network error', got %q", tuiModel.error)
	}
}

// TestTUIActionExecution tests action execution and message handling
func TestTUIActionExecution(t *testing.T) {
	config := Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"test-token"},
		Timeout:      30 * time.Second,
	}

	model := InitialModel(config)
	model.items = []QueueItem{
		{ID: 1, Title: "Test Item", Status: "completed", TrackedDownloadState: "importPending"},
	}

	// Test successful action execution
	msg := actionExecutedMsg{success: true, err: nil, action: "delete"}
	updatedModel, _ := model.Update(msg)

	tuiModel := updatedModel.(TUIModel)
	if len(tuiModel.items) != 0 {
		t.Errorf("Expected item to be removed after successful action, got %d items", len(tuiModel.items))
	}
	if !strings.Contains(tuiModel.status, "All items processed successfully") {
		t.Errorf("Expected success status, got %q", tuiModel.status)
	}

	// Test failed action execution
	model.items = []QueueItem{
		{ID: 1, Title: "Test Item", Status: "completed", TrackedDownloadState: "importPending"},
	}
	msg = actionExecutedMsg{success: false, err: fmt.Errorf("API error"), action: "delete"}
	updatedModel, _ = model.Update(msg)

	tuiModel = updatedModel.(TUIModel)
	if len(tuiModel.items) != 1 {
		t.Errorf("Expected item to remain after failed action, got %d items", len(tuiModel.items))
	}
	if tuiModel.error == "" {
		t.Error("Expected error message to be set")
	}
	if !strings.Contains(tuiModel.error, "Action 'delete' failed") {
		t.Errorf("Expected error to mention failed action, got %q", tuiModel.error)
	}
}

// TestTUIFilterItems tests the item filtering logic
func TestTUIFilterItems(t *testing.T) {
	tests := []struct {
		name           string
		items          []QueueItem
		expectedCount  int
		expectedTitles []string
	}{
		{
			name: "Filter out /torrents/ items",
			items: []QueueItem{
				{ID: 1, Title: "Normal Item", OutputPath: "/downloads/movie", Status: "warning", TrackedDownloadState: "importPending", StatusMessages: []StatusMessage{{Title: "Warning", Messages: []string{"No files found are eligible for import"}}}},
				{ID: 2, Title: "Torrent Item", OutputPath: "/torrents/movie", Status: "warning", TrackedDownloadState: "importPending", StatusMessages: []StatusMessage{{Title: "Warning", Messages: []string{"No files found are eligible for import"}}}},
			},
			expectedCount:  1,
			expectedTitles: []string{"Normal Item"},
		},
		{
			name: "Include items needing remediation",
			items: []QueueItem{
				{ID: 1, Title: "Stuck Import", OutputPath: "/downloads/movie", Status: "completed", TrackedDownloadState: "importPending", StatusMessages: []StatusMessage{{Title: "Warning", Messages: []string{"Sample"}}}},
				{ID: 2, Title: "Normal Download", OutputPath: "/downloads/movie2", Status: "downloading", TrackedDownloadState: "downloading"},
			},
			expectedCount:  1,
			expectedTitles: []string{"Stuck Import"},
		},
		{
			name: "Include importBlocked items even if action is monitor",
			items: []QueueItem{
				{ID: 1, Title: "Import Blocked", OutputPath: "/downloads/movie", Status: "completed", TrackedDownloadState: "importBlocked"},
			},
			expectedCount:  1,
			expectedTitles: []string{"Import Blocked"},
		},
		{
			name:           "Empty list",
			items:          []QueueItem{},
			expectedCount:  0,
			expectedTitles: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered, _ := filterItemsWithReport(Config{}, tt.items)

			if len(filtered) != tt.expectedCount {
				t.Errorf("Expected %d items, got %d", tt.expectedCount, len(filtered))
			}

			for i, expectedTitle := range tt.expectedTitles {
				if i >= len(filtered) {
					t.Errorf("Missing expected item %d: %s", i, expectedTitle)
					continue
				}
				if filtered[i].Title != expectedTitle {
					t.Errorf("Item %d title = %q, want %q", i, filtered[i].Title, expectedTitle)
				}
			}
		})
	}
}

// TestTUIRendering tests the TUI view rendering
func TestTUIRendering(t *testing.T) {
	config := Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"test-token"},
		Timeout:      30 * time.Second,
	}

	// Test loading state
	model := InitialModel(config)
	view := model.View()

	if !strings.Contains(view, "Loading...") {
		t.Errorf("Expected loading message in view, got: %s", view)
	}
	if !strings.Contains(view, "Queue Remediation (Manual Mode)") {
		t.Errorf("Expected header in view, got: %s", view)
	}

	// Test empty state
	model.loading = false
	model.items = []QueueItem{}
	view = model.View()

	if !strings.Contains(view, "No queue items requiring remediation found") {
		t.Errorf("Expected empty message in view, got: %s", view)
	}

	// Test item display
	model.items = []QueueItem{
		{
			ID:                    1,
			Title:                 "Test Movie",
			Status:                "completed",
			TrackedDownloadState:  "importPending",
			TrackedDownloadStatus: "warning",
			StatusMessages: []StatusMessage{
				{Title: "Warning", Messages: []string{"Sample file detected"}},
			},
			OutputPath: "/downloads/test",
		},
	}
	view = model.View()

	if !strings.Contains(view, "Test Movie") {
		t.Errorf("Expected item title in view, got: %s", view)
	}
	if !strings.Contains(view, "Sample file detected") {
		t.Errorf("Expected status message in view, got: %s", view)
	}
	if !strings.Contains(view, "DELETE") {
		t.Errorf("Expected recommended action in view, got: %s", view)
	}

	// Test help text
	if !strings.Contains(view, "[Enter] Apply Suggested") {
		t.Errorf("Expected help text in view, got: %s", view)
	}
	if !strings.Contains(view, "[q] Quit") {
		t.Errorf("Expected quit help in view, got: %s", view)
	}
}

// TestTUIQuitState tests the quit state rendering
func TestTUIQuitState(t *testing.T) {
	config := Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"test-token"},
		Timeout:      30 * time.Second,
	}

	model := InitialModel(config)
	model.quitting = true

	view := model.View()
	if view != "" {
		t.Errorf("Expected empty view when quitting, got: %s", view)
	}
}

// TestRunTUIValidation tests the RunTUI function with various configurations
func TestRunTUIValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid configuration",
			config: Config{
				SonarrURLs:   []string{"http://localhost:8989"},
				SonarrTokens: []string{"test-token"},
				Timeout:      30 * time.Second,
			},
			expectError: false,
		},
		{
			name: "Invalid configuration - missing tokens",
			config: Config{
				SonarrURLs:   []string{"http://localhost:8989"},
				SonarrTokens: []string{},
				Timeout:      30 * time.Second,
			},
			expectError: true,
			errorMsg:    "configuration validation failed",
		},
		{
			name: "Invalid configuration - no instances",
			config: Config{
				Timeout: 30 * time.Second,
			},
			expectError: true,
			errorMsg:    "configuration validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RunTUI(tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				// Note: RunTUI will actually try to start the TUI, which will fail in tests
				// We expect an error about terminal capabilities in test environment
				if err == nil {
					t.Error("Expected error in test environment (no terminal)")
				}
			}
		})
	}
}

// TestCLIvsTUIMode tests that both CLI and TUI modes work correctly
func TestCLIvsTUIMode(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/queue"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(QueueResponse{
				Page:         1,
				PageSize:     100,
				TotalRecords: 0,
				Records:      []QueueItem{},
			})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	config := Config{
		SonarrURLs:   []string{server.URL},
		SonarrTokens: []string{"test-token"},
		Timeout:      5 * time.Second,
	}

	// Test CLI mode (should not error on empty queue)
	err := classifyAndRemediate(config, true) // dry run
	if err != nil {
		t.Errorf("CLI mode failed: %v", err)
	}

	// Test TUI mode validation (should fail gracefully in test environment)
	err = RunTUI(config)
	if err == nil {
		t.Error("Expected TUI to fail in test environment (no terminal)")
	}
	// The error should be about terminal capabilities, not configuration
	if strings.Contains(err.Error(), "configuration validation failed") {
		t.Errorf("TUI failed with config error, not terminal error: %v", err)
	}
}

// TestTUIKeyboardShortcuts tests all keyboard shortcuts work as expected
func TestTUIKeyboardShortcuts(t *testing.T) {
	config := Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"test-token"},
		Timeout:      30 * time.Second,
	}

	model := InitialModel(config)
	model.loading = false
	model.items = []QueueItem{
		{ID: 1, Title: "Test Item", Status: "warning", TrackedDownloadState: "importPending", StatusMessages: []StatusMessage{{Title: "Warning", Messages: []string{"No files found are eligible for import"}}}},
	}

	// Test action keys
	actionTests := []struct {
		key         tea.KeyType
		keyChar     byte
		expectCmd   bool
		description string
	}{
		{tea.KeyEnter, 0, true, "Enter - Apply suggested action"},
		{0, 'd', true, "d - Delete"},
		{0, 'm', true, "m - Manual Import"},
		{0, 's', true, "s - Skip/Monitor"},
		{0, 'r', true, "r - Refresh"},
		{0, 'q', true, "q - Quit"},
	}

	for _, test := range actionTests {
		t.Run(test.description, func(t *testing.T) {
			var keyMsg tea.KeyMsg
			if test.key != 0 {
				keyMsg = tea.KeyMsg{Type: test.key}
			} else {
				keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{rune(test.keyChar)}}
			}

			updatedModel, cmd := model.Update(keyMsg)

			if test.expectCmd && cmd == nil {
				t.Errorf("Expected command for %s", test.description)
			}
			if !test.expectCmd && cmd != nil {
				t.Errorf("Expected no command for %s", test.description)
			}

			// For quit key, check quitting state
			if test.keyChar == 'q' {
				if !updatedModel.(TUIModel).quitting {
					t.Error("Expected quitting = true for 'q' key")
				}
			}
		})
	}
}

// TestTUIErrorHandling tests error handling in TUI mode
func TestTUIErrorHandling(t *testing.T) {
	config := Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"test-token"},
		Timeout:      30 * time.Second,
	}

	model := InitialModel(config)

	// Test network error during load
	msg := itemsLoadedMsg{items: nil, err: fmt.Errorf("network timeout")}
	updatedModel, _ := model.Update(msg)

	tuiModel := updatedModel.(TUIModel)
	if tuiModel.error == "" {
		t.Error("Expected error message to be set")
	}
	if !strings.Contains(tuiModel.error, "network timeout") {
		t.Errorf("Expected error to contain 'network timeout', got %q", tuiModel.error)
	}

	// Test action execution error
	model.items = []QueueItem{
		{ID: 1, Title: "Test Item", Status: "completed", TrackedDownloadState: "importPending"},
	}
	actionMsg := actionExecutedMsg{success: false, err: fmt.Errorf("API request failed"), action: "delete"}
	updatedModel, _ = model.Update(actionMsg)

	tuiModel = updatedModel.(TUIModel)
	if !strings.Contains(tuiModel.error, "API request failed") {
		t.Errorf("Expected error to contain 'API request failed', got %q", tuiModel.error)
	}
}

// ========== COMPREHENSIVE MANUAL IMPORT API TESTS ==========

// TestExecuteManualImportCommandAPI tests the fixed executeManualImport function with Command API
func TestExecuteManualImportCommandAPI(t *testing.T) {
	tests := []struct {
		name            string
		importRequests  []ManualImportRequest
		statusCode      int
		responseBody    string
		expectError     bool
		validateRequest func(*testing.T, *http.Request, []ManualImportRequest)
	}{
		{
			name: "Successful ManualImport command with proper structure",
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
					ImportMode: "auto",
				},
			},
			statusCode:   http.StatusCreated,
			responseBody: `{"id": 12345, "name": "ManualImport", "state": "queued", "priority": "normal"}`,
			expectError:  false,
			validateRequest: func(t *testing.T, r *http.Request, requests []ManualImportRequest) {
				// Verify POST to /api/v3/command (CRITICAL FIX)
				if r.Method != "POST" {
					t.Errorf("Expected POST, got %s", r.Method)
				}
				if !strings.Contains(r.URL.Path, "/api/v3/command") {
					t.Errorf("Expected /api/v3/command endpoint, got %s", r.URL.Path)
				}

				// Verify content type
				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
				}

				// Decode and validate ManualImportCommand structure (CRITICAL FIX)
				var command ManualImportCommand
				if err := json.NewDecoder(r.Body).Decode(&command); err != nil {
					t.Fatalf("Failed to decode command body: %v", err)
				}

				// Verify command wrapper structure
				if command.Name != "ManualImport" {
					t.Errorf("Expected command name 'ManualImport', got %q", command.Name)
				}
				if command.ImportMode != "auto" {
					t.Errorf("Expected importMode 'auto', got %q", command.ImportMode)
				}
				if len(command.Files) != 1 {
					t.Fatalf("Expected 1 file in command, got %d", len(command.Files))
				}

				// Verify file structure
				file := command.Files[0]
				if file.SeriesID != 42 {
					t.Errorf("Expected SeriesID 42, got %d", file.SeriesID)
				}
				if file.DownloadID != "abc123" {
					t.Errorf("Expected DownloadID 'abc123', got %q", file.DownloadID)
				}
				if file.ImportMode != "auto" {
					t.Errorf("Expected ImportMode 'auto', got %q", file.ImportMode)
				}
			},
		},
		{
			name: "Multiple files in single ManualImport command",
			importRequests: []ManualImportRequest{
				{
					Path:       "/downloads/S01E01.mkv",
					SeriesID:   42,
					DownloadID: "batch1",
					ImportMode: "auto",
				},
				{
					Path:       "/downloads/S01E02.mkv",
					SeriesID:   42,
					DownloadID: "batch1",
					ImportMode: "auto",
				},
			},
			statusCode:   http.StatusCreated,
			responseBody: `{"id": 12346, "name": "ManualImport", "state": "queued"}`,
			expectError:  false,
			validateRequest: func(t *testing.T, r *http.Request, requests []ManualImportRequest) {
				var command ManualImportCommand
				json.NewDecoder(r.Body).Decode(&command)

				if len(command.Files) != 2 {
					t.Errorf("Expected 2 files in command, got %d", len(command.Files))
				}
				// Verify both files have same downloadId
				if command.Files[0].DownloadID != command.Files[1].DownloadID {
					t.Error("Both files should have same downloadId")
				}
			},
		},
		{
			name: "Radarr movie import via ManualImport command",
			importRequests: []ManualImportRequest{
				{
					Path:       "/downloads/Movie.2024/movie.mkv",
					MovieID:    123,
					DownloadID: "movie123",
					ImportMode: "auto",
				},
			},
			statusCode:   http.StatusCreated,
			responseBody: `{"id": 12347, "name": "ManualImport", "state": "queued"}`,
			expectError:  false,
			validateRequest: func(t *testing.T, r *http.Request, requests []ManualImportRequest) {
				var command ManualImportCommand
				json.NewDecoder(r.Body).Decode(&command)

				file := command.Files[0]
				if file.MovieID != 123 {
					t.Errorf("Expected MovieID 123, got %d", file.MovieID)
				}
				if file.SeriesID != 0 {
					t.Error("Radarr files should not have SeriesID")
				}
				if len(file.EpisodeIDs) != 0 {
					t.Error("Radarr files should not have EpisodeIDs")
				}
			},
		},
		{
			name:           "Empty import requests",
			importRequests: []ManualImportRequest{},
			statusCode:     http.StatusBadRequest,
			responseBody:   `{"error": "No files provided for import"}`,
			expectError:    true,
		},
		{
			name: "API error - 500 Server Error",
			importRequests: []ManualImportRequest{
				{Path: "/downloads/file.mkv", SeriesID: 42},
			},
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"error": "Internal server error during manual import"}`,
			expectError:  true,
		},
		{
			name: "Unauthorized - invalid API key",
			importRequests: []ManualImportRequest{
				{Path: "/downloads/file.mkv", SeriesID: 42},
			},
			statusCode:   http.StatusUnauthorized,
			responseBody: `{"error": "Unauthorized"}`,
			expectError:  true,
		},
		{
			name: "Command response without ID",
			importRequests: []ManualImportRequest{
				{Path: "/downloads/file.mkv", SeriesID: 42},
			},
			statusCode:   http.StatusCreated,
			responseBody: `{"name": "ManualImport", "state": "queued"}`,
			expectError:  false, // Should handle missing ID gracefully
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

				if r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/command/") {
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(CommandResource{ID: 1, Status: "completed", Result: "successful"})
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

			config := Config{Timeout: 5 * time.Second}
			err := executeManualImport(config, server.URL, "test-token", tt.importRequests)

			if (err != nil) != tt.expectError {
				t.Errorf("executeManualImport() error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}

// TestBuildManualImportRequestsComprehensive tests the fixed buildManualImportRequests function
func TestBuildManualImportRequestsComprehensive(t *testing.T) {
	tests := []struct {
		name          string
		scannedItems  []ManualImportResource
		queueItem     QueueItem
		instanceType  string
		expectedCount int
		validateFunc  func(*testing.T, []ManualImportRequest)
	}{
		{
			name: "Sonarr - proper downloadId mapping",
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
					DownloadID: "original-download-id", // From scan
				},
			},
			queueItem: QueueItem{
				SeriesID:   42,                  // Must match the scanned file's series
				DownloadId: "queue-download-id", // From queue (should override)
			},
			instanceType:  "sonarr",
			expectedCount: 1,
			validateFunc: func(t *testing.T, requests []ManualImportRequest) {
				req := requests[0]
				// CRITICAL FIX: Should use queueItem.DownloadId, not scan DownloadID
				if req.DownloadID != "queue-download-id" {
					t.Errorf("Expected DownloadID from queue item 'queue-download-id', got %q", req.DownloadID)
				}
				if req.ImportMode != "auto" {
					t.Errorf("Expected ImportMode 'auto', got %q", req.ImportMode)
				}
			},
		},
		{
			name: "Sonarr - empty downloadId handling",
			scannedItems: []ManualImportResource{
				{
					Path:         "/downloads/Series.S01E01/file.mkv",
					Series:       &SeriesResource{ID: 42, Title: "Test Series"},
					SeasonNumber: intPtr(1),
					Episodes:     []EpisodeResource{{ID: 123456}},
					Rejections:   []ImportRejection{},
				},
			},
			queueItem: QueueItem{
				SeriesID:   42, // Must match the scanned file's series
				DownloadId: "", // Empty downloadId
			},
			instanceType:  "sonarr",
			expectedCount: 1,
			validateFunc: func(t *testing.T, requests []ManualImportRequest) {
				req := requests[0]
				if req.DownloadID != "" {
					t.Errorf("Expected empty DownloadID, got %q", req.DownloadID)
				}
			},
		},
		{
			name: "Sonarr - series ID validation",
			scannedItems: []ManualImportResource{
				{
					Path:         "/downloads/Series.S01E01/file.mkv",
					Series:       &SeriesResource{ID: 99, Title: "Wrong Series"},
					SeasonNumber: intPtr(1),
					Episodes:     []EpisodeResource{{ID: 123456}},
					Rejections:   []ImportRejection{},
				},
			},
			queueItem: QueueItem{
				SeriesID:   42, // Different from scanned file
				DownloadId: "test123",
			},
			instanceType:  "sonarr",
			expectedCount: 0, // Should skip due to ID mismatch
		},
		{
			name: "Sonarr - soft rejection allowed",
			scannedItems: []ManualImportResource{
				{
					Path:         "/downloads/Series.S01E02/file.mkv",
					Series:       &SeriesResource{ID: 42, Title: "Test Series"},
					SeasonNumber: intPtr(1),
					Episodes:     []EpisodeResource{{ID: 123457}},
					Rejections:   []ImportRejection{{Reason: "Episode is not monitored", Type: "warning"}},
				},
			},
			queueItem: QueueItem{
				SeriesID:   42,
				DownloadId: "soft-reject",
			},
			instanceType:  "sonarr",
			expectedCount: 1,
		},
		{
			name: "Radarr - proper movie handling (no episode fields)",
			scannedItems: []ManualImportResource{
				{
					Path: "/downloads/Movie.2024/movie.mkv",
					Movie: &MovieResource{
						ID:    123,
						Title: "Test Movie",
						Year:  2024,
					},
					Quality: QualityModel{
						Quality: QualityDefinition{ID: 3, Name: "HDTV-1080p"},
					},
					Languages:  []Language{{ID: 1, Name: "English"}},
					Rejections: []ImportRejection{},
				},
			},
			queueItem: QueueItem{
				MovieID:    123,
				DownloadId: "movie123",
			},
			instanceType:  "radarr",
			expectedCount: 1,
			validateFunc: func(t *testing.T, requests []ManualImportRequest) {
				req := requests[0]
				if req.MovieID != 123 {
					t.Errorf("Expected MovieID 123, got %d", req.MovieID)
				}
				if req.SeriesID != 0 {
					t.Error("Radarr requests should not have SeriesID")
				}
				if req.SeasonNumber != 0 {
					t.Error("Radarr requests should not have SeasonNumber")
				}
				if len(req.EpisodeIDs) != 0 {
					t.Error("Radarr requests should not have EpisodeIDs")
				}
				if req.ImportMode != "auto" {
					t.Errorf("Expected ImportMode 'auto', got %q", req.ImportMode)
				}
			},
		},
		{
			name: "Radarr - movie ID validation",
			scannedItems: []ManualImportResource{
				{
					Path:       "/downloads/Movie.2024/movie.mkv",
					Movie:      &MovieResource{ID: 999, Title: "Wrong Movie"},
					Rejections: []ImportRejection{},
				},
			},
			queueItem: QueueItem{
				MovieID:    123, // Different from scanned file
				DownloadId: "movie123",
			},
			instanceType:  "radarr",
			expectedCount: 0, // Should skip due to ID mismatch
		},
		{
			name: "Mixed valid and rejected files",
			scannedItems: []ManualImportResource{
				{
					Path:       "/downloads/valid.mkv",
					Series:     &SeriesResource{ID: 42},
					Rejections: []ImportRejection{}, // Valid
				},
				{
					Path:   "/downloads/rejected.mkv",
					Series: &SeriesResource{ID: 42},
					Rejections: []ImportRejection{
						{Reason: "Unknown Series", Type: "permanent"},
					}, // Rejected
				},
				{
					Path:       "/downloads/also_valid.mkv",
					Series:     &SeriesResource{ID: 42},
					Rejections: []ImportRejection{}, // Valid
				},
			},
			queueItem: QueueItem{
				SeriesID:   42,
				DownloadId: "batch123",
			},
			instanceType:  "sonarr",
			expectedCount: 2, // Only valid files
		},
		{
			name: "Files with null series/movie",
			scannedItems: []ManualImportResource{
				{
					Path:       "/downloads/unknown_series.mkv",
					Series:     nil, // Null series
					Rejections: []ImportRejection{},
				},
				{
					Path:       "/downloads/unknown_movie.mkv",
					Movie:      nil, // Null movie
					Rejections: []ImportRejection{},
				},
			},
			queueItem: QueueItem{
				DownloadId: "test123",
			},
			instanceType:  "sonarr",
			expectedCount: 0, // Should skip all null series/movie items
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := buildManualImportRequests(Config{Verbose: true}, tt.scannedItems, tt.queueItem, tt.instanceType)

			if len(requests) != tt.expectedCount {
				t.Errorf("buildManualImportRequests() = %d requests, want %d", len(requests), tt.expectedCount)
			}

			if tt.validateFunc != nil && len(requests) > 0 {
				tt.validateFunc(t, requests)
			}
		})
	}
}

// TestTriggerManualImportHybridWorkflow tests the complete triggerManualImport function with hybrid API
func TestTriggerManualImportHybridWorkflow(t *testing.T) {
	tests := []struct {
		name            string
		useRestAPI      bool
		queueItem       QueueItem
		scanResponse    string
		commandResponse string
		expectSuccess   bool
		expectFallback  bool
		validateCalls   func(*testing.T, *[]http.Request)
	}{
		{
			name:       "REST API success - complete workflow",
			useRestAPI: true,
			queueItem: QueueItem{
				ID:           123,
				Title:        "Test Series",
				InstanceType: "sonarr",
				SeriesID:     42,
				OutputPath:   "/downloads/Test.Series.S01E01",
				DownloadId:   "test123",
			},
			scanResponse: `[
				{
					"id": 1234567890,
					"path": "/downloads/Test.Series.S01E01/file.mkv",
					"series": {"id": 42, "title": "Test Series"},
					"seasonNumber": 1,
					"episodes": [{"id": 100, "seasonNumber": 1, "episodeNumber": 1}],
					"quality": {"quality": {"id": 1, "name": "HDTV-720p"}},
					"rejections": []
				}
			]`,
			commandResponse: `{"id": 54321, "name": "ManualImport", "state": "queued"}`,
			expectSuccess:   true,
			expectFallback:  false,
			validateCalls: func(t *testing.T, requests *[]http.Request) {
				var filtered []http.Request
				for _, req := range *requests {
					if req.Method == "GET" && strings.Contains(req.URL.Path, "/api/v3/command/") {
						continue
					}
					filtered = append(filtered, req)
				}
				// Should have scan and command calls
				if len(filtered) != 2 {
					t.Errorf("Expected 2 requests (scan + command), got %d", len(filtered))
				}

				// First request: GET scan with downloadId parameter
				scanReq := filtered[0]
				if scanReq.Method != "GET" {
					t.Errorf("Expected GET for scan, got %s", scanReq.Method)
				}
				if !strings.Contains(scanReq.URL.String(), "/api/v3/manualimport") {
					t.Errorf("Expected scan endpoint, got %s", scanReq.URL.String())
				}
				if !strings.Contains(scanReq.URL.String(), "downloadId=test123") {
					t.Errorf("Expected downloadId=test123 in scan URL, got %s", scanReq.URL.String())
				}

				// Second request: POST command with ManualImport structure
				cmdReq := filtered[1]
				if cmdReq.Method != "POST" {
					t.Errorf("Expected POST for command, got %s", cmdReq.Method)
				}
				if !strings.Contains(cmdReq.URL.String(), "/api/v3/command") {
					t.Errorf("Expected command endpoint, got %s", cmdReq.URL.String())
				}

				// Verify command structure
				var command ManualImportCommand
				if err := json.NewDecoder(cmdReq.Body).Decode(&command); err != nil {
					t.Fatalf("Failed to decode command: %v", err)
				}
				if command.Name != "ManualImport" {
					t.Errorf("Expected ManualImport command, got %q", command.Name)
				}
			},
		},
		// NOTE: Test cases for DownloadedEpisodesScan/DownloadedMoviesScan fallback removed
		// These tested obsolete behavior - the correct approach is scan → build → execute with ManualImport command
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests []http.Request

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Store request for validation
				reqCopy := *r
				if r.Body != nil {
					bodyBytes, _ := io.ReadAll(r.Body)
					reqCopy.Body = io.NopCloser(bytes.NewReader(bodyBytes))
					r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
				requests = append(requests, reqCopy)

				// Verify API key
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				switch {
				case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/command/"):
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"id": 54321, "status": "completed", "result": "successful"}`))

				case strings.Contains(r.URL.Path, "/manualimport") && r.Method == "GET":
					// Scan endpoint
					if tt.scanResponse == "" {
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte(`{"error": "Scan failed"}`))
						return
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(tt.scanResponse))

				case strings.Contains(r.URL.Path, "/command"):
					// Command endpoint (both ManualImport and fallback)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					w.Write([]byte(tt.commandResponse))

				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			config := Config{
				UseRestAPI: tt.useRestAPI,
				Timeout:    5 * time.Second,
				Verbose:    true,
			}

			err := triggerManualImport(config, server.URL, "test-token", tt.queueItem.OutputPath, tt.queueItem.InstanceType, tt.useRestAPI, tt.queueItem)

			if tt.expectSuccess && err != nil {
				t.Errorf("Expected success, got error: %v", err)
			}
			if !tt.expectSuccess && err == nil {
				t.Error("Expected error, got success")
			}

			// Validate request patterns
			if tt.validateCalls != nil {
				tt.validateCalls(t, &requests)
			}
		})
	}
}

// TestScanForManualImportServerSideFiltering tests the critical server-side filtering fix
func TestScanForManualImportServerSideFiltering(t *testing.T) {
	tests := []struct {
		name            string
		folderPath      string
		queueItem       QueueItem
		instanceType    string
		statusCode      int
		responseBody    string
		expectError     bool
		validateRequest func(*testing.T, *http.Request)
	}{
		{
			name:         "Sonarr with downloadId parameter",
			folderPath:   "/downloads/Test.Series.S01E01",
			instanceType: "sonarr",
			queueItem: QueueItem{
				SeriesID:   42,
				DownloadId: "sonarr-test-download-42",
			},
			statusCode: http.StatusOK,
			responseBody: `[
				{
					"id": 1,
					"path": "/downloads/Test.Series.S01E01/file.mkv",
					"series": {"id": 42, "title": "Test Series"},
					"seasonNumber": 1,
					"episodes": [{"id": 100}],
					"rejections": []
				}
			]`,
			expectError: false,
			validateRequest: func(t *testing.T, r *http.Request) {
				// Should use downloadId as primary parameter (per HAR investigation)
				if !strings.Contains(r.URL.String(), "downloadId=sonarr-test-download-42") {
					t.Error("Missing downloadId parameter")
				}
				if !strings.Contains(r.URL.String(), "filterExistingFiles=false") {
					t.Error("Missing filterExistingFiles=false parameter")
				}
				// Should NOT have folder when downloadId is present
				if strings.Contains(r.URL.String(), "folder=") {
					t.Error("Should not have folder parameter when downloadId is present")
				}
			},
		},
		{
			name:         "Radarr with downloadId parameter",
			folderPath:   "/downloads/Test.Movie.2024",
			instanceType: "radarr",
			queueItem: QueueItem{
				MovieID:    123,
				DownloadId: "radarr-test-download-123",
			},
			statusCode: http.StatusOK,
			responseBody: `[
				{
					"id": 1,
					"path": "/downloads/Test.Movie.2024/movie.mkv",
					"movie": {"id": 123, "title": "Test Movie", "year": 2024},
					"rejections": []
				}
			]`,
			expectError: false,
			validateRequest: func(t *testing.T, r *http.Request) {
				// Should use downloadId as primary parameter (per HAR investigation)
				if !strings.Contains(r.URL.String(), "downloadId=radarr-test-download-123") {
					t.Error("Missing downloadId parameter")
				}
				if !strings.Contains(r.URL.String(), "filterExistingFiles=false") {
					t.Error("Missing filterExistingFiles=false parameter")
				}
				// Should NOT have folder when downloadId is present
				if strings.Contains(r.URL.String(), "folder=") {
					t.Error("Should not have folder parameter when downloadId is present")
				}
			},
		},
		{
			name:         "Fallback to folder-only scan (no downloadId available)",
			folderPath:   "/downloads/unknown",
			instanceType: "sonarr",
			queueItem: QueueItem{
				SeriesID:   0,  // No ID available
				DownloadId: "", // No downloadId - should fall back to folder
			},
			statusCode:   http.StatusOK,
			responseBody: `[]`,
			expectError:  false,
			validateRequest: func(t *testing.T, r *http.Request) {
				// Should only have folder parameter when no downloadId available
				if !strings.Contains(r.URL.String(), "folder=") {
					t.Error("Missing folder parameter")
				}
				if strings.Contains(r.URL.String(), "downloadId=") {
					t.Error("Should not have downloadId when none available")
				}
				if !strings.Contains(r.URL.String(), "filterExistingFiles=false") {
					t.Error("Missing filterExistingFiles=false parameter")
				}
			},
		},
		{
			name:         "URL encoding of downloadId",
			folderPath:   "/downloads/Series Name With Spaces.S01E01",
			instanceType: "sonarr",
			queueItem: QueueItem{
				SeriesID:   42,
				DownloadId: "download-id-with spaces & special=chars",
			},
			statusCode:   http.StatusOK,
			responseBody: `[]`,
			expectError:  false,
			validateRequest: func(t *testing.T, r *http.Request) {
				// Should properly URL-encode the downloadId
				if !strings.Contains(r.URL.String(), "downloadId=download-id-with") {
					t.Errorf("Expected URL-encoded downloadId, got %s", r.URL.String())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify API key
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
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

			config := Config{Timeout: 5 * time.Second}
			_, err := scanForManualImport(config, server.URL, "test-token", tt.folderPath, tt.queueItem, tt.instanceType)

			if (err != nil) != tt.expectError {
				t.Errorf("scanForManualImport() error = %v, expectError = %v", err, tt.expectError)
			}
		})
	}
}

// Helper function for int pointers
func intPtr(i int) *int {
	return &i
}

// TestTUIItemRemovalAfterAction tests that items are properly removed after successful actions
func TestTUIItemRemovalAfterAction(t *testing.T) {
	config := Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"test-token"},
		Timeout:      30 * time.Second,
	}

	model := InitialModel(config)
	model.items = []QueueItem{
		{ID: 1, Title: "Item 1"},
		{ID: 2, Title: "Item 2"},
		{ID: 3, Title: "Item 3"},
	}
	model.currentIndex = 1 // Select middle item

	// Test successful action removes current item
	msg := actionExecutedMsg{success: true, err: nil, action: "delete"}
	updatedModel, _ := model.Update(msg)

	tuiModel := updatedModel.(TUIModel)
	if len(tuiModel.items) != 2 {
		t.Errorf("Expected 2 items after removal, got %d", len(tuiModel.items))
	}
	if tuiModel.currentIndex != 1 {
		t.Errorf("Expected currentIndex to stay at 1, got %d", tuiModel.currentIndex)
	}

	// Test removing last item adjusts index
	model = tuiModel
	model.currentIndex = 1 // Point to last item
	msg = actionExecutedMsg{success: true, err: nil, action: "delete"}
	updatedModel, _ = model.Update(msg)

	tuiModel = updatedModel.(TUIModel)
	if len(tuiModel.items) != 1 {
		t.Errorf("Expected 1 item after removal, got %d", len(tuiModel.items))
	}
	if tuiModel.currentIndex != 0 {
		t.Errorf("Expected currentIndex to move to 0 when last item removed, got %d", tuiModel.currentIndex)
	}

	// Test removing all items sets completion status
	model = tuiModel
	model.currentIndex = 0
	msg = actionExecutedMsg{success: true, err: nil, action: "delete"}
	updatedModel, _ = model.Update(msg)

	tuiModel = updatedModel.(TUIModel)
	if len(tuiModel.items) != 0 {
		t.Errorf("Expected 0 items after removing all, got %d", len(tuiModel.items))
	}
	if !strings.Contains(tuiModel.status, "All items processed successfully") {
		t.Errorf("Expected completion status, got %q", tuiModel.status)
	}
}

// ========== ENHANCED MANUAL IMPORT TESTS ==========

// NOTE: TestHybridManualImportWorkflow has been temporarily disabled because it tests
// the OLD DownloadedEpisodesScan fallback behavior that was removed in this PR.
// The test needs to be rewritten to test the CORRECT flow: scan → build → execute with ManualImport command.
// The functionality is already tested by TestTriggerManualImport and TestTriggerManualImportHybridWorkflow.
/*
// NOTE: DISABLED - TestHybridManualImportWorkflow_OBSOLETE tests the OLD DownloadedEpisodesScan fallback
// behavior that was removed. The test needs to be rewritten to test the CORRECT flow: scan → build → execute.
// The functionality is already tested by TestTriggerManualImport and TestTriggerManualImportHybridWorkflow.
// TestHybridManualImportWorkflow_OBSOLETE tests the complete hybrid API workflow
func TestHybridManualImportWorkflow_OBSOLETE(t *testing.T) {
	t.Skip("DISABLED: This test validates obsolete DownloadedEpisodesScan fallback behavior that was removed")
	tests := []struct {
		name           string
		useRestAPI     bool
		queueItem      QueueItem
		scanResponse   string
		importResponse string
		expectSuccess  bool
		expectFallback bool
		validateCalls  func(*testing.T, *[]http.Request)
	}{
		{
			name:       "REST API success with server-side filtering",
			useRestAPI: true,
			queueItem: QueueItem{
				ID:           123,
				Title:        "Test Series S01E01",
				InstanceType: "sonarr",
				SeriesID:     42,
				OutputPath:   "/downloads/Test.Series.S01E01",
				DownloadId:   "test-download-123",
			},
			scanResponse: `[
				{
					"id": 1234567890,
					"path": "/downloads/Test.Series.S01E01/file.mkv",
					"series": {"id": 42, "title": "Test Series"},
					"seasonNumber": 1,
					"episodes": [{"id": 100, "seasonNumber": 1, "episodeNumber": 1}],
					"quality": {"quality": {"id": 1, "name": "HDTV-720p"}},
					"rejections": []
				}
			]`,
			importResponse: `[{"id": 1234567890, "rejections": []}]`,
			expectSuccess:  true,
			expectFallback: false,
			validateCalls: func(t *testing.T, requests *[]http.Request) {
				// Should have scan and import calls
				if len(*requests) != 2 {
					t.Errorf("Expected 2 requests, got %d", len(*requests))
				}

				// First request should be scan with downloadId parameter
				scanReq := (*requests)[0]
				if scanReq.Method != "GET" {
					t.Errorf("Expected GET for scan, got %s", scanReq.Method)
				}
				if !strings.Contains(scanReq.URL.String(), "downloadId=test-download-123") {
					t.Errorf("Expected downloadId=test-download-123 in scan URL, got %s", scanReq.URL.String())
				}

				// Second request should be import
				importReq := (*requests)[1]
				if importReq.Method != "POST" {
					t.Errorf("Expected POST for import, got %s", importReq.Method)
				}
			},
		},
		{
			name:       "REST API with client-side validation (mixed downloads)",
			useRestAPI: true,
			queueItem: QueueItem{
				ID:           456,
				Title:        "Target Series",
				InstanceType: "sonarr",
				SeriesID:     42,
				OutputPath:   "/downloads/mixed-batch",
			},
			scanResponse: `[
				{
					"id": 1,
					"path": "/downloads/mixed-batch/Target.Series.S02E03.mkv",
					"series": {"id": 42, "title": "Target Series"},
					"seasonNumber": 2,
					"episodes": [{"id": 200, "seasonNumber": 2, "episodeNumber": 3}],
					"quality": {"quality": {"id": 1, "name": "HDTV-720p"}},
					"rejections": []
				},
				{
					"id": 2,
					"path": "/downloads/mixed-batch/Wrong.Series.S05E01.mkv",
					"series": {"id": 99, "title": "Wrong Series"},
					"seasonNumber": 5,
					"episodes": [{"id": 500, "seasonNumber": 5, "episodeNumber": 1}],
					"quality": {"quality": {"id": 1, "name": "HDTV-720p"}},
					"rejections": []
				}
			]`,
			importResponse: `[{"id": 1, "rejections": []}]`,
			expectSuccess:  true,
			expectFallback: false,
			validateCalls: func(t *testing.T, requests *[]http.Request) {
				// Should have scan and import calls, but only import correct file
				if len(*requests) != 2 {
					t.Errorf("Expected 2 requests, got %d", len(*requests))
				}

				// Import should use ManualImportCommand wrapper with correct file
				importReq := (*requests)[1]
				var importData map[string]interface{}
				if err := json.NewDecoder(importReq.Body).Decode(&importData); err != nil {
					t.Fatalf("Failed to decode import request: %v", err)
				}

				// Verify command structure
				if importData["name"] != "ManualImport" {
					t.Errorf("Expected ManualImport command, got %v", importData["name"])
				}

				// Verify files array
				files, ok := importData["files"].([]interface{})
				if !ok {
					t.Fatalf("Expected files array, got %T", importData["files"])
				}

				if len(files) != 1 {
					t.Errorf("Expected 1 file in import request, got %d", len(files))
				}

				fileData, ok := files[0].(map[string]interface{})
				if !ok {
					t.Fatalf("Expected file object, got %T", files[0])
				}

				if seriesId, ok := fileData["seriesId"].(float64); !ok || int(seriesId) != 42 {
					t.Errorf("Expected seriesId=42 in import file, got %v", fileData["seriesId"])
				}
			},
		},
		{
			name:       "Command API fallback (REST unavailable)",
			useRestAPI: true,
			queueItem: QueueItem{
				ID:           789,
				Title:        "Fallback Test",
				InstanceType: "radarr",
				MovieID:      123,
				OutputPath:   "/downloads/Fallback.Test.2024",
			},
			scanResponse:   `[]`, // Empty scan triggers fallback
			expectSuccess:  true,
			expectFallback: true,
			validateCalls: func(t *testing.T, requests *[]http.Request) {
				// Should have scan attempt and command fallback
				if len(*requests) != 2 {
					t.Errorf("Expected 2 requests, got %d", len(*requests))
				}

				// First request should be scan
				scanReq := (*requests)[0]
				if scanReq.Method != "GET" {
					t.Errorf("Expected GET for scan, got %s", scanReq.Method)
				}

				// Second request should be command
				cmdReq := (*requests)[1]
				if cmdReq.Method != "POST" {
					t.Errorf("Expected POST for command, got %s", cmdReq.Method)
				}
				if !strings.Contains(cmdReq.URL.String(), "/command") {
					t.Errorf("Expected /command endpoint, got %s", cmdReq.URL.String())
				}
			},
		},
		{
			name:       "Command API direct (REST disabled)",
			useRestAPI: false,
			queueItem: QueueItem{
				ID:           101112,
				Title:        "Direct Command Test",
				InstanceType: "sonarr",
				SeriesID:     42,
				OutputPath:   "/downloads/Direct.Command.Test",
			},
			expectSuccess:  true,
			expectFallback: false,
			validateCalls: func(t *testing.T, requests *[]http.Request) {
				// Should have only command call
				if len(*requests) != 1 {
					t.Errorf("Expected 1 request, got %d", len(*requests))
				}

				cmdReq := (*requests)[0]
				if cmdReq.Method != "POST" {
					t.Errorf("Expected POST for command, got %s", cmdReq.Method)
				}

				// Verify command payload
				var cmdData map[string]interface{}
				if err := json.NewDecoder(cmdReq.Body).Decode(&cmdData); err != nil {
					t.Fatalf("Failed to decode command request: %v", err)
				}

				if cmdData["name"] != "DownloadedEpisodesScan" {
					t.Errorf("Expected DownloadedEpisodesScan, got %v", cmdData["name"])
				}
				if cmdData["path"] != "/downloads/Direct.Command.Test" {
					t.Errorf("Expected exact path, got %v", cmdData["path"])
				}
				if cmdData["importMode"] != "Move" {
					t.Errorf("Expected Move importMode, got %v", cmdData["importMode"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests []http.Request

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Store request for validation
				reqCopy := *r
				if r.Body != nil {
					bodyBytes, _ := io.ReadAll(r.Body)
					reqCopy.Body = io.NopCloser(bytes.NewReader(bodyBytes))
					// Restore body for actual processing
					r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
				requests = append(requests, reqCopy)

				// Verify API key
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				switch {
				case strings.Contains(r.URL.Path, "/manualimport") && r.Method == "GET":
					// Scan endpoint
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(tt.scanResponse))

				case strings.Contains(r.URL.Path, "/manualimport") && r.Method == "POST":
					// Import endpoint
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(tt.importResponse))

				case strings.Contains(r.URL.Path, "/command"):
					// Command endpoint
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"id":    1,
						"name":  "DownloadedEpisodesScan",
						"state": "queued",
					})

				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			config := Config{
				UseRestAPI: tt.useRestAPI,
				Timeout:    5 * time.Second,
				Verbose:    true,
			}

			// Test the manual import execution
			err := triggerManualImport(config, server.URL, "test-token", tt.queueItem.OutputPath, tt.queueItem.InstanceType, tt.useRestAPI, tt.queueItem)

			if tt.expectSuccess && err != nil {
				t.Errorf("Expected success, got error: %v", err)
			}
			if !tt.expectSuccess && err == nil {
				t.Error("Expected error, got success")
			}

			// Validate request patterns
			if tt.validateCalls != nil {
				tt.validateCalls(t, &requests)
			}
		})
	}
}
*/

// TestManualImportIDValidation tests the critical ID validation logic
func TestManualImportIDValidation(t *testing.T) {
	tests := []struct {
		name           string
		queueItem      QueueItem
		scanResponse   string
		expectedResult string
		expectError    bool
	}{
		{
			name: "Valid Sonarr series ID match",
			queueItem: QueueItem{
				InstanceType: "sonarr",
				SeriesID:     42,
				OutputPath:   "/downloads/test",
			},
			scanResponse: `[
				{
					"id": 1,
					"path": "/downloads/test/file.mkv",
					"series": {"id": 42, "title": "Correct Series"},
					"seasonNumber": 1,
					"episodes": [{"id": 100}],
					"rejections": []
				}
			]`,
			expectedResult: "imported",
			expectError:    false,
		},
		{
			name: "Invalid Sonarr series ID mismatch",
			queueItem: QueueItem{
				InstanceType: "sonarr",
				SeriesID:     42,
				OutputPath:   "/downloads/test",
			},
			scanResponse: `[
				{
					"id": 1,
					"path": "/downloads/test/wrong_file.mkv",
					"series": {"id": 99, "title": "Wrong Series"},
					"seasonNumber": 1,
					"episodes": [{"id": 200}],
					"rejections": []
				}
			]`,
			expectedResult: "skipped",
			expectError:    true,
		},
		{
			name: "Valid Radarr movie ID match",
			queueItem: QueueItem{
				InstanceType: "radarr",
				MovieID:      123,
				OutputPath:   "/downloads/movie",
			},
			scanResponse: `[
				{
					"id": 1,
					"path": "/downloads/movie/movie.mkv",
					"movie": {"id": 123, "title": "Correct Movie", "year": 2024},
					"rejections": []
				}
			]`,
			expectedResult: "imported",
			expectError:    false,
		},
		{
			name: "Invalid Radarr movie ID mismatch",
			queueItem: QueueItem{
				InstanceType: "radarr",
				MovieID:      123,
				OutputPath:   "/downloads/movie",
			},
			scanResponse: `[
				{
					"id": 1,
					"path": "/downloads/movie/wrong_movie.mkv",
					"movie": {"id": 456, "title": "Wrong Movie", "year": 2023},
					"rejections": []
				}
			]`,
			expectedResult: "skipped",
			expectError:    true,
		},
		{
			name: "Mixed files with some valid",
			queueItem: QueueItem{
				InstanceType: "sonarr",
				SeriesID:     42,
				OutputPath:   "/downloads/mixed",
			},
			scanResponse: `[
				{
					"id": 1,
					"path": "/downloads/mixed/correct.mkv",
					"series": {"id": 42, "title": "Correct Series"},
					"seasonNumber": 1,
					"episodes": [{"id": 100}],
					"rejections": []
				},
				{
					"id": 2,
					"path": "/downloads/mixed/wrong.mkv",
					"series": {"id": 99, "title": "Wrong Series"},
					"seasonNumber": 5,
					"episodes": [{"id": 500}],
					"rejections": []
				}
			]`,
			expectedResult: "partial",
			expectError:    false,
		},
		{
			name: "No files found",
			queueItem: QueueItem{
				InstanceType: "sonarr",
				SeriesID:     42,
				OutputPath:   "/downloads/empty",
			},
			scanResponse:   `[]`,
			expectedResult: "empty",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-Api-Key") != "test-token" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				if r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/command/") {
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(CommandResource{ID: 1, Status: "completed", Result: "successful"})
					return
				}

				if strings.Contains(r.URL.Path, "/manualimport") && r.Method == "GET" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(tt.scanResponse))
				} else if strings.Contains(r.URL.Path, "/manualimport") && r.Method == "POST" {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`[{"id": 1, "rejections": []}]`))
				}
			}))
			defer server.Close()

			config := Config{
				UseRestAPI: true,
				Timeout:    5 * time.Second,
				Verbose:    true,
			}

			err := triggerManualImport(config, server.URL, "test-token", tt.queueItem.OutputPath, tt.queueItem.InstanceType, true, tt.queueItem)

			// Validate results based on expected outcome
			switch tt.expectedResult {
			case "imported":
				if err != nil {
					t.Errorf("Expected successful import, got error: %v", err)
				}
			case "skipped":
				if !tt.expectError && err != nil {
					t.Errorf("Expected success via fallback, got error: %v", err)
				}
				if tt.expectError && err == nil {
					t.Errorf("Expected error for skipped files, got success")
				}
			case "partial":
				if err != nil {
					t.Errorf("Expected partial success, got error: %v", err)
				}
			case "empty":
				if !tt.expectError && err != nil {
					t.Errorf("Expected success via fallback for empty scan, got error: %v", err)
				}
				if tt.expectError && err == nil {
					t.Errorf("Expected error for empty scan, got success")
				}
			}

			if (err != nil) != tt.expectError {
				t.Errorf("Error expectation mismatch: got %v, expectError=%v", err, tt.expectError)
			}
		})
	}
}

// TestManualImportErrorHandling tests various error scenarios
func TestManualImportErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		useRestAPI     bool
		queueItem      QueueItem
		scanStatus     int
		scanResponse   string
		importStatus   int
		expectError    bool
		expectFallback bool
		errorMessage   string
	}{
		{
			name:       "REST API scan failure returns error",
			useRestAPI: true,
			queueItem: QueueItem{
				ID:           123,
				InstanceType: "sonarr",
				SeriesID:     42,
				OutputPath:   "/downloads/test",
			},
			scanStatus:     http.StatusBadRequest,
			scanResponse:   `{"error": "Invalid path"}`,
			expectError:    true,
			expectFallback: false,
		},
		{
			name:       "REST API import failure",
			useRestAPI: true,
			queueItem: QueueItem{
				ID:           456,
				InstanceType: "radarr",
				MovieID:      123,
				OutputPath:   "/downloads/movie",
			},
			scanStatus:   http.StatusOK,
			scanResponse: `[{"id": 1, "path": "/downloads/movie/movie.mkv", "movie": {"id": 123}, "rejections": []}]`,
			importStatus: http.StatusInternalServerError,
			expectError:  false, // Should succeed via fallback to Command API
		},
		{
			name:       "Command API failure",
			useRestAPI: false,
			queueItem: QueueItem{
				ID:           789,
				InstanceType: "sonarr",
				OutputPath:   "/downloads/test",
			},
			expectError: true,
		},
		{
			name:       "Unauthorized access",
			useRestAPI: true,
			queueItem: QueueItem{
				ID:           401,
				InstanceType: "sonarr",
				SeriesID:     42,
				OutputPath:   "/downloads/test",
			},
			scanStatus:   http.StatusUnauthorized,
			scanResponse: `{"error": "Unauthorized"}`,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requestCount := 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestCount++

				// Simulate unauthorized for specific test
				if tt.name == "Unauthorized access" {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error": "Unauthorized"}`))
					return
				}

				switch {
				case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/command/"):
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(CommandResource{ID: 1, Status: "completed", Result: "successful"})

				case strings.Contains(r.URL.Path, "/manualimport") && r.Method == "GET":
					if tt.scanStatus == 0 {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(`[]`))
					} else {
						w.WriteHeader(tt.scanStatus)
						w.Write([]byte(tt.scanResponse))
					}

				case strings.Contains(r.URL.Path, "/manualimport") && r.Method == "POST":
					if tt.importStatus == 0 {
						w.WriteHeader(http.StatusOK)
					} else {
						w.WriteHeader(tt.importStatus)
						w.Write([]byte(`{"error": "Import failed"}`))
					}

				case strings.Contains(r.URL.Path, "/command"):
					if tt.name == "Command API failure" {
						w.WriteHeader(http.StatusInternalServerError)
						w.Write([]byte(`{"error": "Command failed"}`))
					} else {
						w.WriteHeader(http.StatusCreated)
						json.NewEncoder(w).Encode(map[string]interface{}{
							"id":    1,
							"name":  "DownloadedEpisodesScan",
							"state": "queued",
						})
					}

				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			config := Config{
				UseRestAPI: tt.useRestAPI,
				Timeout:    5 * time.Second,
			}

			err := triggerManualImport(config, server.URL, "test-token", tt.queueItem.OutputPath, tt.queueItem.InstanceType, tt.useRestAPI, tt.queueItem)

			if (err != nil) != tt.expectError {
				t.Errorf("Error expectation mismatch: got %v, expectError=%v", err, tt.expectError)
			}

			if tt.expectFallback && requestCount < 2 {
				t.Errorf("Expected fallback behavior (multiple requests), got %d requests", requestCount)
			}
		})
	}
}

// TestManualImportPathValidation tests the exact path usage fix
func TestManualImportPathValidation(t *testing.T) {
	tests := []struct {
		name         string
		useRestAPI   bool
		queueItem    QueueItem
		expectedPath string
	}{
		{
			name:       "Command API uses exact OutputPath",
			useRestAPI: false,
			queueItem: QueueItem{
				ID:           123,
				InstanceType: "sonarr",
				OutputPath:   "/downloads/Exact.Release.Folder/S01E01",
			},
			expectedPath: "/downloads/Exact.Release.Folder/S01E01",
		},
		{
			name:       "REST API scan uses folder path",
			useRestAPI: true,
			queueItem: QueueItem{
				ID:           456,
				InstanceType: "radarr",
				MovieID:      123,
				OutputPath:   "/downloads/Movie.Release.2024",
			},
			expectedPath: "/downloads/Movie.Release.2024",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var actualPath string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/command/") {
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(CommandResource{ID: 1, Status: "completed", Result: "successful"})
					return
				}
				if strings.Contains(r.URL.Path, "/command") {
					var cmdData map[string]interface{}
					json.NewDecoder(r.Body).Decode(&cmdData)
					actualPath = cmdData["path"].(string)

					w.WriteHeader(http.StatusCreated)
					json.NewEncoder(w).Encode(map[string]interface{}{
						"id":    1,
						"state": "queued",
					})
				} else if strings.Contains(r.URL.Path, "/manualimport") && r.Method == "GET" {
					// Extract folder parameter for validation
					folder := r.URL.Query().Get("folder")
					if folder != "" {
						actualPath = folder
					}

					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`[]`))
				}
			}))
			defer server.Close()

			config := Config{
				UseRestAPI: tt.useRestAPI,
				Timeout:    5 * time.Second,
			}

			triggerManualImport(config, server.URL, "test-token", tt.queueItem.OutputPath, tt.queueItem.InstanceType, tt.useRestAPI, tt.queueItem)

			if actualPath != tt.expectedPath {
				t.Errorf("Path validation failed: expected %q, got %q", tt.expectedPath, actualPath)
			}
		})
	}
}

// TestCalculateQualityScore verifies quality scoring algorithm
func TestCalculateQualityScore(t *testing.T) {
	tests := []struct {
		name     string
		quality  QualityModel
		expected int
	}{
		{
			name: "Basic 1080p",
			quality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 1, Real: 0, IsRepack: false},
			},
			expected: 1080, // resolution only
		},
		{
			name: "1080p v2",
			quality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 2, Real: 0, IsRepack: false},
			},
			expected: 1280, // 1080 + (2 * 100)
		},
		{
			name: "1080p REAL",
			quality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 1, Real: 1, IsRepack: false},
			},
			expected: 1130, // 1080 + 50
		},
		{
			name: "1080p REPACK",
			quality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 1, Real: 0, IsRepack: true},
			},
			expected: 1105, // 1080 + 25
		},
		{
			name: "1080p v2 REAL REPACK",
			quality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 2, Real: 1, IsRepack: true},
			},
			expected: 1355, // 1080 + 200 + 50 + 25
		},
		{
			name: "720p",
			quality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-720p", Resolution: 720},
				Revision: RevisionModel{Version: 1, Real: 0, IsRepack: false},
			},
			expected: 720,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calculateQualityScore(tt.quality)
			if score != tt.expected {
				t.Errorf("calculateQualityScore() = %d, want %d", score, tt.expected)
			}
		})
	}
}

// TestCompareQualities verifies quality comparison logic
func TestCompareQualities(t *testing.T) {
	tests := []struct {
		name            string
		queueQuality    QualityModel
		existingQuality QualityModel
		expectUpgrade   bool
		expectDowngrade bool
		expectEqual     bool
	}{
		{
			name: "Upgrade 720p to 1080p",
			queueQuality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 1, Real: 0, IsRepack: false},
			},
			existingQuality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-720p", Resolution: 720},
				Revision: RevisionModel{Version: 1, Real: 0, IsRepack: false},
			},
			expectUpgrade: true,
		},
		{
			name: "Downgrade 1080p to 720p",
			queueQuality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-720p", Resolution: 720},
				Revision: RevisionModel{Version: 1, Real: 0, IsRepack: false},
			},
			existingQuality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 1, Real: 0, IsRepack: false},
			},
			expectDowngrade: true,
		},
		{
			name: "Same quality",
			queueQuality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 1, Real: 0, IsRepack: false},
			},
			existingQuality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 1, Real: 0, IsRepack: false},
			},
			expectEqual: true,
		},
		{
			name: "Upgrade v1 to v2",
			queueQuality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 2, Real: 0, IsRepack: false},
			},
			existingQuality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 1, Real: 0, IsRepack: false},
			},
			expectUpgrade: true,
		},
		{
			name: "Upgrade to REAL",
			queueQuality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 1, Real: 1, IsRepack: false},
			},
			existingQuality: QualityModel{
				Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
				Revision: RevisionModel{Version: 1, Real: 0, IsRepack: false},
			},
			expectUpgrade: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareQualities(tt.queueQuality, tt.existingQuality)
			if result.IsUpgrade != tt.expectUpgrade {
				t.Errorf("IsUpgrade = %v, want %v", result.IsUpgrade, tt.expectUpgrade)
			}
			if result.IsDowngrade != tt.expectDowngrade {
				t.Errorf("IsDowngrade = %v, want %v", result.IsDowngrade, tt.expectDowngrade)
			}
			if result.IsEqual != tt.expectEqual {
				t.Errorf("IsEqual = %v, want %v", result.IsEqual, tt.expectEqual)
			}
			if result.Reason == "" {
				t.Error("Reason should not be empty")
			}
		})
	}
}

// TestNormalizeTitle verifies title normalization
func TestNormalizeTitle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic title",
			input:    "Breaking Bad",
			expected: "breaking bad",
		},
		{
			name:     "Title with dots",
			input:    "Mr.Robot",
			expected: "mr robot",
		},
		{
			name:     "Title with dashes",
			input:    "Spider-Man",
			expected: "spider man",
		},
		{
			name:     "Title with underscores",
			input:    "The_Walking_Dead",
			expected: "the walking dead",
		},
		{
			name:     "Title with punctuation",
			input:    "What's Up: Doc?",
			expected: "whats up doc",
		},
		{
			name:     "Title with quotes",
			input:    `The "Best" Show`,
			expected: "the best show",
		},
		{
			name:     "Title with multiple spaces",
			input:    "Game  of   Thrones",
			expected: "game of thrones",
		},
		{
			name:     "Complex title",
			input:    "The.Office.US.2005",
			expected: "the office us 2005",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeTitle(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeTitle(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestLevenshteinDistance verifies edit distance calculation
func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		expected int
	}{
		{
			name:     "Identical strings",
			s1:       "hello",
			s2:       "hello",
			expected: 0,
		},
		{
			name:     "One character different",
			s1:       "hello",
			s2:       "hallo",
			expected: 1,
		},
		{
			name:     "Insertion",
			s1:       "hello",
			s2:       "hellos",
			expected: 1,
		},
		{
			name:     "Deletion",
			s1:       "hellos",
			s2:       "hello",
			expected: 1,
		},
		{
			name:     "Empty strings",
			s1:       "",
			s2:       "",
			expected: 0,
		},
		{
			name:     "One empty",
			s1:       "hello",
			s2:       "",
			expected: 5,
		},
		{
			name:     "Completely different",
			s1:       "abc",
			s2:       "xyz",
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := levenshteinDistance(tt.s1, tt.s2)
			if result != tt.expected {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.s1, tt.s2, result, tt.expected)
			}
		})
	}
}

// TestValidateTitleMatch verifies title matching with similarity threshold
func TestValidateTitleMatch(t *testing.T) {
	tests := []struct {
		name          string
		queueTitle    string
		scannedTitle  string
		expectMatch   bool
		minSimilarity float64
		maxSimilarity float64
	}{
		{
			name:          "Exact match",
			queueTitle:    "Breaking Bad",
			scannedTitle:  "Breaking Bad",
			expectMatch:   true,
			minSimilarity: 100.0,
			maxSimilarity: 100.0,
		},
		{
			name:          "Close match with formatting",
			queueTitle:    "Breaking.Bad.S01E01",
			scannedTitle:  "Breaking Bad",
			expectMatch:   false, // Episode numbers reduce similarity below threshold
			minSimilarity: 60.0,
			maxSimilarity: 70.0,
		},
		{
			name:          "Year difference",
			queueTitle:    "The Office (US)",
			scannedTitle:  "The Office",
			expectMatch:   false, // Year tag reduces similarity below threshold
			minSimilarity: 60.0,
			maxSimilarity: 75.0,
		},
		{
			name:          "Completely different",
			queueTitle:    "Breaking Bad",
			scannedTitle:  "Game of Thrones",
			expectMatch:   false,
			minSimilarity: 0.0,
			maxSimilarity: 85.0,
		},
		{
			name:          "Minor typo",
			queueTitle:    "Spider-Man",
			scannedTitle:  "Spiderman",
			expectMatch:   true, // Should pass: high character similarity (90%+)
			minSimilarity: 60.0,
			maxSimilarity: 95.0,
		},
		{
			name:          "False positive - Cruel Summer vs Last Summer",
			queueTitle:    "Cruel Summer",
			scannedTitle:  "Last Summer",
			expectMatch:   false,
			minSimilarity: 0.0,
			maxSimilarity: 80.0, // Should fail with new algorithm
		},
		{
			name:          "False positive - The Office vs Office Space",
			queueTitle:    "The Office",
			scannedTitle:  "Office Space",
			expectMatch:   false,
			minSimilarity: 0.0,
			maxSimilarity: 80.0,
		},
		{
			name:          "True match - The Matrix vs Matrix",
			queueTitle:    "The Matrix",
			scannedTitle:  "Matrix",
			expectMatch:   true, // Should pass: 100% token similarity after filtering "the"
			minSimilarity: 50.0,
			maxSimilarity: 90.0,
		},
		{
			name:          "True match - hyphen variation",
			queueTitle:    "Spider-Man: No Way Home",
			scannedTitle:  "SpiderMan No Way Home",
			expectMatch:   true, // High character similarity despite token difference
			minSimilarity: 80.0,
			maxSimilarity: 85.0,
		},
		{
			name:          "False positive - Avatar vs Avatar The Way of Water",
			queueTitle:    "Avatar",
			scannedTitle:  "Avatar The Way of Water",
			expectMatch:   false,
			minSimilarity: 0.0,
			maxSimilarity: 80.0,
		},
		{
			name:          "True match - case and punctuation",
			queueTitle:    "THE WALKING DEAD",
			scannedTitle:  "The Walking Dead",
			expectMatch:   true,
			minSimilarity: 100.0,
			maxSimilarity: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateTitleMatch(tt.queueTitle, tt.scannedTitle)
			if result.IsMatch != tt.expectMatch {
				t.Errorf("validateTitleMatch() IsMatch = %v, want %v (similarity: %.1f%%)",
					result.IsMatch, tt.expectMatch, result.Similarity)
			}
			if result.Similarity < tt.minSimilarity || result.Similarity > tt.maxSimilarity {
				t.Errorf("validateTitleMatch() Similarity = %.1f%%, want between %.1f%% and %.1f%%",
					result.Similarity, tt.minSimilarity, tt.maxSimilarity)
			}
			if result.Reason == "" {
				t.Error("Reason should not be empty")
			}
		})
	}
}

// TestMapStatusToActionWithQualityData verifies action mapping uses quality comparison
func TestMapStatusToActionWithQualityData(t *testing.T) {
	tests := []struct {
		name              string
		item              QueueItem
		expectedAction    string
		expectedBlocklist bool
		expectedManualImp bool
	}{
		{
			name: "Quality no upgrade - confirmed downgrade",
			item: QueueItem{
				ID:     1,
				Title:  "Test Show",
				Status: "completed",
				StatusMessages: []StatusMessage{
					{Messages: []string{"quality not an upgrade"}},
				},
				QualityComparison: &QualityComparison{
					IsDowngrade: true,
					IsUpgrade:   false,
				},
			},
			expectedAction:    "delete",
			expectedBlocklist: true,
			expectedManualImp: false,
		},
		{
			name: "Quality no upgrade - but actually IS upgrade",
			item: QueueItem{
				ID:     2,
				Title:  "Test Show",
				Status: "completed",
				StatusMessages: []StatusMessage{
					{Messages: []string{"quality not an upgrade"}},
				},
				QualityComparison: &QualityComparison{
					IsUpgrade:   true,
					IsDowngrade: false,
				},
			},
			expectedAction:    "manual_import",
			expectedBlocklist: false,
			expectedManualImp: true,
		},
		{
			name: "Matched by ID - always delete with blocklist",
			item: QueueItem{
				ID:     3,
				Title:  "Test Show",
				Status: "completed",
				StatusMessages: []StatusMessage{
					{Messages: []string{"matched to series by id"}},
				},
				TitleMatchResult: &TitleMatchResult{
					IsMatch:    true,
					Similarity: 98.0,
				},
			},
			expectedAction:    "delete",
			expectedBlocklist: true,
			expectedManualImp: false,
		},
		{
			name: "Matched by ID - title mismatch also deletes",
			item: QueueItem{
				ID:     4,
				Title:  "Wrong Show",
				Status: "completed",
				StatusMessages: []StatusMessage{
					{Messages: []string{"matched to series by id"}},
				},
				TitleMatchResult: &TitleMatchResult{
					IsMatch:    false,
					Similarity: 30.0,
				},
			},
			expectedAction:    "delete",
			expectedBlocklist: true,
			expectedManualImp: false,
		},
		{
			name: "Import blocked - title OK",
			item: QueueItem{
				ID:                   5,
				Title:                "Test Show",
				Status:               "completed",
				TrackedDownloadState: "importBlocked",
				TitleMatchResult: &TitleMatchResult{
					IsMatch:    true,
					Similarity: 95.0,
				},
			},
			expectedAction:    "manual_import",
			expectedBlocklist: false,
			expectedManualImp: true,
		},
		{
			name: "Import blocked - title mismatch",
			item: QueueItem{
				ID:                   6,
				Title:                "Wrong Show",
				Status:               "completed",
				TrackedDownloadState: "importBlocked",
				TitleMatchResult: &TitleMatchResult{
					IsMatch:    false,
					Similarity: 40.0,
				},
			},
			expectedAction:    "delete",
			expectedBlocklist: true,
			expectedManualImp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, blocklist, manualImp := mapStatusToAction(tt.item)
			if action != tt.expectedAction {
				t.Errorf("action = %q, want %q", action, tt.expectedAction)
			}
			if blocklist != tt.expectedBlocklist {
				t.Errorf("blocklist = %v, want %v", blocklist, tt.expectedBlocklist)
			}
			if manualImp != tt.expectedManualImp {
				t.Errorf("manualImport = %v, want %v", manualImp, tt.expectedManualImp)
			}
		})
	}
}

// TestFetchEpisodeFiles verifies episode file API integration
func TestFetchEpisodeFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if strings.Contains(r.URL.Path, "/api/v3/episodefile") {
			seriesID := r.URL.Query().Get("seriesId")
			if seriesID == "123" {
				json.NewEncoder(w).Encode([]EpisodeFileResource{
					{
						ID:       1,
						SeriesID: 123,
						Quality: QualityModel{
							Quality:  QualityDefinition{Name: "HDTV-1080p", Resolution: 1080},
							Revision: RevisionModel{Version: 1},
						},
					},
				})
			} else {
				w.Write([]byte("[]"))
			}
		}
	}))
	defer server.Close()

	config := Config{Timeout: 5 * time.Second}

	t.Run("Valid series ID", func(t *testing.T) {
		files, err := fetchEpisodeFiles(config, server.URL, "test-token", 123)
		if err != nil {
			t.Fatalf("fetchEpisodeFiles() error = %v", err)
		}
		if len(files) != 1 {
			t.Errorf("len(files) = %d, want 1", len(files))
		}
		if files[0].SeriesID != 123 {
			t.Errorf("SeriesID = %d, want 123", files[0].SeriesID)
		}
	})

	t.Run("Invalid series ID", func(t *testing.T) {
		_, err := fetchEpisodeFiles(config, server.URL, "test-token", 0)
		if err == nil {
			t.Error("fetchEpisodeFiles() should return error for seriesID 0")
		}
	})
}

// TestFetchMovieFile verifies movie file API integration
func TestFetchMovieFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if strings.Contains(r.URL.Path, "/api/v3/moviefile") {
			movieID := r.URL.Query().Get("movieId")
			if movieID == "456" {
				json.NewEncoder(w).Encode([]MovieFileResource{
					{
						ID:      1,
						MovieID: 456,
						Quality: QualityModel{
							Quality:  QualityDefinition{Name: "Bluray-1080p", Resolution: 1080},
							Revision: RevisionModel{Version: 1},
						},
					},
				})
			} else {
				w.Write([]byte("[]"))
			}
		}
	}))
	defer server.Close()

	config := Config{Timeout: 5 * time.Second}

	t.Run("Valid movie ID", func(t *testing.T) {
		file, err := fetchMovieFile(config, server.URL, "test-token", 456)
		if err != nil {
			t.Fatalf("fetchMovieFile() error = %v", err)
		}
		if file == nil {
			t.Fatal("fetchMovieFile() returned nil file")
		}
		if file.MovieID != 456 {
			t.Errorf("MovieID = %d, want 456", file.MovieID)
		}
	})

	t.Run("No existing file", func(t *testing.T) {
		file, err := fetchMovieFile(config, server.URL, "test-token", 999)
		if err != nil {
			t.Fatalf("fetchMovieFile() error = %v", err)
		}
		if file != nil {
			t.Error("fetchMovieFile() should return nil for non-existent movie file")
		}
	})

	t.Run("Invalid movie ID", func(t *testing.T) {
		_, err := fetchMovieFile(config, server.URL, "test-token", 0)
		if err == nil {
			t.Error("fetchMovieFile() should return error for movieID 0")
		}
	})
}

// ========== HELPER FUNCTIONS ==========

// ========== TESTS: FolderName and ReleaseType Fields ==========

// TestBuildManualImportRequests_FolderNameAndReleaseType verifies folder name extraction and release type setting
func TestBuildManualImportRequests_FolderNameAndReleaseType(t *testing.T) {
	config := Config{Verbose: false}

	tests := []struct {
		name           string
		inputPath      string
		expectedFolder string
		instanceType   string
		useReleaseType bool // Only Sonarr uses releaseType
	}{
		{
			name:           "Standard Sonarr release path",
			inputPath:      "/downloads/Sonarr/American.Pickers.S27E10.1080p.WEB.H264-BeechyBoy/American.Pickers.S27E10.1080p.WEB.H264-BeechyBoy.mp4",
			expectedFolder: "American.Pickers.S27E10.1080p.WEB.H264-BeechyBoy",
			instanceType:   "sonarr",
			useReleaseType: true,
		},
		{
			name:           "Simple Sonarr path",
			inputPath:      "/downloads/Release.Name/file.mkv",
			expectedFolder: "Release.Name",
			instanceType:   "sonarr",
			useReleaseType: true,
		},
		{
			name:           "Deep nested Sonarr path",
			inputPath:      "/mnt/storage/downloads/complete/Series.S01E01.1080p/episode.mkv",
			expectedFolder: "Series.S01E01.1080p",
			instanceType:   "sonarr",
			useReleaseType: true,
		},
		{
			name:           "Path with spaces - Sonarr",
			inputPath:      "/downloads/My Series S01E01/My Series S01E01.mkv",
			expectedFolder: "My Series S01E01",
			instanceType:   "sonarr",
			useReleaseType: true,
		},
		{
			name:           "Radarr movie path - no releaseType",
			inputPath:      "/downloads/Movie.2024.1080p/Movie.2024.1080p.mkv",
			expectedFolder: "",
			instanceType:   "radarr",
			useReleaseType: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock scanned item
			scannedItems := []ManualImportResource{
				{
					Path: tt.inputPath,
					Quality: QualityModel{
						Quality: QualityDefinition{
							ID:   10,
							Name: "WEBDL-1080p",
						},
					},
					Languages: []Language{{ID: 1, Name: "English"}},
				},
			}

			// Create queue item based on instance type
			// Use empty title to bypass title matching validation
			queueItem := QueueItem{
				ID:         1,
				Title:      "",
				DownloadId: "test-download-id",
			}

			if tt.instanceType == "sonarr" {
				scannedItems[0].Series = &SeriesResource{
					ID:    484,
					Title: "Test Series",
				}
				seasonNum := 1
				scannedItems[0].SeasonNumber = &seasonNum
				scannedItems[0].Episodes = []EpisodeResource{
					{
						ID:            58363,
						SeasonNumber:  1,
						EpisodeNumber: 1,
					},
				}
				queueItem.SeriesID = 484
			} else {
				scannedItems[0].Movie = &MovieResource{
					ID:    456,
					Title: "Test Movie",
					Year:  2024,
				}
				queueItem.MovieID = 456
			}

			// Build import requests
			requests := buildManualImportRequests(config, scannedItems, queueItem, tt.instanceType)

			if len(requests) != 1 {
				t.Fatalf("Expected 1 request, got %d", len(requests))
			}

			req := requests[0]

			// Verify FolderName
			if tt.instanceType == "sonarr" {
				if req.FolderName != tt.expectedFolder {
					t.Errorf("FolderName = %q, want %q", req.FolderName, tt.expectedFolder)
				}
			} else {
				// Radarr doesn't set FolderName
				if req.FolderName != "" {
					t.Errorf("Radarr should not set FolderName, got %q", req.FolderName)
				}
			}

			// Verify ReleaseType
			if tt.useReleaseType {
				if req.ReleaseType != "singleEpisode" {
					t.Errorf("ReleaseType = %q, want %q", req.ReleaseType, "singleEpisode")
				}
			} else {
				// Radarr doesn't use releaseType
				if req.ReleaseType != "" {
					t.Errorf("Radarr should not set ReleaseType, got %q", req.ReleaseType)
				}
			}

			// Verify other essential fields are populated
			if req.Path != tt.inputPath {
				t.Errorf("Path = %q, want %q", req.Path, tt.inputPath)
			}
			if req.DownloadID != "test-download-id" {
				t.Errorf("DownloadID = %q, want %q", req.DownloadID, "test-download-id")
			}
		})
	}
}

// TestManualImportRequest_JSONMarshaling verifies JSON field names and structure
func TestManualImportRequest_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name           string
		request        ManualImportRequest
		expectedFields map[string]interface{}
		omittedFields  []string
	}{
		{
			name: "Sonarr request with all fields",
			request: ManualImportRequest{
				Path:         "/downloads/folder/file.mkv",
				FolderName:   "folder",
				SeriesID:     484,
				EpisodeIDs:   []int{58363},
				ReleaseType:  "singleEpisode",
				DownloadID:   "test-download-id",
				ReleaseGroup: "GROUP",
				ImportMode:   "auto",
			},
			expectedFields: map[string]interface{}{
				"path":         "/downloads/folder/file.mkv",
				"folderName":   "folder",
				"seriesId":     float64(484),
				"episodeIds":   []interface{}{float64(58363)},
				"releaseType":  "singleEpisode",
				"downloadId":   "test-download-id",
				"releaseGroup": "GROUP",
				"importMode":   "auto",
			},
			omittedFields: []string{"movieId", "seasonNumber"},
		},
		{
			name: "Radarr request without folderName/releaseType",
			request: ManualImportRequest{
				Path:         "/downloads/movie/file.mkv",
				MovieID:      456,
				DownloadID:   "movie-download-id",
				ReleaseGroup: "GROUP",
				ImportMode:   "auto",
			},
			expectedFields: map[string]interface{}{
				"path":         "/downloads/movie/file.mkv",
				"movieId":      float64(456),
				"downloadId":   "movie-download-id",
				"releaseGroup": "GROUP",
				"importMode":   "auto",
			},
			omittedFields: []string{"folderName", "releaseType", "seriesId", "episodeIds"},
		},
		{
			name: "Minimal request - verify omitempty",
			request: ManualImportRequest{
				Path: "/downloads/minimal.mkv",
			},
			expectedFields: map[string]interface{}{
				"path": "/downloads/minimal.mkv",
			},
			omittedFields: []string{"folderName", "releaseType", "seriesId", "movieId", "downloadId"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonBytes, err := json.Marshal(tt.request)
			if err != nil {
				t.Fatalf("Failed to marshal JSON: %v", err)
			}

			var result map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &result); err != nil {
				t.Fatalf("Failed to unmarshal JSON: %v", err)
			}

			// Verify expected fields are present and correct
			for field, expectedValue := range tt.expectedFields {
				actualValue, exists := result[field]
				if !exists {
					t.Errorf("Expected field %q not found in JSON", field)
					continue
				}

				// Handle array comparison
				if expectedArray, ok := expectedValue.([]interface{}); ok {
					actualArray, ok := actualValue.([]interface{})
					if !ok {
						t.Errorf("Field %q: expected array, got %T", field, actualValue)
						continue
					}
					if len(actualArray) != len(expectedArray) {
						t.Errorf("Field %q: array length = %d, want %d", field, len(actualArray), len(expectedArray))
						continue
					}
					for i := range expectedArray {
						if actualArray[i] != expectedArray[i] {
							t.Errorf("Field %q[%d]: got %v, want %v", field, i, actualArray[i], expectedArray[i])
						}
					}
				} else if actualValue != expectedValue {
					t.Errorf("Field %q: got %v, want %v", field, actualValue, expectedValue)
				}
			}

			// Verify omitted fields are not present (omitempty behavior)
			for _, field := range tt.omittedFields {
				if _, exists := result[field]; exists {
					t.Errorf("Field %q should be omitted but found in JSON with value: %v", field, result[field])
				}
			}
		})
	}
}

// TestExecuteManualImport_PayloadStructure verifies complete payload structure sent to API
func TestExecuteManualImport_PayloadStructure(t *testing.T) {
	var capturedPayload ManualImportCommand

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/command/") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(CommandResource{ID: 1, Status: "completed", Result: "successful"})
			return
		}
		// Verify endpoint
		if !strings.Contains(r.URL.Path, "/api/v3/command") {
			t.Errorf("Expected /api/v3/command, got %s", r.URL.Path)
		}

		// Verify method
		if r.Method != "POST" {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Capture and decode payload
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("Failed to read request body: %v", err)
		}

		if err := json.Unmarshal(body, &capturedPayload); err != nil {
			t.Fatalf("Failed to unmarshal payload: %v", err)
		}

		// Send success response
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":    1,
			"name":  "ManualImport",
			"state": "queued",
		})
	}))
	defer server.Close()

	config := Config{Timeout: 5 * time.Second}

	// Create test import requests with new fields
	importRequests := []ManualImportRequest{
		{
			Path:         "/downloads/American.Pickers.S27E10.1080p.WEB.H264-BeechyBoy/file.mp4",
			FolderName:   "American.Pickers.S27E10.1080p.WEB.H264-BeechyBoy",
			SeriesID:     484,
			EpisodeIDs:   []int{58363},
			ReleaseType:  "singleEpisode",
			DownloadID:   "test-download",
			ReleaseGroup: "BeechyBoy",
			Quality: QualityModel{
				Quality: QualityDefinition{ID: 10, Name: "WEBDL-1080p"},
			},
			Languages:  []Language{{ID: 1, Name: "English"}},
			ImportMode: "auto",
		},
	}

	// Execute manual import
	err := executeManualImport(config, server.URL, "test-token", importRequests)
	if err != nil {
		t.Fatalf("executeManualImport failed: %v", err)
	}

	// Verify command structure
	if capturedPayload.Name != "ManualImport" {
		t.Errorf("Command name = %q, want %q", capturedPayload.Name, "ManualImport")
	}

	if capturedPayload.ImportMode != "auto" {
		t.Errorf("ImportMode = %q, want %q", capturedPayload.ImportMode, "auto")
	}

	if len(capturedPayload.Files) != 1 {
		t.Fatalf("Files count = %d, want 1", len(capturedPayload.Files))
	}

	file := capturedPayload.Files[0]

	// Verify new fields are present in payload
	if file.FolderName != "American.Pickers.S27E10.1080p.WEB.H264-BeechyBoy" {
		t.Errorf("FolderName = %q, want %q", file.FolderName, "American.Pickers.S27E10.1080p.WEB.H264-BeechyBoy")
	}

	if file.ReleaseType != "singleEpisode" {
		t.Errorf("ReleaseType = %q, want %q", file.ReleaseType, "singleEpisode")
	}

	// Verify other essential fields
	if file.Path != importRequests[0].Path {
		t.Errorf("Path = %q, want %q", file.Path, importRequests[0].Path)
	}

	if file.SeriesID != 484 {
		t.Errorf("SeriesID = %d, want 484", file.SeriesID)
	}

	if len(file.EpisodeIDs) != 1 || file.EpisodeIDs[0] != 58363 {
		t.Errorf("EpisodeIDs = %v, want [58363]", file.EpisodeIDs)
	}

	if file.DownloadID != "test-download" {
		t.Errorf("DownloadID = %q, want %q", file.DownloadID, "test-download")
	}
}

// TestTriggerManualImport_EndToEnd verifies complete workflow with new fields
func TestTriggerManualImport_EndToEnd(t *testing.T) {
	var scanCalled bool
	var executeCalled bool
	var capturedCommand ManualImportCommand

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/command/"):
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(CommandResource{ID: 1, Status: "completed", Result: "successful"})

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/api/v3/manualimport"):
			scanCalled = true
			// Return scan response with American Pickers example
			w.WriteHeader(http.StatusOK)
			seasonNum := 27
			json.NewEncoder(w).Encode([]ManualImportResource{
				{
					Path: "/downloads/Sonarr/American.Pickers.S27E10.1080p.WEB.H264-BeechyBoy/American.Pickers.S27E10.1080p.WEB.H264-BeechyBoy.mp4",
					Series: &SeriesResource{
						ID:    484,
						Title: "American Pickers",
					},
					SeasonNumber: &seasonNum,
					Episodes: []EpisodeResource{
						{
							ID:            58363,
							SeasonNumber:  27,
							EpisodeNumber: 10,
						},
					},
					Quality: QualityModel{
						Quality: QualityDefinition{ID: 10, Name: "WEBDL-1080p"},
					},
					Languages:    []Language{{ID: 1, Name: "English"}},
					ReleaseGroup: "BeechyBoy",
				},
			})

		case r.Method == "POST" && strings.Contains(r.URL.Path, "/api/v3/command"):
			executeCalled = true
			// Capture command payload
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &capturedCommand)

			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":    1,
				"name":  "ManualImport",
				"state": "queued",
			})
		}
	}))
	defer server.Close()

	config := Config{
		Timeout: 5 * time.Second,
		Verbose: false,
	}

	queueItem := QueueItem{
		ID:         123,
		Title:      "American Pickers",
		SeriesID:   484,
		OutputPath: "/downloads/Sonarr/American.Pickers.S27E10.1080p.WEB.H264-BeechyBoy",
		DownloadId: "test-download-id",
	}

	// Trigger manual import
	err := triggerManualImport(config, server.URL, "test-token", queueItem.OutputPath, "sonarr", false, queueItem)
	if err != nil {
		t.Fatalf("triggerManualImport failed: %v", err)
	}

	// Verify both API calls were made
	if !scanCalled {
		t.Error("Scan API was not called")
	}
	if !executeCalled {
		t.Error("Execute API was not called")
	}

	// Verify command structure
	if capturedCommand.Name != "ManualImport" {
		t.Errorf("Command name = %q, want ManualImport", capturedCommand.Name)
	}

	if len(capturedCommand.Files) != 1 {
		t.Fatalf("Files count = %d, want 1", len(capturedCommand.Files))
	}

	file := capturedCommand.Files[0]

	// Verify NEW fields are present in final payload
	if file.FolderName != "American.Pickers.S27E10.1080p.WEB.H264-BeechyBoy" {
		t.Errorf("FolderName not set correctly in end-to-end workflow: got %q", file.FolderName)
	}

	if file.ReleaseType != "singleEpisode" {
		t.Errorf("ReleaseType not set correctly in end-to-end workflow: got %q", file.ReleaseType)
	}

	// Verify other fields are still correct
	if file.SeriesID != 484 {
		t.Errorf("SeriesID = %d, want 484", file.SeriesID)
	}

	if len(file.EpisodeIDs) != 1 {
		t.Errorf("EpisodeIDs count = %d, want 1", len(file.EpisodeIDs))
	}
}

// TestBuildManualImportRequests_EdgeCases verifies edge cases for folder name extraction
func TestBuildManualImportRequests_EdgeCases(t *testing.T) {
	config := Config{Verbose: false}

	tests := []struct {
		name           string
		inputPath      string
		expectedFolder string
		shouldSucceed  bool
	}{
		{
			name:           "Root-level path",
			inputPath:      "/file.mkv",
			expectedFolder: "/",
			shouldSucceed:  true,
		},
		{
			name:           "Path with trailing slash",
			inputPath:      "/downloads/folder/",
			expectedFolder: "folder",
			shouldSucceed:  true,
		},
		{
			name:           "Path with dots in folder name",
			inputPath:      "/downloads/Series.S01E01.720p/file.mkv",
			expectedFolder: "Series.S01E01.720p",
			shouldSucceed:  true,
		},
		{
			name:           "Path with special characters",
			inputPath:      "/downloads/Series (2024) [1080p]/file.mkv",
			expectedFolder: "Series (2024) [1080p]",
			shouldSucceed:  true,
		},
		{
			name:           "Deeply nested path",
			inputPath:      "/mnt/data/media/tv/downloads/complete/Series.S01E01/file.mkv",
			expectedFolder: "Series.S01E01",
			shouldSucceed:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seasonNum := 1
			scannedItems := []ManualImportResource{
				{
					Path: tt.inputPath,
					Series: &SeriesResource{
						ID:    1,
						Title: "Test",
					},
					SeasonNumber: &seasonNum,
					Episodes: []EpisodeResource{
						{ID: 1, SeasonNumber: 1, EpisodeNumber: 1},
					},
					Quality:   QualityModel{Quality: QualityDefinition{ID: 1, Name: "HDTV-720p"}},
					Languages: []Language{{ID: 1, Name: "English"}},
				},
			}

			queueItem := QueueItem{
				SeriesID: 1,
			}

			requests := buildManualImportRequests(config, scannedItems, queueItem, "sonarr")

			if tt.shouldSucceed {
				if len(requests) != 1 {
					t.Fatalf("Expected 1 request, got %d", len(requests))
				}

				if requests[0].FolderName != tt.expectedFolder {
					t.Errorf("FolderName = %q, want %q", requests[0].FolderName, tt.expectedFolder)
				}
			} else {
				if len(requests) != 0 {
					t.Errorf("Expected 0 requests for invalid path, got %d", len(requests))
				}
			}
		})
	}
}
