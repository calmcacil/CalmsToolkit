//go:build queueremediation

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"strings"
	"time"
)

type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

type QueueItem struct {
	ID                    int             `json:"id"`
	Title                 string          `json:"title"`
	Status                string          `json:"status"`
	TrackedDownloadState  string          `json:"trackedDownloadState"`
	TrackedDownloadStatus string          `json:"trackedDownloadStatus"`
	StatusMessages        []StatusMessage `json:"statusMessages"`
	DownloadClient        string          `json:"downloadClient"`
	DownloadId            string          `json:"downloadId"`
	OutputPath            string          `json:"outputPath"`
	ErrorMessage          string          `json:"errorMessage"`
	InstanceURL           string          `json:"-"`
	InstanceType          string          `json:"-"`
	SeriesID              int             `json:"seriesId,omitempty"`
	MovieID               int             `json:"movieId,omitempty"`
}

type QueueResponse struct {
	Page         int         `json:"page"`
	PageSize     int         `json:"pageSize"`
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

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

type SeriesResource struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

type MovieResource struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

type EpisodeResource struct {
	ID            int    `json:"id"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Title         string `json:"title"`
}

type QualityModel struct {
	Quality  QualityDefinition `json:"quality"`
	Revision RevisionModel     `json:"revision"`
}

type QualityDefinition struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Resolution int    `json:"resolution"`
}

type RevisionModel struct {
	Version  int  `json:"version"`
	Real     int  `json:"real"`
	IsRepack bool `json:"isRepack"`
}

type Language struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ImportRejection struct {
	Reason string `json:"reason"`
	Type   string `json:"type"`
}

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
}

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

func mapStatusToAction(item QueueItem) (action string, blocklist bool, manualImport bool) {
	if item.Status == "failed" {
		return "delete", false, false
	}

	reason, shouldBlocklist := parseStatusMessages(item.StatusMessages)

	switch reason {
	case "custom_format_no_upgrade":
		return "delete", shouldBlocklist, false
	case "quality_no_upgrade":
		return "delete", shouldBlocklist, false
	case "no_files_found":
		return "delete", shouldBlocklist, false
	case "sample_file":
		return "delete", shouldBlocklist, false
	case "matched_by_id":
		return "manual_import", false, true
	}

	if item.TrackedDownloadState == "importBlocked" {
		return "manual_import", false, true
	}

	return "monitor", false, false
}

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

			if strings.Contains(msgLower, "quality revision") {
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

func fetchQueue(config Config, url string, token string) ([]QueueItem, error) {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	endpoint := fmt.Sprintf("%s/api/v3/queue?pageSize=100", url)

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

	return queueResp.Records, nil
}

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

func triggerManualImport(config Config, url string, token string, downloadPath string, instanceType string, useRestAPI bool, queueItem QueueItem) error {
	if useRestAPI {
		if config.Verbose {
			log.Printf("[INFO] Using REST API for manual import (queue item #%d: %s)", queueItem.ID, queueItem.Title)
		}

		scannedItems, err := scanForManualImport(config, url, token, downloadPath)
		if err != nil {
			if config.Verbose {
				log.Printf("[VERBOSE] REST API scan failed, falling back to Command API: %v", err)
			}
			fmt.Fprintf(os.Stderr, "[WARN] REST API scan failed, falling back to Command API: %v\n", err)
		} else {
			importRequests := buildManualImportRequests(config, scannedItems, queueItem, instanceType)
			if len(importRequests) > 0 {
				if err := executeManualImport(config, url, token, importRequests); err != nil {
					if config.Verbose {
						log.Printf("[VERBOSE] REST API import failed, falling back to Command API: %v", err)
					}
					fmt.Fprintf(os.Stderr, "[WARN] REST API import failed, falling back to Command API: %v\n", err)
				} else {
					return nil
				}
			} else {
				if config.Verbose {
					log.Printf("[VERBOSE] No importable files found in scan results, falling back to Command API")
				}
			}
		}
	} else {
		if config.Verbose {
			log.Printf("[INFO] Using Command API for manual import (queue item #%d: %s)", queueItem.ID, queueItem.Title)
		}
	}

	client := &http.Client{
		Timeout: config.Timeout,
	}

	var commandName string
	if instanceType == "sonarr" {
		commandName = "DownloadedEpisodesScan"
	} else {
		commandName = "DownloadedMoviesScan"
	}

	commandData := map[string]interface{}{
		"name": commandName,
		"path": downloadPath,
	}

	jsonData, err := json.Marshal(commandData)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/api/v3/command", url)

	if config.Verbose {
		log.Printf("[VERBOSE] Command API: POST %s (command=%s, path=%s)", sanitizeURL(endpoint), commandName, downloadPath)
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

func scanForManualImport(config Config, url, token, folderPath string) ([]ManualImportResource, error) {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	endpoint := fmt.Sprintf("%s/api/v3/manualimport?folder=%s&filterExistingFiles=true", url, neturl.QueryEscape(folderPath))

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

func executeManualImport(config Config, url, token string, importRequests []ManualImportRequest) error {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	jsonData, err := json.Marshal(importRequests)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/api/v3/manualimport", url)

	if config.Verbose {
		log.Printf("[VERBOSE] Executing manual import: POST %s (%d files)", sanitizeURL(endpoint), len(importRequests))
	}

	if config.Debug {
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, jsonData, "", "  "); err == nil {
			log.Printf("[DEBUG] Import request payload:\n%s", prettyJSON.String())
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
		log.Printf("[ERROR] Import failed: status %d - %s", resp.StatusCode, string(bodyBytes))
		return fmt.Errorf("failed to execute manual import: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	if config.Verbose {
		log.Printf("[VERBOSE] Import response: status %d", resp.StatusCode)
	}

	if config.Debug {
		log.Printf("[DEBUG] Import response body: %s", string(bodyBytes))
	}

	// Parse response to detect partial failures
	var results []ManualImportResource
	if err := json.Unmarshal(bodyBytes, &results); err != nil {
		log.Printf("[WARN] Manual import succeeded but couldn't parse response: %v", err)
		log.Printf("[INFO] Successfully imported %d file(s) via REST API", len(importRequests))
		return nil
	}

	// Analyze results for rejections
	successCount := 0
	failedCount := 0
	var failedFiles []string

	for _, result := range results {
		if len(result.Rejections) > 0 {
			failedCount++
			failedFiles = append(failedFiles, result.Path)
			if config.Verbose {
				log.Printf("[VERBOSE] Import failed for %s: %s", result.Name, formatRejections(result.Rejections))
			}
		} else {
			successCount++
		}
	}

	totalCount := len(results)

	// Log summary based on results
	if failedCount == 0 {
		log.Printf("[INFO] Successfully imported %d/%d files via REST API", successCount, totalCount)
	} else if successCount == 0 {
		// All files failed - return error
		log.Printf("[ERROR] Failed to import all %d files", failedCount)
		return fmt.Errorf("all %d files failed import validation", failedCount)
	} else {
		// Partial failure
		log.Printf("[WARN] Partial import: %d/%d files succeeded, %d failed", successCount, totalCount, failedCount)
		if config.Verbose {
			for _, file := range failedFiles {
				log.Printf("[VERBOSE]   Failed: %s", file)
			}
		}
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

			req := ManualImportRequest{
				Path:         item.Path,
				SeriesID:     item.Series.ID,
				Quality:      item.Quality,
				Languages:    item.Languages,
				ReleaseGroup: item.ReleaseGroup,
				DownloadID:   queueItem.DownloadId,
				IndexerFlags: item.IndexerFlags,
			}

			if item.SeasonNumber != nil {
				req.SeasonNumber = *item.SeasonNumber
			}

			for _, ep := range item.Episodes {
				req.EpisodeIDs = append(req.EpisodeIDs, ep.ID)
			}

			if config.Verbose {
				log.Printf("[VERBOSE] Accepted file %s: Series=%s, Season=%v, Episodes=%d, Quality=%s",
					item.Path, item.Series.Title, item.SeasonNumber, len(item.Episodes), item.Quality.Quality.Name)
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

			req := ManualImportRequest{
				Path:         item.Path,
				MovieID:      item.Movie.ID,
				Quality:      item.Quality,
				Languages:    item.Languages,
				ReleaseGroup: item.ReleaseGroup,
				DownloadID:   queueItem.DownloadId,
				IndexerFlags: item.IndexerFlags,
			}

			if config.Verbose {
				log.Printf("[VERBOSE] Accepted file %s: Movie=%s (%d), Quality=%s",
					item.Path, item.Movie.Title, item.Movie.Year, item.Quality.Quality.Name)
			}

			requests = append(requests, req)
		}
	}

	if config.Verbose {
		log.Printf("[VERBOSE] Built %d import request(s) from %d scanned file(s)", len(requests), len(scannedItems))
	}

	return requests
}

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
		action, blocklist, manualImport := mapStatusToAction(item)

		if dryRun {
			instanceName := getInstanceName(config, item.InstanceURL, item.InstanceType)
			reason := getReason(item)
			fmt.Printf("[DRY-RUN] %s - Item #%d (%s)\n", instanceName, item.ID, item.Title)

			switch action {
			case "delete":
				fmt.Printf("[DRY-RUN]   → Would DELETE (blocklist=%v) - %s\n\n", blocklist, reason)
				deleteCount++
			case "manual_import":
				if item.OutputPath == "" {
					fmt.Printf("[DRY-RUN]   → Would MANUAL_IMPORT (no output path available!) - %s\n\n", reason)
				} else {
					fmt.Printf("[DRY-RUN]   → Would MANUAL_IMPORT to %s - %s\n\n", item.OutputPath, reason)
				}
				manualImportCount++
			case "monitor":
				fmt.Printf("[DRY-RUN]   → MONITORING - %s\n\n", reason)
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
					return err
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
					return err
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

func main() {
	var (
		sonarrURLs   = flag.String("sonarr-urls", "", "Comma-separated Sonarr URLs")
		sonarrTokens = flag.String("sonarr-tokens", "", "Comma-separated Sonarr API tokens")
		radarrURLs   = flag.String("radarr-urls", "", "Comma-separated Radarr URLs")
		radarrTokens = flag.String("radarr-tokens", "", "Comma-separated Radarr API tokens")
		timeout      = flag.Duration("timeout", 30*time.Second, "HTTP request timeout")
		dryRun       = flag.Bool("dry-run", false, "Show what would be done without making changes")
		useRestAPI   = flag.Bool("use-rest-api", false, "Use REST API for manual imports instead of Command API")
		verbose      = flag.Bool("verbose", false, "Show verbose logging (API calls, filtering decisions)")
		debug        = flag.Bool("debug", false, "Show debug logging (full request/response payloads, implies -verbose)")
	)
	flag.Parse()

	// Debug implies verbose
	verboseMode := *verbose || *debug

	// Configure log output to stderr with no prefix (we add our own [LEVEL] tags)
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags)

	config := loadConfig(*sonarrURLs, *sonarrTokens, *radarrURLs, *radarrTokens, *timeout, *useRestAPI, verboseMode, *debug)

	if err := validateQueueConfig(config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please set SONARR_URLS/SONARR_TOKENS or RADARR_URLS/RADARR_TOKENS environment variables\n")
		fmt.Fprintf(os.Stderr, "Or use -sonarr-urls/-sonarr-tokens or -radarr-urls/-radarr-tokens flags\n")
		os.Exit(1)
	}

	if err := classifyAndRemediate(config, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

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
