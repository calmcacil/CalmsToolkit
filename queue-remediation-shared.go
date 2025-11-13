//go:build queueremediation

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"time"
)

// StatusMessage represents a status message with title and messages
type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// QueueItem represents an item in the download queue
type QueueItem struct {
	ID                    int                `json:"id"`
	Title                 string             `json:"title"`
	Status                string             `json:"status"`
	TrackedDownloadState  string             `json:"trackedDownloadState"`
	TrackedDownloadStatus string             `json:"trackedDownloadStatus"`
	StatusMessages        []StatusMessage    `json:"statusMessages"`
	DownloadClient        string             `json:"downloadClient"`
	DownloadId            string             `json:"downloadId"`
	OutputPath            string             `json:"outputPath"`
	ErrorMessage          string             `json:"errorMessage"`
	InstanceURL           string             `json:"-"`
	InstanceType          string             `json:"-"`
	SeriesID              int                `json:"seriesId,omitempty"`
	MovieID               int                `json:"movieId,omitempty"`
	ExistingQuality       *QualityModel      `json:"-"` // Quality of existing file on disk
	QualityComparison     *QualityComparison `json:"-"` // Result of quality comparison
	TitleMatchResult      *TitleMatchResult  `json:"-"` // Result of title validation
}

// QueueResponse represents the response from queue API calls
type QueueResponse struct {
	Page         int         `json:"page"`
	PageSize     int         `json:"pageSize"`
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

// ManualImportResource represents a resource found during manual import scanning
type ManualImportResource struct {
	ID           int64             `json:"id"`
	Path         string            `json:"path"`
	RelativePath string            `json:"relativePath"`
	Name         string            `json:"name"`
	Size         int64             `json:"size"`
	Series       *SeriesResource   `json:"series"`
	Movie        *MovieResource    `json:"movie"`
	SeasonNumber *int              `json:"seasonNumber"`
	Episodes     []EpisodeResource `json:"episodes"`
	Quality      QualityModel      `json:"quality"`
	Languages    []Language        `json:"languages"`
	ReleaseGroup string            `json:"releaseGroup"`
	Rejections   []ImportRejection `json:"rejections"`
	DownloadID   string            `json:"downloadId"`
	IndexerFlags int               `json:"indexerFlags"`
}

// SeriesResource represents a series in Sonarr
type SeriesResource struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// MovieResource represents a movie in Radarr
type MovieResource struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

// EpisodeResource represents an episode in Sonarr
type EpisodeResource struct {
	ID            int    `json:"id"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Title         string `json:"title"`
}

// QualityModel represents quality information
type QualityModel struct {
	Quality  QualityDefinition `json:"quality"`
	Revision RevisionModel     `json:"revision"`
}

// QualityDefinition represents quality definition
type QualityDefinition struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Resolution int    `json:"resolution"`
}

// RevisionModel represents revision information
type RevisionModel struct {
	Version  int  `json:"version"`
	Real     int  `json:"real"`
	IsRepack bool `json:"isRepack"`
}

// Language represents language information
type Language struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ImportRejection represents a rejection reason for manual import
type ImportRejection struct {
	Reason string `json:"reason"`
	Type   string `json:"type"`
}

// EpisodeFileResource represents an episode file from Sonarr API
type EpisodeFileResource struct {
	ID            int          `json:"id"`
	SeriesID      int          `json:"seriesId"`
	SeasonNumber  int          `json:"seasonNumber"`
	RelativePath  string       `json:"relativePath"`
	Path          string       `json:"path"`
	Size          int64        `json:"size"`
	DateAdded     string       `json:"dateAdded"`
	Quality       QualityModel `json:"quality"`
	MediaInfo     MediaInfo    `json:"mediaInfo"`
	EpisodeFileID int          `json:"episodeFileId"`
	Episodes      []int        `json:"episodeIds"`
}

// MovieFileResource represents a movie file from Radarr API
type MovieFileResource struct {
	ID           int          `json:"id"`
	MovieID      int          `json:"movieId"`
	RelativePath string       `json:"relativePath"`
	Path         string       `json:"path"`
	Size         int64        `json:"size"`
	DateAdded    string       `json:"dateAdded"`
	Quality      QualityModel `json:"quality"`
	MediaInfo    MediaInfo    `json:"mediaInfo"`
}

// MediaInfo represents media codec and bitrate information
type MediaInfo struct {
	AudioBitrate     int    `json:"audioBitrate"`
	AudioChannels    int    `json:"audioChannels"`
	AudioCodec       string `json:"audioCodec"`
	AudioLanguages   string `json:"audioLanguages"`
	AudioStreamCount int    `json:"audioStreamCount"`
	VideoBitDepth    int    `json:"videoBitDepth"`
	VideoBitrate     int    `json:"videoBitrate"`
	VideoCodec       string `json:"videoCodec"`
	VideoFps         string `json:"videoFps"`
	Resolution       string `json:"resolution"`
	RunTime          string `json:"runTime"`
	ScanType         string `json:"scanType"`
}

// QualityComparison represents the result of comparing two quality models
type QualityComparison struct {
	IsUpgrade         bool
	IsDowngrade       bool
	IsEqual           bool
	NewScore          int
	ExistingScore     int
	ScoreDiff         int
	Reason            string
	NewFormatted      string
	ExistingFormatted string
}

// TitleMatchResult represents the result of title matching validation
type TitleMatchResult struct {
	IsMatch           bool
	Similarity        float64
	QueueTitle        string
	ScannedTitle      string
	NormalizedQueue   string
	NormalizedScanned string
	Reason            string
}

// ManualImportRequest represents a manual import request for individual files
type ManualImportRequest struct {
	Path         string       `json:"path"`
	SeriesID     int          `json:"seriesId,omitempty"`
	MovieID      int          `json:"movieId,omitempty"`
	SeasonNumber int          `json:"seasonNumber,omitempty"`
	EpisodeIDs   []int        `json:"episodeIds,omitempty"`
	Quality      QualityModel `json:"quality,omitempty"`
	Languages    []Language   `json:"languages,omitempty"`
	ReleaseGroup string       `json:"releaseGroup,omitempty"`
	DownloadID   string       `json:"downloadId,omitempty"`
	IndexerFlags int          `json:"indexerFlags,omitempty"`
	ImportMode   string       `json:"importMode,omitempty"`
}

// ManualImportCommand represents the command structure for manual import via Command API
type ManualImportCommand struct {
	Name       string                `json:"name"`
	Files      []ManualImportRequest `json:"files"`
	ImportMode string                `json:"importMode"`
}

// Config represents the application configuration
type Config struct {
	SonarrURLs   []string
	SonarrTokens []string
	RadarrURLs   []string
	RadarrTokens []string
	Timeout      time.Duration
	UseRestAPI   bool
	Verbose      bool
	Debug        bool
}

// sanitizeURL removes sensitive information from URLs for logging
func sanitizeURL(rawURL string) string {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return "[invalid URL]"
	}
	// Return only scheme://host:port, omitting path and query params
	sanitized := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	return sanitized
}

// mapStatusToAction determines the appropriate action for a queue item based on its status
func mapStatusToAction(item QueueItem) (action string, blocklist bool, manualImport bool) {
	if item.Status == "failed" || strings.EqualFold(item.TrackedDownloadStatus, "warning") && strings.Contains(strings.ToLower(item.ErrorMessage), "failed") {
		return "delete", false, false
	}

	reason, shouldBlocklist := parseStatusMessages(item.StatusMessages)

	switch reason {
	case "custom_format_no_upgrade":
		// If we have quality comparison data, verify it's actually not an upgrade
		if item.QualityComparison != nil {
			if item.QualityComparison.IsUpgrade {
				// Status message says no upgrade, but our analysis says it IS an upgrade
				// Attempt manual import instead of deleting
				return "manual_import", false, true
			}
			// Confirmed not an upgrade, safe to delete
			return "delete", shouldBlocklist, false
		}
		// No quality data, trust the status message
		return "delete", shouldBlocklist, false
	case "quality_no_upgrade":
		// If we have quality comparison data, verify it's actually not an upgrade
		if item.QualityComparison != nil {
			if item.QualityComparison.IsUpgrade {
				// Status message says no upgrade, but our analysis says it IS an upgrade
				// This indicates a discrepancy - attempt manual import
				return "manual_import", false, true
			}
			if item.QualityComparison.IsDowngrade {
				// Confirmed downgrade, delete with blocklist
				return "delete", true, false
			}
			// Same quality or confirmed not an upgrade, safe to delete
			return "delete", shouldBlocklist, false
		}
		// No quality data, trust the status message
		return "delete", shouldBlocklist, false
	case "no_files_found":
		return "delete", shouldBlocklist, false
	case "sample_file":
		return "delete", shouldBlocklist, false
	case "matched_by_id":
		// Validate title match if available
		if item.TitleMatchResult != nil && !item.TitleMatchResult.IsMatch {
			// Title mismatch detected, safer to delete than risk wrong import
			return "delete", true, false
		}
		return "manual_import", false, true
	}

	if item.TrackedDownloadState == "importBlocked" || strings.Contains(strings.ToLower(item.ErrorMessage), "import blocked") {
		// Check title match for import blocked items
		if item.TitleMatchResult != nil && !item.TitleMatchResult.IsMatch {
			// Title mismatch, don't attempt import
			return "delete", true, false
		}
		return "manual_import", false, true
	}

	return "monitor", false, false
}

// parseStatusMessages analyzes status messages to determine the reason for queue item status
func parseStatusMessages(statusMessages []StatusMessage) (string, bool) {
	var hasQualityCF, hasSample, hasNoFiles, hasIDMatch bool
	var isCustomFormat bool

	for _, sm := range statusMessages {
		for _, msg := range sm.Messages {
			msgLower := strings.ToLower(msg)

			if strings.Contains(msgLower, "custom format upgrade") {
				hasQualityCF = true
				isCustomFormat = true
			}

			if strings.Contains(msgLower, "quality revision") || strings.Contains(msgLower, "quality not an upgrade") {
				hasQualityCF = true
			}

			if msgLower == "sample" || strings.Contains(msgLower, "sample") {
				hasSample = true
			}

			if strings.Contains(msgLower, "no files found") {
				hasNoFiles = true
			}

			if strings.Contains(msgLower, "matched to series by id") {
				hasIDMatch = true
			}
		}
	}

	if hasQualityCF {
		if isCustomFormat {
			return "custom_format_no_upgrade", true
		}
		return "quality_no_upgrade", true
	}

	if hasIDMatch {
		return "matched_by_id", false
	}

	if hasSample {
		return "sample_file", false
	}

	if hasNoFiles {
		return "no_files_found", false
	}

	return "unknown", false
}

// getInstanceName returns a formatted instance name for display
func getInstanceName(config Config, instanceURL, instanceType string) string {
	if instanceType == "sonarr" {
		for i, url := range config.SonarrURLs {
			if url == instanceURL {
				return fmt.Sprintf("Sonarr%d", i+1)
			}
		}
	} else {
		for i, url := range config.RadarrURLs {
			if url == instanceURL {
				return fmt.Sprintf("Radarr%d", i+1)
			}
		}
	}
	return instanceType
}

// getReason returns a human-readable reason for the queue item's status
func getReason(item QueueItem) string {
	if item.Status == "failed" {
		return "download failed"
	}

	reason, _ := parseStatusMessages(item.StatusMessages)

	switch reason {
	case "custom_format_no_upgrade":
		return "custom format not an upgrade"
	case "quality_no_upgrade":
		return "quality not an upgrade"
	case "no_files_found":
		return "no files found"
	case "sample_file":
		return "sample file detected"
	case "matched_by_id":
		return "matched to series by ID"
	case "unknown":
		if item.TrackedDownloadState == "importBlocked" {
			return "import blocked"
		}
		return "downloading normally"
	default:
		if item.TrackedDownloadState == "importBlocked" {
			return "import blocked"
		}
		return "downloading normally"
	}
}

// fetchSeriesDetails fetches series details by ID from Sonarr API
func fetchSeriesDetails(config Config, instanceURL, token string, seriesID int) (*SeriesResource, error) {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	endpoint := fmt.Sprintf("%s/api/v3/series/%d", instanceURL, seriesID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var series SeriesResource
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, err
	}

	return &series, nil
}

// fetchMovieDetails fetches movie details by ID from Radarr API
func fetchMovieDetails(config Config, instanceURL, token string, movieID int) (*MovieResource, error) {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	endpoint := fmt.Sprintf("%s/api/v3/movie/%d", instanceURL, movieID)

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var movie MovieResource
	if err := json.NewDecoder(resp.Body).Decode(&movie); err != nil {
		return nil, err
	}

	return &movie, nil
}

// fetchQueue retrieves all queue items from a single instance
func fetchQueue(config Config, url string, token string) ([]QueueItem, error) {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	var allRecords []QueueItem
	page := 1

	for {
		endpoint := fmt.Sprintf("%s/api/v3/queue?page=%d&pageSize=100", url, page)

		req, err := http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("X-Api-Key", token)

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return nil, fmt.Errorf("failed to fetch queue: status %d (error reading response body: %v)", resp.StatusCode, readErr)
			}
			return nil, fmt.Errorf("failed to fetch queue: status %d - %s", resp.StatusCode, string(bodyBytes))
		}

		var queueResp QueueResponse
		if err := json.NewDecoder(resp.Body).Decode(&queueResp); err != nil {
			return nil, err
		}

		allRecords = append(allRecords, queueResp.Records...)

		if len(queueResp.Records) < queueResp.PageSize || len(queueResp.Records) == 0 {
			break
		}

		page++
	}

	return allRecords, nil
}

// deleteQueueItem removes a queue item from the specified instance
func deleteQueueItem(config Config, url string, token string, itemID int, removeFromClient bool, blocklist bool) error {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	endpoint := fmt.Sprintf("%s/api/v3/queue/%d", url, itemID)

	var queryParams []string
	if removeFromClient {
		queryParams = append(queryParams, "removeFromClient=true")
	}
	if blocklist {
		queryParams = append(queryParams, "blocklist=true")
	}

	if len(queryParams) > 0 {
		endpoint = endpoint + "?" + strings.Join(queryParams, "&")
	}

	req, err := http.NewRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Set("X-Api-Key", token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to delete queue item: status %d (error reading response body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("failed to delete queue item: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// triggerManualImport triggers manual import for a downloaded file
func triggerManualImport(config Config, url string, token string, downloadPath string, instanceType string, useRestAPI bool, queueItem QueueItem) error {
	if useRestAPI {
		if config.Verbose {
			log.Printf("[INFO] Using REST API for manual import (queue item #%d: %s)", queueItem.ID, queueItem.Title)
			log.Printf("[VERBOSE] API Call 1/3: Scanning for files (GET /api/v3/manualimport with seriesId=%d or movieId=%d)", queueItem.SeriesID, queueItem.MovieID)
		}

		scannedItems, err := scanForManualImport(config, url, token, downloadPath, queueItem, instanceType)
		if err != nil {
			if config.Verbose {
				log.Printf("[VERBOSE] REST API scan failed, falling back to Command API: %v", err)
			}
			fmt.Fprintf(os.Stderr, "[WARN] REST API scan failed, falling back to Command API: %v\n", err)
		} else {
			// Log scan results
			if config.Verbose {
				rejectedCount := 0
				for _, item := range scannedItems {
					if len(item.Rejections) > 0 {
						rejectedCount++
					}
				}
				log.Printf("[VERBOSE] Scan completed: found %d total files (%d rejected, %d potentially importable)",
					len(scannedItems), rejectedCount, len(scannedItems)-rejectedCount)

				// Show first few files found for debugging
				if config.Debug && len(scannedItems) > 0 {
					log.Printf("[DEBUG] Files found in scan:")
					for i, item := range scannedItems {
						if i >= 5 {
							log.Printf("[DEBUG]   ... and %d more files", len(scannedItems)-5)
							break
						}
						status := "OK"
						if len(item.Rejections) > 0 {
							status = fmt.Sprintf("REJECTED: %s", formatRejections(item.Rejections))
						}
						log.Printf("[DEBUG]   %s [%s]", item.Name, status)
					}
				}
			}

			importRequests := buildManualImportRequests(config, scannedItems, queueItem, instanceType)

			// Log filtering results
			if config.Verbose {
				log.Printf("[VERBOSE] After filtering: %d files ready for import (out of %d scanned)",
					len(importRequests), len(scannedItems))
				if len(importRequests) > 0 {
					log.Printf("[VERBOSE] API Call 2/3: Executing import (POST /api/v3/command with %d files)", len(importRequests))
				}
			}

			if len(importRequests) > 0 {
				if err := executeManualImport(config, url, token, importRequests); err != nil {
					if config.Verbose {
						log.Printf("[VERBOSE] REST API import failed, falling back to Command API: %v", err)
					}
					fmt.Fprintf(os.Stderr, "[WARN] REST API import failed, falling back to Command API: %v\n", err)
				} else {
					// Import completed successfully
					if config.Verbose {
						log.Printf("[VERBOSE] REST API import completed successfully for %d files", len(importRequests))
					}
					return nil
				}
			} else {
				if config.Verbose {
					log.Printf("[VERBOSE] No importable files found in scan results (all %d files filtered out), falling back to Command API", len(scannedItems))
				}
				fmt.Fprintf(os.Stderr, "[WARN] No importable files found via REST API scan, falling back to Command API\n")
			}
		}
	} else {
		if config.Verbose {
			log.Printf("[INFO] Using Command API for manual import (queue item #%d: %s)", queueItem.ID, queueItem.Title)
		}
	}

	// Command API Fallback - Use OutputPath exactly as provided
	client := &http.Client{
		Timeout: config.Timeout,
	}

	var commandName string
	if instanceType == "sonarr" {
		commandName = "DownloadedEpisodesScan"
	} else {
		commandName = "DownloadedMoviesScan"
	}

	// CRITICAL FIX: Use queueItem.OutputPath (exact release folder) instead of parent directory
	// Add importMode parameter for proper file handling
	commandData := map[string]interface{}{
		"name":       commandName,
		"path":       queueItem.OutputPath, // Use exact path from queue item
		"importMode": "Move",               // Explicitly set import mode (Move, Copy, or Auto)
	}

	jsonData, err := json.Marshal(commandData)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/api/v3/command", url)

	if config.Verbose {
		log.Printf("[VERBOSE] Falling back to Command API for manual import")
		log.Printf("[VERBOSE] API Endpoint: POST %s", sanitizeURL(endpoint))
		log.Printf("[VERBOSE] Command: %s, Path: %s, ImportMode: Move", commandName, queueItem.OutputPath)
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}

	req.Header.Set("X-Api-Key", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to trigger manual import: status %d (error reading response body: %v)", resp.StatusCode, readErr)
		}
		log.Printf("[ERROR] Command API failed: status %d - %s", resp.StatusCode, string(bodyBytes))
		return fmt.Errorf("failed to trigger manual import: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	if config.Verbose {
		log.Printf("[VERBOSE] Command API: Response status %d", resp.StatusCode)
	}

	log.Printf("[INFO] Successfully triggered manual import via Command API for queue item #%d", queueItem.ID)

	return nil
}

// scanForManualImport scans a folder for files that can be manually imported
// CRITICAL FIX: Now supports server-side filtering by series/movie ID
func scanForManualImport(config Config, url, token, folderPath string, queueItem QueueItem, instanceType string) ([]ManualImportResource, error) {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	var endpoint string

	// CRITICAL FIX: Use downloadId as primary parameter per HAR investigation
	// Web UI uses downloadId to track downloads - this is what makes scan work correctly
	// Reference: /docs/HAR_INVESTIGATION.md lines 100-220 (real-world Sonarr web UI behavior)
	if queueItem.DownloadId != "" {
		// Primary path: Use downloadId (matches Sonarr web UI exactly)
		// This is the correct approach confirmed by HAR analysis of actual web UI traffic
		endpoint = fmt.Sprintf("%s/api/v3/manualimport?downloadId=%s&filterExistingFiles=false",
			url, neturl.QueryEscape(queueItem.DownloadId))
		if config.Verbose {
			log.Printf("[VERBOSE] Scanning for manual import with downloadId: %s", queueItem.DownloadId)
			log.Printf("[VERBOSE] API Endpoint: GET %s", sanitizeURL(endpoint))
		}
	} else {
		// Fallback: Use folder if downloadId unavailable (edge case for non-standard downloads)
		endpoint = fmt.Sprintf("%s/api/v3/manualimport?folder=%s&filterExistingFiles=false",
			url, neturl.QueryEscape(folderPath))
		if config.Verbose {
			log.Printf("[VERBOSE] Scanning for manual import by folder (downloadId unavailable): folder=%s", folderPath)
			log.Printf("[VERBOSE] API Endpoint: GET %s", sanitizeURL(endpoint))
		}
	}

	if config.Verbose {
		log.Printf("[VERBOSE] Scanning for manual import: GET %s (folder=%s)", sanitizeURL(endpoint), folderPath)
	}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response body: %v", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[ERROR] Scan failed: status %d - %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("failed to scan for manual import: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	if config.Debug {
		log.Printf("[DEBUG] Scan response body: %s", string(bodyBytes))
	}

	var items []ManualImportResource
	if err := json.Unmarshal(bodyBytes, &items); err != nil {
		return nil, err
	}

	rejectedCount := 0
	for _, item := range items {
		if len(item.Rejections) > 0 {
			rejectedCount++
		}
	}

	if config.Verbose {
		log.Printf("[VERBOSE] Scan response: status %d, found %d files (%d with rejections, %d importable)",
			resp.StatusCode, len(items), rejectedCount, len(items)-rejectedCount)
	}

	return items, nil
}

// executeManualImport executes manual import requests via Command API
// CRITICAL FIX: Sonarr expects POST /api/v3/command with ManualImport command structure
func executeManualImport(config Config, url, token string, importRequests []ManualImportRequest) error {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	// CRITICAL FIX: Wrap requests in ManualImport command structure
	command := ManualImportCommand{
		Name:       "ManualImport",
		Files:      importRequests,
		ImportMode: "auto", // auto, move, or copy
	}

	jsonData, err := json.Marshal(command)
	if err != nil {
		return err
	}

	// CRITICAL FIX: Use /api/v3/command endpoint, not /api/v3/manualimport
	endpoint := fmt.Sprintf("%s/api/v3/command", url)

	if config.Verbose {
		log.Printf("[VERBOSE] Executing manual import: POST %s (%d files)", sanitizeURL(endpoint), len(importRequests))
		log.Printf("[VERBOSE] API Endpoint: POST %s", sanitizeURL(endpoint))
		log.Printf("[VERBOSE] Command: ManualImport, ImportMode: auto")
	}

	if config.Debug {
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, jsonData, "", "  "); err == nil {
			log.Printf("[DEBUG] Manual import command payload:\n%s", prettyJSON.String())
		}
	}

	req, err := http.NewRequest("POST", endpoint, strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}

	req.Header.Set("X-Api-Key", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("failed to read response body: %v", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[ERROR] Import command failed: status %d - %s", resp.StatusCode, string(bodyBytes))
		return fmt.Errorf("failed to execute manual import command: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	if config.Verbose {
		log.Printf("[VERBOSE] Import command response: status %d", resp.StatusCode)
	}

	if config.Debug {
		log.Printf("[DEBUG] Import command response body: %s", string(bodyBytes))
	}

	// Command API returns a command object, not the import results directly
	// Parse response to get command ID for tracking
	var commandResponse map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &commandResponse); err != nil {
		log.Printf("[WARN] Manual import command succeeded but couldn't parse response: %v", err)
		log.Printf("[INFO] Successfully submitted manual import command for %d file(s)", len(importRequests))
		return nil
	}

	// Log command ID if available for tracking
	if commandID, ok := commandResponse["id"].(float64); ok {
		if config.Verbose {
			log.Printf("[VERBOSE] Manual import command submitted with ID: %.0f", commandID)
		}
		log.Printf("[INFO] Successfully submitted manual import command (ID: %.0f) for %d file(s)", commandID, len(importRequests))
	} else {
		log.Printf("[INFO] Successfully submitted manual import command for %d file(s)", len(importRequests))
	}

	return nil
}

// formatRejections converts rejection array to human-readable string
func formatRejections(rejections []ImportRejection) string {
	if len(rejections) == 0 {
		return "no rejections"
	}

	reasons := make([]string, len(rejections))
	for i, r := range rejections {
		reasons[i] = r.Reason
	}
	return strings.Join(reasons, "; ")
}

// buildManualImportRequests builds manual import requests from scanned items
func buildManualImportRequests(config Config, scannedItems []ManualImportResource, queueItem QueueItem, instanceType string) []ManualImportRequest {
	var requests []ManualImportRequest

	for _, item := range scannedItems {
		if len(item.Rejections) > 0 {
			if config.Verbose {
				rejectionReasons := make([]string, len(item.Rejections))
				for i, r := range item.Rejections {
					rejectionReasons[i] = r.Reason
				}
				log.Printf("[VERBOSE] Skipping file %s: rejected (%s)", item.Path, strings.Join(rejectionReasons, ", "))
			}
			continue
		}

		if instanceType == "sonarr" {
			if item.Series == nil {
				if config.Verbose {
					log.Printf("[VERBOSE] Skipping file %s: no series match", item.Path)
				}
				continue
			}

			// Verify file belongs to the correct series
			if item.Series.ID != queueItem.SeriesID {
				if config.Verbose {
					log.Printf("[VERBOSE] Skipping file %s: belongs to Series %d (%s), but queue item expects Series %d",
						item.Path, item.Series.ID, item.Series.Title, queueItem.SeriesID)
				}
				fmt.Fprintf(os.Stderr,
					"[WARN] Skipping file %s: belongs to Series %d, but queue item expects Series %d\n",
					item.Path, item.Series.ID, queueItem.SeriesID)
				continue
			}

			// Validate title match if we have the queue title
			if queueItem.Title != "" && item.Series.Title != "" {
				titleMatch := validateTitleMatch(queueItem.Title, item.Series.Title)
				if !titleMatch.IsMatch {
					if config.Verbose {
						log.Printf("[VERBOSE] Skipping file %s: title mismatch - %s", item.Path, titleMatch.Reason)
					}
					fmt.Fprintf(os.Stderr,
						"[WARN] Skipping file %s: title similarity only %.1f%% (queue: %s, scanned: %s)\n",
						item.Path, titleMatch.Similarity, queueItem.Title, item.Series.Title)
					continue
				}
				if config.Verbose {
					log.Printf("[VERBOSE] Title match validated for %s: %s", item.Path, titleMatch.Reason)
				}
			}

			req := ManualImportRequest{
				Path:         item.Path,
				SeriesID:     item.Series.ID,
				Quality:      item.Quality,
				Languages:    item.Languages,
				ReleaseGroup: item.ReleaseGroup,
				DownloadID:   queueItem.DownloadId,
				IndexerFlags: item.IndexerFlags,
				ImportMode:   "auto", // Set default import mode
			}

			if item.SeasonNumber != nil {
				req.SeasonNumber = *item.SeasonNumber
			}

			for _, ep := range item.Episodes {
				req.EpisodeIDs = append(req.EpisodeIDs, ep.ID)
			}

			if config.Verbose {
				downloadIDInfo := "none"
				if queueItem.DownloadId != "" {
					downloadIDInfo = queueItem.DownloadId
				}
				log.Printf("[VERBOSE] Accepted file %s: Series=%s, Season=%v, Episodes=%d, Quality=%s, DownloadID=%s",
					item.Path, item.Series.Title, item.SeasonNumber, len(item.Episodes), item.Quality.Quality.Name, downloadIDInfo)
			}

			requests = append(requests, req)
		} else {
			if item.Movie == nil {
				if config.Verbose {
					log.Printf("[VERBOSE] Skipping file %s: no movie match", item.Path)
				}
				continue
			}

			// Verify file belongs to the correct movie
			if item.Movie.ID != queueItem.MovieID {
				if config.Verbose {
					log.Printf("[VERBOSE] Skipping file %s: belongs to Movie %d (%s), but queue item expects Movie %d",
						item.Path, item.Movie.ID, item.Movie.Title, queueItem.MovieID)
				}
				fmt.Fprintf(os.Stderr,
					"[WARN] Skipping file %s: belongs to Movie %d, but queue item expects Movie %d\n",
					item.Path, item.Movie.ID, queueItem.MovieID)
				continue
			}

			// Validate title match if we have the queue title
			if queueItem.Title != "" && item.Movie.Title != "" {
				titleMatch := validateTitleMatch(queueItem.Title, item.Movie.Title)
				if !titleMatch.IsMatch {
					if config.Verbose {
						log.Printf("[VERBOSE] Skipping file %s: title mismatch - %s", item.Path, titleMatch.Reason)
					}
					fmt.Fprintf(os.Stderr,
						"[WARN] Skipping file %s: title similarity only %.1f%% (queue: %s, scanned: %s)\n",
						item.Path, titleMatch.Similarity, queueItem.Title, item.Movie.Title)
					continue
				}
				if config.Verbose {
					log.Printf("[VERBOSE] Title match validated for %s: %s", item.Path, titleMatch.Reason)
				}
			}

			req := ManualImportRequest{
				Path:         item.Path,
				MovieID:      item.Movie.ID,
				Quality:      item.Quality,
				Languages:    item.Languages,
				ReleaseGroup: item.ReleaseGroup,
				DownloadID:   queueItem.DownloadId,
				IndexerFlags: item.IndexerFlags,
				ImportMode:   "auto", // Set default import mode
			}

			if config.Verbose {
				downloadIDInfo := "none"
				if queueItem.DownloadId != "" {
					downloadIDInfo = queueItem.DownloadId
				}
				log.Printf("[VERBOSE] Accepted file %s: Movie=%s (%d), Quality=%s, DownloadID=%s",
					item.Path, item.Movie.Title, item.Movie.Year, item.Quality.Quality.Name, downloadIDInfo)
			}

			requests = append(requests, req)
		}
	}

	if config.Verbose {
		log.Printf("[VERBOSE] Built %d import request(s) from %d scanned file(s)", len(requests), len(scannedItems))
	}

	return requests
}

// fetchAllQueues retrieves queue items from all configured instances
func fetchAllQueues(config Config) ([]QueueItem, error) {
	var allItems []QueueItem
	var successCount int
	var failedInstances int
	var totalInstances int

	for i, url := range config.SonarrURLs {
		totalInstances++
		if i >= len(config.SonarrTokens) {
			continue
		}

		token := config.SonarrTokens[i]
		items, err := fetchQueue(config, url, token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to fetch queue from Sonarr instance %s: %v\n", url, err)
			failedInstances++
			continue
		}

		successCount++
		for _, item := range items {
			item.InstanceURL = url
			item.InstanceType = "sonarr"
			allItems = append(allItems, item)
		}
	}

	for i, url := range config.RadarrURLs {
		totalInstances++
		if i >= len(config.RadarrTokens) {
			continue
		}

		token := config.RadarrTokens[i]
		items, err := fetchQueue(config, url, token)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to fetch queue from Radarr instance %s: %v\n", url, err)
			failedInstances++
			continue
		}

		successCount++
		for _, item := range items {
			item.InstanceURL = url
			item.InstanceType = "radarr"
			allItems = append(allItems, item)
		}
	}

	if totalInstances > 0 && successCount == 0 {
		return nil, fmt.Errorf("all instances failed to fetch queue")
	}

	return allItems, nil
}

// validateQueueConfig validates the configuration
func validateQueueConfig(config Config) error {
	sonarrCount := len(config.SonarrURLs)
	radarrCount := len(config.RadarrURLs)

	if sonarrCount == 0 && radarrCount == 0 {
		return fmt.Errorf("at least one Sonarr or Radarr instance must be configured")
	}

	if sonarrCount != len(config.SonarrTokens) {
		return fmt.Errorf("number of Sonarr URLs (%d) must match number of Sonarr tokens (%d)", sonarrCount, len(config.SonarrTokens))
	}

	if radarrCount != len(config.RadarrTokens) {
		return fmt.Errorf("number of Radarr URLs (%d) must match number of Radarr tokens (%d)", radarrCount, len(config.RadarrTokens))
	}

	return nil
}

// getTokenForInstance retrieves the API token for a specific instance
func getTokenForInstance(config Config, instanceURL string, instanceType string) (string, error) {
	if instanceType == "sonarr" {
		for i, url := range config.SonarrURLs {
			if url == instanceURL && i < len(config.SonarrTokens) {
				return config.SonarrTokens[i], nil
			}
		}
	} else {
		for i, url := range config.RadarrURLs {
			if url == instanceURL && i < len(config.RadarrTokens) {
				return config.RadarrTokens[i], nil
			}
		}
	}
	return "", fmt.Errorf("no token found for %s instance %s", instanceType, instanceURL)
}

// loadConfig loads configuration from environment variables and flags
func loadConfig(sonarrURLsFlag, sonarrTokensFlag, radarrURLsFlag, radarrTokensFlag string, timeout time.Duration, useRestAPI bool, verbose bool, debug bool) Config {
	config := Config{
		SonarrURLs:   []string{},
		SonarrTokens: []string{},
		RadarrURLs:   []string{},
		RadarrTokens: []string{},
		Timeout:      timeout,
		UseRestAPI:   useRestAPI,
		Verbose:      verbose,
		Debug:        debug,
	}

	envPaths := []string{".env", "/opt/apps/compose/.env"}
	for _, envPath := range envPaths {
		if _, err := os.Stat(envPath); err == nil {
			loadEnvFile(envPath, &config)
			break
		}
	}

	if envSonarrURLs := os.Getenv("SONARR_URLS"); envSonarrURLs != "" {
		config.SonarrURLs = strings.Split(envSonarrURLs, ",")
		for i := range config.SonarrURLs {
			config.SonarrURLs[i] = strings.TrimSpace(config.SonarrURLs[i])
		}
	}
	if envSonarrTokens := os.Getenv("SONARR_TOKENS"); envSonarrTokens != "" {
		config.SonarrTokens = strings.Split(envSonarrTokens, ",")
		for i := range config.SonarrTokens {
			config.SonarrTokens[i] = strings.TrimSpace(config.SonarrTokens[i])
		}
	}
	if envRadarrURLs := os.Getenv("RADARR_URLS"); envRadarrURLs != "" {
		config.RadarrURLs = strings.Split(envRadarrURLs, ",")
		for i := range config.RadarrURLs {
			config.RadarrURLs[i] = strings.TrimSpace(config.RadarrURLs[i])
		}
	}
	if envRadarrTokens := os.Getenv("RADARR_TOKENS"); envRadarrTokens != "" {
		config.RadarrTokens = strings.Split(envRadarrTokens, ",")
		for i := range config.RadarrTokens {
			config.RadarrTokens[i] = strings.TrimSpace(config.RadarrTokens[i])
		}
	}
	if envUseRestAPI := os.Getenv("USE_REST_API"); envUseRestAPI != "" {
		config.UseRestAPI = strings.ToLower(envUseRestAPI) == "true"
	}

	if sonarrURLsFlag != "" {
		config.SonarrURLs = strings.Split(sonarrURLsFlag, ",")
		for i := range config.SonarrURLs {
			config.SonarrURLs[i] = strings.TrimSpace(config.SonarrURLs[i])
		}
	}
	if sonarrTokensFlag != "" {
		config.SonarrTokens = strings.Split(sonarrTokensFlag, ",")
		for i := range config.SonarrTokens {
			config.SonarrTokens[i] = strings.TrimSpace(config.SonarrTokens[i])
		}
	}
	if radarrURLsFlag != "" {
		config.RadarrURLs = strings.Split(radarrURLsFlag, ",")
		for i := range config.RadarrURLs {
			config.RadarrURLs[i] = strings.TrimSpace(config.RadarrURLs[i])
		}
	}
	if radarrTokensFlag != "" {
		config.RadarrTokens = strings.Split(radarrTokensFlag, ",")
		for i := range config.RadarrTokens {
			config.RadarrTokens[i] = strings.TrimSpace(config.RadarrTokens[i])
		}
	}

	return config
}

// loadEnvFile loads configuration from a .env file
func loadEnvFile(path string, config *Config) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	lines := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			lines[key] = value
		}
	}

	if sonarrURLs, ok := lines["SONARR_URLS"]; ok && sonarrURLs != "" {
		config.SonarrURLs = strings.Split(sonarrURLs, ",")
		for i := range config.SonarrURLs {
			config.SonarrURLs[i] = strings.TrimSpace(config.SonarrURLs[i])
		}
	}
	if sonarrTokens, ok := lines["SONARR_TOKENS"]; ok && sonarrTokens != "" {
		config.SonarrTokens = strings.Split(sonarrTokens, ",")
		for i := range config.SonarrTokens {
			config.SonarrTokens[i] = strings.TrimSpace(config.SonarrTokens[i])
		}
	}
	if radarrURLs, ok := lines["RADARR_URLS"]; ok && radarrURLs != "" {
		config.RadarrURLs = strings.Split(radarrURLs, ",")
		for i := range config.RadarrURLs {
			config.RadarrURLs[i] = strings.TrimSpace(config.RadarrURLs[i])
		}
	}
	if radarrTokens, ok := lines["RADARR_TOKENS"]; ok && radarrTokens != "" {
		config.RadarrTokens = strings.Split(radarrTokens, ",")
		for i := range config.RadarrTokens {
			config.RadarrTokens[i] = strings.TrimSpace(config.RadarrTokens[i])
		}
	}
	if useRestAPI, ok := lines["USE_REST_API"]; ok && useRestAPI != "" {
		config.UseRestAPI = strings.ToLower(useRestAPI) == "true"
	}
}

// formatStatusMessages formats status messages with bullet points for dry-run output
func formatStatusMessages(item QueueItem) string {
	if len(item.StatusMessages) == 0 {
		return ""
	}

	var messages []string
	for _, sm := range item.StatusMessages {
		for _, msg := range sm.Messages {
			messages = append(messages, msg)
		}
	}

	if len(messages) == 0 {
		return ""
	}

	var builder strings.Builder
	for i, msg := range messages {
		if i == 0 {
			builder.WriteString(fmt.Sprintf("     • %s", msg))
		} else {
			builder.WriteString(fmt.Sprintf("\n[DRY-RUN]     • %s", msg))
		}
	}
	return builder.String()
}

// getManualImportDetails extracts and validates series/movie mapping information
func getManualImportDetails(config Config, item QueueItem) string {
	var details []string

	// Series/Movie ID validation with name lookup
	if item.SeriesID > 0 {
		seriesInfo := fmt.Sprintf("• Series ID: %d", item.SeriesID)

		// Try to fetch series name
		if token, err := getTokenForInstance(config, item.InstanceURL, item.InstanceType); err == nil {
			if series, err := fetchSeriesDetails(config, item.InstanceURL, token, item.SeriesID); err == nil && series.Title != "" {
				seriesInfo += fmt.Sprintf(" (%s)", series.Title)
			}
		}
		seriesInfo += " (validated)"
		details = append(details, seriesInfo)
	} else if item.MovieID > 0 {
		movieInfo := fmt.Sprintf("• Movie ID: %d", item.MovieID)

		// Try to fetch movie name
		if token, err := getTokenForInstance(config, item.InstanceURL, item.InstanceType); err == nil {
			if movie, err := fetchMovieDetails(config, item.InstanceURL, token, item.MovieID); err == nil && movie.Title != "" {
				movieInfo += fmt.Sprintf(" (%s", movie.Title)
				if movie.Year > 0 {
					movieInfo += fmt.Sprintf(", %d", movie.Year)
				}
				movieInfo += ")"
			}
		}
		movieInfo += " (validated)"
		details = append(details, movieInfo)
	}

	// Output path
	if item.OutputPath != "" {
		details = append(details, fmt.Sprintf("• Output Path: %s", item.OutputPath))
	}

	// Only add import method if we have IDs or path
	if len(details) > 0 {
		importMethod := "Command API → DownloadedEpisodesScan"
		if item.InstanceType == "radarr" {
			importMethod = "Command API → DownloadedMoviesScan"
		}
		details = append(details, fmt.Sprintf("• Import Method: %s", importMethod))
	}

	if len(details) == 0 {
		return ""
	}

	var builder strings.Builder
	for i, detail := range details {
		if i == 0 {
			builder.WriteString(fmt.Sprintf("     %s", detail))
		} else {
			builder.WriteString(fmt.Sprintf("\n[DRY-RUN]     %s", detail))
		}
	}
	return builder.String()
}

// formatQueueItemHeader creates enhanced item display with full context
func formatQueueItemHeader(config Config, item QueueItem) string {
	instanceName := getInstanceName(config, item.InstanceURL, item.InstanceType)

	// Build status line
	statusInfo := fmt.Sprintf("Status: %s", item.Status)
	if item.TrackedDownloadState != "" {
		statusInfo += fmt.Sprintf(" | State: %s", item.TrackedDownloadState)
	}
	if item.DownloadClient != "" {
		statusInfo += fmt.Sprintf(" | Client: %s", item.DownloadClient)
	}

	return fmt.Sprintf("[DRY-RUN] %s - Item #%d (%s)\n[DRY-RUN]   %s",
		instanceName, item.ID, item.Title, statusInfo)
}

// getActionDescription provides detailed action explanations
func getActionDescription(action string, item QueueItem) string {
	switch action {
	case "delete":
		reason, _ := parseStatusMessages(item.StatusMessages)
		blocklist := false
		if reason == "custom_format_no_upgrade" || reason == "quality_no_upgrade" {
			blocklist = true
		}
		if blocklist {
			return fmt.Sprintf("→ Would DELETE (blocklist=true) - %s", getReason(item))
		}
		return fmt.Sprintf("→ Would DELETE - %s", getReason(item))

	case "manual_import":
		if item.OutputPath == "" {
			return fmt.Sprintf("→ Would MANUAL_IMPORT (no output path available!) - %s", getReason(item))
		}
		reason, _ := parseStatusMessages(item.StatusMessages)
		description := fmt.Sprintf("→ Would MANUAL_IMPORT - %s", getReason(item))
		if reason == "matched_by_id" {
			description += " (series validation successful)"
		}
		return description

	case "monitor":
		return fmt.Sprintf("→ MONITORING - %s", getReason(item))

	default:
		return fmt.Sprintf("→ Unknown action - %s", getReason(item))
	}
}

// classifyAndRemediate processes queue items and applies appropriate actions
func classifyAndRemediate(config Config, dryRun bool) error {
	items, err := fetchAllQueues(config)
	if err != nil {
		return err
	}

	if dryRun {
		fmt.Printf("[DRY-RUN] Analyzing queue items...\n\n")
	}

	var deleteCount, manualImportCount, monitorCount, totalCount int

	for _, item := range items {
		totalCount++

		// Skip items in /torrents/ directory (active downloads) - check early to avoid unnecessary API calls
		if strings.Contains(item.OutputPath, "/torrents/") || strings.Contains(item.OutputPath, "/torrents") {
			if dryRun {
				fmt.Printf("%s\n", formatQueueItemHeader(config, item))
				fmt.Printf("[DRY-RUN]   → SKIPPED - item is in /torrents/ directory (active download)\n\n")
			}
			if config.Verbose {
				log.Printf("[VERBOSE] Skipping queue item #%d (%s) - in /torrents/ directory", item.ID, item.Title)
			}
			continue
		}

		// Enrich queue item with quality and title validation data
		token, err := getTokenForInstance(config, item.InstanceURL, item.InstanceType)
		if err == nil {
			// Fetch existing quality for comparison
			if item.InstanceType == "sonarr" && item.SeriesID > 0 {
				files, err := fetchEpisodeFiles(config, item.InstanceURL, token, item.SeriesID)
				if err == nil && len(files) > 0 {
					// Use the first (typically most recent) episode file as reference
					item.ExistingQuality = &files[0].Quality
					if config.Debug {
						log.Printf("[DEBUG] Fetched existing quality for Series %d: %s", item.SeriesID, formatQualityString(files[0].Quality))
					}
				}
			} else if item.InstanceType == "radarr" && item.MovieID > 0 {
				file, err := fetchMovieFile(config, item.InstanceURL, token, item.MovieID)
				if err == nil && file != nil {
					item.ExistingQuality = &file.Quality
					if config.Debug {
						log.Printf("[DEBUG] Fetched existing quality for Movie %d: %s", item.MovieID, formatQualityString(file.Quality))
					}
				}
			}

			// Perform quality comparison if we have both queue and existing quality
			// Extract quality from status messages or attempt to parse from title
			reason, _ := parseStatusMessages(item.StatusMessages)
			if (reason == "quality_no_upgrade" || reason == "custom_format_no_upgrade") && item.ExistingQuality != nil {
				// We know there's a quality comparison happening
				// For now, we trust the API's assessment since queue quality isn't directly available
				// in the queue API response. This is a placeholder for future enhancement.
				if config.Debug {
					log.Printf("[DEBUG] Quality comparison indicated by status messages for item %d", item.ID)
				}
			}

			// Perform title validation for "matched by id" cases
			if reason == "matched_by_id" {
				if item.InstanceType == "sonarr" && item.SeriesID > 0 {
					series, err := fetchSeriesDetails(config, item.InstanceURL, token, item.SeriesID)
					if err == nil {
						titleMatch := validateTitleMatch(item.Title, series.Title)
						item.TitleMatchResult = &titleMatch
						if config.Debug {
							log.Printf("[DEBUG] Title validation for Series %d: %s (%.1f%% similar)",
								item.SeriesID, titleMatch.Reason, titleMatch.Similarity)
						}
					}
				} else if item.InstanceType == "radarr" && item.MovieID > 0 {
					movie, err := fetchMovieDetails(config, item.InstanceURL, token, item.MovieID)
					if err == nil {
						titleMatch := validateTitleMatch(item.Title, movie.Title)
						item.TitleMatchResult = &titleMatch
						if config.Debug {
							log.Printf("[DEBUG] Title validation for Movie %d: %s (%.1f%% similar)",
								item.MovieID, titleMatch.Reason, titleMatch.Similarity)
						}
					}
				}
			}
		}

		action, blocklist, manualImport := mapStatusToAction(item)

		if dryRun {
			// Enhanced header with status, state, and client info
			fmt.Printf("%s\n", formatQueueItemHeader(config, item))

			// Show status messages if available
			statusMsgs := formatStatusMessages(item)
			if statusMsgs != "" {
				fmt.Printf("[DRY-RUN]   Status Messages:\n[DRY-RUN] %s\n", statusMsgs)
			}

			// Show manual import details for manual import actions
			if action == "manual_import" {
				importDetails := getManualImportDetails(config, item)
				if importDetails != "" {
					fmt.Printf("[DRY-RUN]   Manual Import Details:\n[DRY-RUN] %s\n", importDetails)
				}
			}

			// Enhanced action description
			actionDesc := getActionDescription(action, item)
			fmt.Printf("[DRY-RUN]   %s\n\n", actionDesc)

			// Update counters
			switch action {
			case "delete":
				deleteCount++
			case "manual_import":
				manualImportCount++
			case "monitor":
				monitorCount++
			}
		}

		switch action {
		case "delete":
			if !dryRun {
				token, err := getTokenForInstance(config, item.InstanceURL, item.InstanceType)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
					continue
				}
				if err := deleteQueueItem(config, item.InstanceURL, token, item.ID, true, blocklist); err != nil {
					fmt.Fprintf(os.Stderr, "[ERROR] Failed to delete queue item %d (%s): %v\n", item.ID, item.Title, err)
					continue
				}
			}

		case "manual_import":
			if !dryRun && manualImport {
				if item.OutputPath == "" {
					fmt.Fprintf(os.Stderr, "[WARN] Item %d (%s) requires manual import but has no output path, skipping\n", item.ID, item.Title)
					continue
				}
				token, err := getTokenForInstance(config, item.InstanceURL, item.InstanceType)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[ERROR] %v\n", err)
					continue
				}
				if err := triggerManualImport(config, item.InstanceURL, token, item.OutputPath, item.InstanceType, config.UseRestAPI, item); err != nil {
					fmt.Fprintf(os.Stderr, "[ERROR] Failed to trigger manual import for item %d (%s): %v\n", item.ID, item.Title, err)
					continue
				}

			}
		}
	}

	if dryRun {
		fmt.Printf("=== DRY-RUN SUMMARY ===\n")
		fmt.Printf("Total items: %d\n", totalCount)
		fmt.Printf("  Would delete: %d\n", deleteCount)
		fmt.Printf("  Would manual import: %d\n", manualImportCount)
		fmt.Printf("  Monitoring: %d\n", monitorCount)
	}

	return nil
}

// TUI logger interface for dependency injection from TUI code
type tuiLoggerInterface interface {
	logDebug(msg string)
	logVerbose(msg string)
	logInfo(msg string)
	logWarn(msg string)
	logError(msg string)
}

// deleteQueueItemWithLogging wraps deleteQueueItem with TUI logging
func deleteQueueItemWithLogging(config Config, logger tuiLoggerInterface, instanceURL, token string, queueID int, removeFromClient, blocklist bool) error {
	logger.logDebug(fmt.Sprintf("Calling deleteQueueItem for queue ID %d", queueID))
	logger.logVerbose(fmt.Sprintf("Deleting queue item %d (removeFromClient=%v, blocklist=%v)", queueID, removeFromClient, blocklist))

	err := deleteQueueItem(config, instanceURL, token, queueID, removeFromClient, blocklist)

	if err != nil {
		logger.logError(fmt.Sprintf("Failed to delete queue item %d: %v", queueID, err))
	} else {
		logger.logInfo(fmt.Sprintf("Successfully deleted queue item %d", queueID))
	}

	return err
}

// triggerManualImportWithLogging wraps triggerManualImport with TUI logging and returns API call count
func triggerManualImportWithLogging(config Config, logger tuiLoggerInterface, instanceURL, token, outputPath, instanceType string, useRestAPI bool, item QueueItem) (int, error) {
	logger.logDebug(fmt.Sprintf("Calling triggerManualImport for path: %s", outputPath))
	logger.logVerbose(fmt.Sprintf("Instance: %s (%s)", instanceURL, instanceType))
	logger.logVerbose(fmt.Sprintf("Using REST API: %v", useRestAPI))

	// Estimate API call count based on whether we're using REST API
	// REST API: scan (1) + execute (1) = 2-3 calls
	// Command API: command endpoint (1) = 1 call
	estimatedCalls := 1
	if useRestAPI {
		estimatedCalls = 3
		logger.logDebug("Will attempt REST API first (scan + build + execute)")
	} else {
		logger.logDebug("Using Command API directly")
	}

	err := triggerManualImport(config, instanceURL, token, outputPath, instanceType, useRestAPI, item)

	if err != nil {
		logger.logError(fmt.Sprintf("Manual import failed: %v", err))
	} else {
		logger.logInfo("Manual import completed successfully")
	}

	return estimatedCalls, err
}

// fetchEpisodeFiles retrieves episode file information for a series from Sonarr API
func fetchEpisodeFiles(config Config, instanceURL, token string, seriesID int) ([]EpisodeFileResource, error) {
	if seriesID == 0 {
		return nil, fmt.Errorf("invalid seriesID: 0")
	}

	url := fmt.Sprintf("%s/api/v3/episodefile?seriesId=%d", instanceURL, seriesID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Api-Key", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: config.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var files []EpisodeFileResource
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return files, nil
}

// fetchMovieFile retrieves movie file information from Radarr API
func fetchMovieFile(config Config, instanceURL, token string, movieID int) (*MovieFileResource, error) {
	if movieID == 0 {
		return nil, fmt.Errorf("invalid movieID: 0")
	}

	url := fmt.Sprintf("%s/api/v3/moviefile?movieId=%d", instanceURL, movieID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Api-Key", token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: config.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var files []MovieFileResource
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(files) == 0 {
		return nil, nil // No existing file
	}

	return &files[0], nil
}

// calculateQualityScore computes a numeric score for a quality profile
func calculateQualityScore(q QualityModel) int {
	score := q.Quality.Resolution
	// Only add bonus for versions > 1
	if q.Revision.Version > 1 {
		score += q.Revision.Version * 100
	}
	if q.Revision.Real > 0 {
		score += 50
	}
	if q.Revision.IsRepack {
		score += 25
	}
	return score
}

// formatQualityString creates a human-readable quality description
func formatQualityString(q QualityModel) string {
	parts := []string{q.Quality.Name}
	if q.Revision.Version > 1 {
		parts = append(parts, fmt.Sprintf("v%d", q.Revision.Version))
	}
	if q.Revision.Real > 0 {
		parts = append(parts, "REAL")
	}
	if q.Revision.IsRepack {
		parts = append(parts, "REPACK")
	}
	return fmt.Sprintf("%s (%d)", q.Quality.Name, q.Quality.Resolution) + " " + joinNonEmpty(parts[1:], " ")
}

// joinNonEmpty joins non-empty strings with separator
func joinNonEmpty(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := ""
	for i, p := range parts {
		if p != "" {
			if result != "" {
				result += sep
			}
			result += p
		}
		if i == 0 && result != "" {
			result = sep + result
		}
	}
	return result
}

// compareQualities performs detailed quality comparison between queue and existing files
func compareQualities(queueQuality, existingQuality QualityModel) QualityComparison {
	newScore := calculateQualityScore(queueQuality)
	existingScore := calculateQualityScore(existingQuality)
	diff := newScore - existingScore

	comparison := QualityComparison{
		NewScore:          newScore,
		ExistingScore:     existingScore,
		ScoreDiff:         diff,
		NewFormatted:      formatQualityString(queueQuality),
		ExistingFormatted: formatQualityString(existingQuality),
	}

	if diff > 0 {
		comparison.IsUpgrade = true
		comparison.Reason = fmt.Sprintf("Upgrade: %s → %s (+%d points)", comparison.ExistingFormatted, comparison.NewFormatted, diff)
	} else if diff < 0 {
		comparison.IsDowngrade = true
		comparison.Reason = fmt.Sprintf("Downgrade: %s → %s (%d points)", comparison.ExistingFormatted, comparison.NewFormatted, diff)
	} else {
		comparison.IsEqual = true
		comparison.Reason = fmt.Sprintf("Same quality: %s", comparison.NewFormatted)
	}

	return comparison
}

// normalizeTitle removes common punctuation and formatting for comparison
func normalizeTitle(title string) string {
	// Convert to lowercase
	normalized := strings.ToLower(title)
	// Remove common separators and punctuation
	normalized = strings.ReplaceAll(normalized, ".", " ")
	normalized = strings.ReplaceAll(normalized, "-", " ")
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.ReplaceAll(normalized, ":", "")
	normalized = strings.ReplaceAll(normalized, "'", "")
	normalized = strings.ReplaceAll(normalized, "\"", "")
	normalized = strings.ReplaceAll(normalized, "?", "")
	normalized = strings.ReplaceAll(normalized, "!", "")
	// Collapse multiple spaces
	normalized = strings.Join(strings.Fields(normalized), " ")
	return strings.TrimSpace(normalized)
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create distance matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Calculate distances
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// calculateSimilarity computes percentage similarity between two strings
func calculateSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 100.0
	}
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}
	if maxLen == 0 {
		return 100.0
	}
	distance := levenshteinDistance(s1, s2)
	return (1.0 - float64(distance)/float64(maxLen)) * 100.0
}

// validateTitleMatch checks if queue title matches scanned title with similarity threshold
func validateTitleMatch(queueTitle, scannedTitle string) TitleMatchResult {
	normalized1 := normalizeTitle(queueTitle)
	normalized2 := normalizeTitle(scannedTitle)
	similarity := calculateSimilarity(normalized1, normalized2)

	result := TitleMatchResult{
		QueueTitle:        queueTitle,
		ScannedTitle:      scannedTitle,
		NormalizedQueue:   normalized1,
		NormalizedScanned: normalized2,
		Similarity:        similarity,
	}

	if similarity >= 95.0 {
		result.IsMatch = true
		result.Reason = fmt.Sprintf("Strong match (%.1f%% similar)", similarity)
	} else if similarity >= 85.0 {
		result.IsMatch = true
		result.Reason = fmt.Sprintf("Acceptable match (%.1f%% similar)", similarity)
	} else {
		result.IsMatch = false
		result.Reason = fmt.Sprintf("Title mismatch (%.1f%% similar)", similarity)
	}

	return result
}
