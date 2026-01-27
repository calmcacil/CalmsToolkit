//go:build queueremediation

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
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
	EpisodeID             int                `json:"episodeId,omitempty"`
	EpisodeIDs            []int              `json:"episodeIds,omitempty"`
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
	ID                    int    `json:"id"`
	SeriesID              int    `json:"seriesId"`
	SeasonNumber          int    `json:"seasonNumber"`
	EpisodeNumber         int    `json:"episodeNumber"`
	AbsoluteEpisodeNumber int    `json:"absoluteEpisodeNumber,omitempty"`
	Title                 string `json:"title"`
	AirDateUtc            string `json:"airDateUtc,omitempty"`
}

// QualityModel represents quality information
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
	FolderName   string       `json:"folderName,omitempty"`
	SeriesID     int          `json:"seriesId,omitempty"`
	MovieID      int          `json:"movieId,omitempty"`
	SeasonNumber int          `json:"seasonNumber,omitempty"`
	EpisodeIDs   []int        `json:"episodeIds,omitempty"`
	Quality      QualityModel `json:"quality,omitempty"`
	Languages    []Language   `json:"languages,omitempty"`
	ReleaseGroup string       `json:"releaseGroup,omitempty"`
	ReleaseType  string       `json:"releaseType,omitempty"`
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

// ManualImportReprocessResource represents a resource for POST /api/v3/manualimport
type ManualImportReprocessResource struct {
	ID           int          `json:"id,omitempty"`
	Path         string       `json:"path"`
	SeriesID     int          `json:"seriesId,omitempty"`
	SeasonNumber int          `json:"seasonNumber,omitempty"`
	EpisodeIDs   []int        `json:"episodeIds,omitempty"`
	Quality      QualityModel `json:"quality,omitempty"`
	Languages    []Language   `json:"languages,omitempty"`
	ReleaseGroup string       `json:"releaseGroup,omitempty"`
	DownloadID   string       `json:"downloadId,omitempty"`
	IndexerFlags int          `json:"indexerFlags,omitempty"`
	ReleaseType  string       `json:"releaseType,omitempty"`
}

// CommandResource represents a command status response
type CommandResource struct {
	ID        int    `json:"id"`
	Status    string `json:"status"`
	Result    string `json:"result"`
	Message   string `json:"message"`
	Exception string `json:"exception"`
}

// ParseResource represents Sonarr parse endpoint response (partial)
type ParseResource struct {
	Series            SeriesResource    `json:"series"`
	Episodes          []EpisodeResource `json:"episodes"`
	ParsedEpisodeInfo ParsedEpisodeInfo `json:"parsedEpisodeInfo"`
}

// ParsedEpisodeInfo represents parsed episode details from release title
type ParsedEpisodeInfo struct {
	SeasonNumber           int   `json:"seasonNumber"`
	EpisodeNumbers         []int `json:"episodeNumbers"`
	AbsoluteEpisodeNumbers []int `json:"absoluteEpisodeNumbers"`
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

var errManualImportNoMapping = errors.New("manual import mapping not possible")

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
	statusLower := strings.ToLower(item.Status)
	stateLower := strings.ToLower(item.TrackedDownloadState)
	healthLower := strings.ToLower(item.TrackedDownloadStatus)

	reason, shouldBlocklist := parseStatusMessages(item.StatusMessages)

	// Hard failures
	if statusLower == "failed" || stateLower == "failed" || stateLower == "failedpending" || healthLower == "error" {
		return "delete", shouldBlocklist, false
	}

	// Import blocked needs intervention
	if stateLower == "importblocked" || strings.Contains(strings.ToLower(item.ErrorMessage), "import blocked") {
		if reason == "custom_format_no_upgrade" || reason == "quality_no_upgrade" {
			return "delete", true, false
		}
		// Sample files and no-files should be deleted, not manual imported
		if reason == "sample_file" || reason == "no_files_found" {
			return "delete", false, false
		}
		if item.TitleMatchResult != nil && !item.TitleMatchResult.IsMatch {
			return "delete", true, false
		}
		return "manual_import", false, true
	}

	switch reason {
	case "custom_format_no_upgrade":
		if item.QualityComparison != nil {
			if item.QualityComparison.IsUpgrade {
				return "manual_import", false, true
			}
			return "delete", shouldBlocklist, false
		}
		return "delete", shouldBlocklist, false
	case "quality_no_upgrade":
		if item.QualityComparison != nil {
			if item.QualityComparison.IsUpgrade {
				return "manual_import", false, true
			}
			if item.QualityComparison.IsDowngrade {
				return "delete", true, false
			}
			return "delete", shouldBlocklist, false
		}
		return "delete", shouldBlocklist, false
	case "no_files_found":
		return "delete", shouldBlocklist, false
	case "sample_file":
		return "delete", shouldBlocklist, false
	case "matched_by_id":
		// Sonarr: attempt manual import with downloadId/library fallback.
		// Radarr: keep delete behavior.
		if item.InstanceType == "sonarr" {
			return "manual_import", false, true
		}
		return "delete", true, false
	case "mapping_pending":
		return "manual_import", false, true
	}

	// Warnings and degraded health on completed downloads should be handled
	if statusLower == "warning" || healthLower == "warning" {
		return "manual_import", false, true
	}

	if stateLower == "importpending" && reason != "unknown" {
		return "manual_import", false, true
	}

	return "monitor", false, false
}

// parseStatusMessages analyzes status messages to determine the reason for queue item status
func parseStatusMessages(statusMessages []StatusMessage) (string, bool) {
	var hasQualityCF, hasSample, hasNoFiles, hasIDMatch, hasMappingPending bool
	var isCustomFormat bool

	for _, sm := range statusMessages {
		for _, msg := range sm.Messages {
			msgLower := strings.ToLower(msg)

			if strings.Contains(msgLower, "custom format upgrade") {
				hasQualityCF = true
				isCustomFormat = true
			}

			if strings.Contains(msgLower, "quality revision") || strings.Contains(msgLower, "quality not an upgrade") || strings.Contains(msgLower, "not an upgrade") {
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

			if strings.Contains(msgLower, "thexem") || strings.Contains(msgLower, "needs manual input") || strings.Contains(msgLower, "tba title") {
				hasMappingPending = true
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

	// mapping_pending takes precedence over sample_file because:
	// 1. Downloads often contain BOTH sample files AND the actual episode
	// 2. The "Sample" status message refers to one file, not the entire download
	// 3. Manual import scan will filter out sample files via Rejections array
	// 4. TheXEM mapping issues require manual import to resolve
	if hasMappingPending {
		return "mapping_pending", false
	}

	// Only treat as sample_file if that's the ONLY issue (no mapping pending)
	if hasSample {
		return "sample_file", false
	}

	if hasNoFiles {
		return "no_files_found", false
	}

	return "unknown", false
}

// needsRemediation determines if an item should be surfaced for action
func needsRemediation(item QueueItem) bool {
	if strings.Contains(item.OutputPath, "/torrents/") {
		return false
	}

	statusLower := strings.ToLower(item.Status)
	stateLower := strings.ToLower(item.TrackedDownloadState)
	healthLower := strings.ToLower(item.TrackedDownloadStatus)

	if statusLower == "failed" || statusLower == "warning" || statusLower == "downloadclientunavailable" {
		return true
	}

	if stateLower == "importblocked" || stateLower == "failedpending" || stateLower == "failed" || stateLower == "ignored" {
		return true
	}

	if healthLower == "warning" || healthLower == "error" {
		return true
	}

	reason, _ := parseStatusMessages(item.StatusMessages)

	if stateLower == "importpending" && reason != "unknown" {
		return true
	}

	if stateLower == "importpending" && healthLower != "" && healthLower != "ok" {
		return true
	}

	return false
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
	case "mapping_pending":
		return "metadata mapping pending"
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

// fetchEpisodeDetails fetches episode details by ID from Sonarr API
func fetchEpisodeDetails(config Config, instanceURL, token string, episodeID int) (*EpisodeResource, error) {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	endpoint := fmt.Sprintf("%s/api/v3/episode/%d", instanceURL, episodeID)

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

	var episode EpisodeResource
	if err := json.NewDecoder(resp.Body).Decode(&episode); err != nil {
		return nil, err
	}

	return &episode, nil
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
// Both REST API and Command API paths use the same workflow: scan → build → execute
// The executeManualImport() function uses the correct ManualImport command
func triggerManualImport(config Config, url string, token string, downloadPath string, instanceType string, useRestAPI bool, queueItem QueueItem) error {
	if config.Verbose {
		if useRestAPI {
			log.Printf("[INFO] Using REST API for manual import (queue item #%d: %s)", queueItem.ID, queueItem.Title)
		} else {
			log.Printf("[INFO] Using Command API for manual import (queue item #%d: %s)", queueItem.ID, queueItem.Title)
		}
	}

	// Step 1: Scan for files
	if config.Verbose {
		log.Printf("[VERBOSE] API Call 1/3: Scanning for files (GET /api/v3/manualimport with seriesId=%d or movieId=%d)", queueItem.SeriesID, queueItem.MovieID)
	}

	scannedItems, err := scanForManualImport(config, url, token, downloadPath, queueItem, instanceType)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

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

	// Step 2: Build import requests
	importRequests := buildManualImportRequests(config, scannedItems, queueItem, instanceType)

	// Fallback mapping for Sonarr matched_by_id when scan can't be used directly
	if len(importRequests) == 0 && instanceType == "sonarr" {
		reason, _ := parseStatusMessages(queueItem.StatusMessages)
		if reason == "matched_by_id" {
			fallbackRequests, match, fallbackErr := buildFallbackManualImportRequests(config, url, token, scannedItems, queueItem)
			if fallbackErr == nil && len(fallbackRequests) > 0 {
				importRequests = fallbackRequests
				if config.Verbose {
					log.Printf("[VERBOSE] Fallback mapping selected series match: %.1f%% (%s)", match.Similarity, match.Reason)
				}
			} else if fallbackErr != nil && fallbackErr != errManualImportNoMapping {
				return fmt.Errorf("fallback mapping failed: %w", fallbackErr)
			}
		}
	}

	// Log filtering results
	if config.Verbose {
		log.Printf("[VERBOSE] After filtering: %d files ready for import (out of %d scanned)",
			len(importRequests), len(scannedItems))
	}

	if len(importRequests) == 0 {
		reason, _ := parseStatusMessages(queueItem.StatusMessages)
		if reason == "matched_by_id" {
			return errManualImportNoMapping
		}
		return fmt.Errorf("no importable files found (all %d scanned files were rejected or filtered out)", len(scannedItems))
	}

	// Step 3: Execute import using ManualImport command
	if config.Verbose {
		log.Printf("[VERBOSE] API Call 2/3: Executing import (POST /api/v3/command with %d files)", len(importRequests))
	}

	if err := executeManualImport(config, url, token, importRequests); err != nil {
		if instanceType == "sonarr" {
			if config.Verbose {
				log.Printf("[VERBOSE] Manual import command failed; attempting reprocess fallback: %v", err)
			}
			if fallbackErr := executeManualImportReprocess(config, url, token, importRequests); fallbackErr == nil {
				return nil
			}
		}
		return fmt.Errorf("manual import failed: %w", err)
	}

	// Import completed successfully
	if config.Verbose {
		log.Printf("[VERBOSE] Manual import completed successfully for %d files", len(importRequests))
	}

	log.Printf("[INFO] Successfully triggered manual import for queue item #%d", queueItem.ID)

	return nil
}

// scanForManualImport scans a folder for files that can be manually imported
// CRITICAL FIX: Now supports server-side filtering by series/movie ID
func scanForManualImport(config Config, url, token, folderPath string, queueItem QueueItem, instanceType string) ([]ManualImportResource, error) {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	var endpoint string

	// Prefer downloadId-first (matches Sonarr/Radarr web UI); fall back to folder scan
	if queueItem.DownloadId != "" {
		// downloadId path should NOT include folder; filterExistingFiles=false per HAR traces
		endpoint = fmt.Sprintf("%s/api/v3/manualimport?downloadId=%s&filterExistingFiles=false",
			url, neturl.QueryEscape(queueItem.DownloadId))
		if config.Verbose {
			log.Printf("[VERBOSE] Scanning for manual import with downloadId: %s", queueItem.DownloadId)
			log.Printf("[VERBOSE] API Endpoint: GET %s", sanitizeURL(endpoint))
		}
	} else if instanceType == "sonarr" && queueItem.SeriesID > 0 {
		// Fallback with folder + seriesId when no downloadId is present
		endpoint = fmt.Sprintf("%s/api/v3/manualimport?folder=%s&seriesId=%d&filterExistingFiles=false",
			url, neturl.QueryEscape(folderPath), queueItem.SeriesID)
		if config.Verbose {
			log.Printf("[VERBOSE] Scanning for manual import with folder + SeriesID filter: folder=%s, seriesId=%d", folderPath, queueItem.SeriesID)
			log.Printf("[VERBOSE] API Endpoint: GET %s", sanitizeURL(endpoint))
		}
	} else if instanceType == "radarr" && queueItem.MovieID > 0 {
		// Fallback with folder + movieId when no downloadId is present
		endpoint = fmt.Sprintf("%s/api/v3/manualimport?folder=%s&movieId=%d&filterExistingFiles=false",
			url, neturl.QueryEscape(folderPath), queueItem.MovieID)
		if config.Verbose {
			log.Printf("[VERBOSE] Scanning for manual import with folder + MovieID filter: folder=%s, movieId=%d", folderPath, queueItem.MovieID)
			log.Printf("[VERBOSE] API Endpoint: GET %s", sanitizeURL(endpoint))
		}
	} else {
		// Folder-only fallback
		endpoint = fmt.Sprintf("%s/api/v3/manualimport?folder=%s&filterExistingFiles=false",
			url, neturl.QueryEscape(folderPath))
		if config.Verbose {
			log.Printf("[VERBOSE] Scanning for manual import by folder only (no ID available for filtering)")
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

func hasHardRejection(rejections []ImportRejection) bool {
	for _, rejection := range rejections {
		typeLower := strings.ToLower(rejection.Type)
		if typeLower == "permanent" {
			return true
		}
		reasonLower := strings.ToLower(rejection.Reason)
		if strings.Contains(reasonLower, "unknown series") || strings.Contains(reasonLower, "unknown movie") {
			return true
		}
	}
	return false
}

func buildFallbackManualImportRequests(config Config, url, token string, scannedItems []ManualImportResource, queueItem QueueItem) ([]ManualImportRequest, TitleMatchResult, error) {
	if len(scannedItems) == 0 {
		return nil, TitleMatchResult{}, errManualImportNoMapping
	}
	if len(scannedItems) > 1 {
		if config.Verbose {
			log.Printf("[VERBOSE] Fallback mapping skipped: multiple scanned items (%d)", len(scannedItems))
		}
		return nil, TitleMatchResult{}, errManualImportNoMapping
	}
	item := scannedItems[0]
	if item.Path == "" {
		return nil, TitleMatchResult{}, errManualImportNoMapping
	}

	series, match, err := resolveSeriesForQueueItem(config, url, token, queueItem)
	if err != nil {
		return nil, TitleMatchResult{}, err
	}

	episodeIDs, err := resolveEpisodeIDs(config, url, token, series.ID, queueItem)
	if err != nil {
		return nil, TitleMatchResult{}, err
	}
	if len(episodeIDs) == 0 {
		return nil, TitleMatchResult{}, errManualImportNoMapping
	}

	request := buildFallbackRequestFromScan(item, queueItem, series.ID, episodeIDs)
	return []ManualImportRequest{request}, match, nil
}

func buildFallbackRequestFromScan(item ManualImportResource, queueItem QueueItem, seriesID int, episodeIDs []int) ManualImportRequest {
	folderName := filepath.Base(filepath.Dir(item.Path))
	return ManualImportRequest{
		Path:         item.Path,
		FolderName:   folderName,
		SeriesID:     seriesID,
		EpisodeIDs:   episodeIDs,
		Quality:      item.Quality,
		Languages:    item.Languages,
		ReleaseGroup: item.ReleaseGroup,
		ReleaseType:  "singleEpisode",
		DownloadID:   queueItem.DownloadId,
		IndexerFlags: item.IndexerFlags,
		ImportMode:   "auto",
	}
}

func resolveSeriesForQueueItem(config Config, url, token string, queueItem QueueItem) (SeriesResource, TitleMatchResult, error) {
	if queueItem.SeriesID > 0 {
		series, err := fetchSeriesDetails(config, url, token, queueItem.SeriesID)
		if err == nil && series != nil {
			match := validateTitleMatch(queueItem.Title, series.Title)
			if match.Similarity >= 80.0 {
				return *series, match, nil
			}
		}
	}

	matchTitle := queueItem.Title
	parsed, err := parseReleaseTitle(config, url, token, queueItem.Title)
	if err == nil && parsed != nil && parsed.Series.Title != "" {
		matchTitle = parsed.Series.Title
	}

	allSeries, err := fetchSeriesList(config, url, token)
	if err != nil {
		return SeriesResource{}, TitleMatchResult{}, err
	}

	var best SeriesResource
	bestMatch := TitleMatchResult{Similarity: 0}
	for _, series := range allSeries {
		match := validateTitleMatch(matchTitle, series.Title)
		if match.Similarity > bestMatch.Similarity {
			bestMatch = match
			best = series
		}
	}

	if bestMatch.Similarity < 80.0 {
		return SeriesResource{}, bestMatch, errManualImportNoMapping
	}

	return best, bestMatch, nil
}

func resolveEpisodeIDs(config Config, url, token string, seriesID int, queueItem QueueItem) ([]int, error) {
	if queueItem.EpisodeID > 0 {
		return []int{queueItem.EpisodeID}, nil
	}

	parsed, err := parseReleaseTitle(config, url, token, queueItem.Title)
	if err != nil {
		return nil, err
	}

	if len(parsed.Episodes) > 0 {
		var ids []int
		for _, episode := range parsed.Episodes {
			if episode.ID > 0 {
				ids = append(ids, episode.ID)
			}
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}

	season := parsed.ParsedEpisodeInfo.SeasonNumber
	if season == 0 {
		return nil, errManualImportNoMapping
	}

	if len(parsed.ParsedEpisodeInfo.EpisodeNumbers) == 0 && len(parsed.ParsedEpisodeInfo.AbsoluteEpisodeNumbers) == 0 {
		return nil, errManualImportNoMapping
	}

	episodes, err := fetchEpisodesBySeries(config, url, token, seriesID)
	if err != nil {
		return nil, err
	}

	var ids []int
	if len(parsed.ParsedEpisodeInfo.EpisodeNumbers) > 0 {
		wanted := make(map[int]bool)
		for _, number := range parsed.ParsedEpisodeInfo.EpisodeNumbers {
			wanted[number] = true
		}
		for _, episode := range episodes {
			if episode.SeasonNumber == season && wanted[episode.EpisodeNumber] {
				ids = append(ids, episode.ID)
			}
		}
	} else {
		wanted := make(map[int]bool)
		for _, number := range parsed.ParsedEpisodeInfo.AbsoluteEpisodeNumbers {
			wanted[number] = true
		}
		for _, episode := range episodes {
			if wanted[episode.AbsoluteEpisodeNumber] {
				ids = append(ids, episode.ID)
			}
		}
	}

	if len(ids) == 0 {
		return nil, errManualImportNoMapping
	}

	return ids, nil
}

func fetchSeriesList(config Config, instanceURL, token string) ([]SeriesResource, error) {
	client := &http.Client{Timeout: config.Timeout}
	endpoint := fmt.Sprintf("%s/api/v3/series", instanceURL)
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
	var series []SeriesResource
	if err := json.NewDecoder(resp.Body).Decode(&series); err != nil {
		return nil, err
	}
	return series, nil
}

func fetchEpisodesBySeries(config Config, instanceURL, token string, seriesID int) ([]EpisodeResource, error) {
	client := &http.Client{Timeout: config.Timeout}
	endpoint := fmt.Sprintf("%s/api/v3/episode?seriesId=%d", instanceURL, seriesID)
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
	var episodes []EpisodeResource
	if err := json.NewDecoder(resp.Body).Decode(&episodes); err != nil {
		return nil, err
	}
	return episodes, nil
}

func parseReleaseTitle(config Config, instanceURL, token, title string) (*ParseResource, error) {
	client := &http.Client{Timeout: config.Timeout}
	endpoint := fmt.Sprintf("%s/api/v3/parse?title=%s", instanceURL, neturl.QueryEscape(title))
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
	var parsed ParseResource
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
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
		if err := waitForCommandCompletion(config, url, token, int(commandID)); err != nil {
			return err
		}
	} else {
		log.Printf("[INFO] Successfully submitted manual import command for %d file(s)", len(importRequests))
	}

	return nil
}

func executeManualImportReprocess(config Config, url, token string, importRequests []ManualImportRequest) error {
	client := &http.Client{Timeout: config.Timeout}

	resources := make([]ManualImportReprocessResource, 0, len(importRequests))
	for _, request := range importRequests {
		resources = append(resources, ManualImportReprocessResource{
			Path:         request.Path,
			SeriesID:     request.SeriesID,
			SeasonNumber: request.SeasonNumber,
			EpisodeIDs:   request.EpisodeIDs,
			Quality:      request.Quality,
			Languages:    request.Languages,
			ReleaseGroup: request.ReleaseGroup,
			DownloadID:   request.DownloadID,
			IndexerFlags: request.IndexerFlags,
			ReleaseType:  request.ReleaseType,
		})
	}

	jsonData, err := json.Marshal(resources)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/api/v3/manualimport", url)
	if config.Verbose {
		log.Printf("[VERBOSE] Executing manual import reprocess: POST %s (%d files)", sanitizeURL(endpoint), len(resources))
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
		return fmt.Errorf("failed to execute manual import reprocess: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	if config.Verbose {
		log.Printf("[VERBOSE] Manual import reprocess response: status %d", resp.StatusCode)
	}

	return nil
}

func waitForCommandCompletion(config Config, url, token string, commandID int) error {
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		command, err := fetchCommandStatus(config, url, token, commandID)
		if err != nil {
			return err
		}
		status := strings.ToLower(command.Status)
		result := strings.ToLower(command.Result)
		switch status {
		case "completed":
			if result == "successful" || result == "" {
				return nil
			}
			return fmt.Errorf("manual import command failed: %s", command.Message)
		case "failed", "aborted", "cancelled", "orphaned":
			return fmt.Errorf("manual import command %s: %s", status, command.Message)
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("manual import command %d did not complete before timeout", commandID)
}

func fetchCommandStatus(config Config, instanceURL, token string, commandID int) (*CommandResource, error) {
	client := &http.Client{Timeout: config.Timeout}
	endpoint := fmt.Sprintf("%s/api/v3/command/%d", instanceURL, commandID)
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
	var command CommandResource
	if err := json.NewDecoder(resp.Body).Decode(&command); err != nil {
		return nil, err
	}
	return &command, nil
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
			rejectionReasons := make([]string, len(item.Rejections))
			for i, r := range item.Rejections {
				rejectionReasons[i] = r.Reason
			}
			if hasHardRejection(item.Rejections) {
				if config.Verbose {
					log.Printf("[VERBOSE] Skipping file %s: hard rejection (%s)", item.Path, strings.Join(rejectionReasons, ", "))
				}
				continue
			}
			if config.Verbose {
				log.Printf("[VERBOSE] Soft rejections present for %s (%s) - continuing", item.Path, strings.Join(rejectionReasons, ", "))
			}
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

			// Extract folder name from file path (parent directory)
			folderName := filepath.Base(filepath.Dir(item.Path))

			req := ManualImportRequest{
				Path:         item.Path,
				FolderName:   folderName,
				SeriesID:     item.Series.ID,
				Quality:      item.Quality,
				Languages:    item.Languages,
				ReleaseGroup: item.ReleaseGroup,
				ReleaseType:  "singleEpisode",
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
				log.Printf("[VERBOSE] Accepted file %s: Series=%s, Season=%v, Episodes=%d, Quality=%s, FolderName=%s, ReleaseType=%s, DownloadID=%s",
					item.Path, item.Series.Title, item.SeasonNumber, len(item.Episodes), item.Quality.Quality.Name, folderName, "singleEpisode", downloadIDInfo)
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
	reason, _ := parseStatusMessages(item.StatusMessages)

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
		importMethod := "Command API → ManualImport"
		details = append(details, fmt.Sprintf("• Import Method: %s", importMethod))
		if item.InstanceType == "sonarr" && reason == "matched_by_id" {
			details = append(details, "• Mapping: downloadId scan, fallback to library match if needed")
		}
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
			description += " (fallback to delete on mapping failure)"
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
					if errors.Is(err, errManualImportNoMapping) {
						fmt.Fprintf(os.Stderr, "[WARN] Mapping failed; deleting queue item %d (%s)\n", item.ID, item.Title)
						if delErr := deleteQueueItem(config, item.InstanceURL, token, item.ID, true, false); delErr != nil {
							fmt.Fprintf(os.Stderr, "[ERROR] Failed to delete queue item %d (%s) after mapping failure: %v\n", item.ID, item.Title, delErr)
						} else if config.Verbose {
							log.Printf("[VERBOSE] Deleted queue item %d (%s) after mapping failure", item.ID, item.Title)
						}
						continue
					}
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

// calculateSimilarity computes weighted similarity between two strings
// using character-level (60%), token-level (30%), and length ratio (10%) scoring
func calculateSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 100.0
	}

	// Component 1: Character-level similarity (Levenshtein) - 60% weight
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}
	if maxLen == 0 {
		return 100.0
	}
	distance := levenshteinDistance(s1, s2)
	charSimilarity := (1.0 - float64(distance)/float64(maxLen)) * 100.0

	// Component 2: Token-level similarity (Jaccard) - 30% weight
	tokenSimilarity := calculateTokenSimilarity(s1, s2)

	// Component 3: Length ratio - 10% weight
	lengthRatio := calculateLengthRatio(s1, s2)

	// Weighted final score
	finalScore := (0.6 * charSimilarity) + (0.3 * tokenSimilarity) + (0.1 * lengthRatio)

	return finalScore
}

// calculateTokenSimilarity computes word-level overlap using Jaccard similarity
// Filters out common articles and stop words before comparison
func calculateTokenSimilarity(s1, s2 string) float64 {
	// Common articles and stop words to ignore
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true,
	}

	// Split into tokens and filter stop words
	tokens1Raw := strings.Fields(s1)
	tokens2Raw := strings.Fields(s2)

	tokens1 := []string{}
	for _, t := range tokens1Raw {
		if !stopWords[t] {
			tokens1 = append(tokens1, t)
		}
	}

	tokens2 := []string{}
	for _, t := range tokens2Raw {
		if !stopWords[t] {
			tokens2 = append(tokens2, t)
		}
	}

	if len(tokens1) == 0 && len(tokens2) == 0 {
		return 100.0
	}
	if len(tokens1) == 0 || len(tokens2) == 0 {
		return 0.0
	}

	// Build sets for intersection/union calculation
	set1 := make(map[string]bool)
	for _, token := range tokens1 {
		set1[token] = true
	}

	set2 := make(map[string]bool)
	for _, token := range tokens2 {
		set2[token] = true
	}

	// Calculate intersection
	intersection := 0
	for token := range set1 {
		if set2[token] {
			intersection++
		}
	}

	// Calculate union
	union := len(set1)
	for token := range set2 {
		if !set1[token] {
			union++
		}
	}

	if union == 0 {
		return 100.0
	}

	return (float64(intersection) / float64(union)) * 100.0
}

// calculateLengthRatio computes length similarity between two strings
func calculateLengthRatio(s1, s2 string) float64 {
	len1 := len(s1)
	len2 := len(s2)

	if len1 == 0 && len2 == 0 {
		return 100.0
	}
	if len1 == 0 || len2 == 0 {
		return 0.0
	}

	minLen := float64(len1)
	maxLen := float64(len2)
	if len1 > len2 {
		minLen = float64(len2)
		maxLen = float64(len1)
	}

	return (minLen / maxLen) * 100.0
}

// validateTitleMatch checks if queue title matches scanned title with hybrid similarity logic
func validateTitleMatch(queueTitle, scannedTitle string) TitleMatchResult {
	normalized1 := normalizeTitle(queueTitle)
	normalized2 := normalizeTitle(scannedTitle)

	// Calculate individual components
	maxLen := len(normalized1)
	if len(normalized2) > maxLen {
		maxLen = len(normalized2)
	}
	if maxLen == 0 {
		return TitleMatchResult{
			QueueTitle:        queueTitle,
			ScannedTitle:      scannedTitle,
			NormalizedQueue:   normalized1,
			NormalizedScanned: normalized2,
			Similarity:        100.0,
			IsMatch:           true,
			Reason:            "Both titles empty",
		}
	}

	distance := levenshteinDistance(normalized1, normalized2)
	charSimilarity := (1.0 - float64(distance)/float64(maxLen)) * 100.0
	tokenSimilarity := calculateTokenSimilarity(normalized1, normalized2)
	lengthRatio := calculateLengthRatio(normalized1, normalized2)

	// Weighted final score (used as fallback)
	finalScore := (0.6 * charSimilarity) + (0.3 * tokenSimilarity) + (0.1 * lengthRatio)

	result := TitleMatchResult{
		QueueTitle:        queueTitle,
		ScannedTitle:      scannedTitle,
		NormalizedQueue:   normalized1,
		NormalizedScanned: normalized2,
		Similarity:        finalScore,
	}

	// HYBRID LOGIC:
	// 1. If character similarity is very high (≥90%), accept regardless of token score
	//    (handles hyphenation, punctuation differences: "Spider-Man" vs "Spiderman")
	if charSimilarity >= 90.0 {
		result.IsMatch = true
		result.Reason = fmt.Sprintf("Strong character match (%.1f%% char, %.1f%% final)", charSimilarity, finalScore)
		return result
	}

	// 2. If token similarity is very high (≥80%), accept regardless of character score
	//    (handles word order, article differences: "The Matrix" vs "Matrix")
	if tokenSimilarity >= 80.0 {
		result.IsMatch = true
		result.Reason = fmt.Sprintf("Strong token match (%.1f%% token, %.1f%% final)", tokenSimilarity, finalScore)
		return result
	}

	// 3. Otherwise, require weighted score ≥85%
	if finalScore >= 95.0 {
		result.IsMatch = true
		result.Reason = fmt.Sprintf("Strong match (%.1f%% similar)", finalScore)
	} else if finalScore >= 85.0 {
		result.IsMatch = true
		result.Reason = fmt.Sprintf("Acceptable match (%.1f%% similar)", finalScore)
	} else {
		result.IsMatch = false
		result.Reason = fmt.Sprintf("Title mismatch (%.1f%% similar, %.1f%% char, %.1f%% token)",
			finalScore, charSimilarity, tokenSimilarity)
	}

	return result
}
