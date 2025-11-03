package calendar

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/api"
)

// APIClient handles calendar API operations
type APIClient struct {
	sonarrClients []*api.Client
	radarrClients []*api.Client
	debug         bool
}

// NewAPIClient creates a new calendar API client
func NewAPIClient(sonarrURLs, sonarrTokens, radarrURLs, radarrTokens []string, timeout time.Duration, debug bool) *APIClient {
	client := &APIClient{
		debug: debug,
	}

	// Create Sonarr clients
	for i, url := range sonarrURLs {
		if i < len(sonarrTokens) {
			client.sonarrClients = append(client.sonarrClients, api.NewClient(url, sonarrTokens[i], timeout))
		}
	}

	// Create Radarr clients
	for i, url := range radarrURLs {
		if i < len(radarrTokens) {
			client.radarrClients = append(client.radarrClients, api.NewClient(url, radarrTokens[i], timeout))
		}
	}

	return client
}

// FetchCalendar fetches calendar items from all configured instances
func (c *APIClient) FetchCalendar(dateRange DateRange) ([]CalendarItem, []QueueIssue, error) {
	items := make([]CalendarItem, 0)
	queueIssues := make([]QueueIssue, 0)

	// Track seen items for deduplication
	seenEpisodes := make(map[string]bool)
	seenMovies := make(map[string]bool)

	// Fetch from Sonarr instances
	for i, client := range c.sonarrClients {
		episodes, err := c.fetchSonarrCalendar(client, dateRange)
		if err != nil {
			continue // Skip failed instances, continue with others
		}

		instanceName := fmt.Sprintf("Sonarr-%d", i+1)

		for _, ep := range episodes {
			if ep.Series == nil {
				continue
			}

			// Deduplicate by series title + season + episode
			key := fmt.Sprintf("%s-S%02dE%02d", ep.Series.Title, ep.SeasonNumber, ep.EpisodeNumber)
			if seenEpisodes[key] {
				continue
			}
			seenEpisodes[key] = true

			// Detect premiere
			isPremiere := ep.SeasonNumber == 1 && ep.EpisodeNumber == 1

			item := CalendarItem{
				Type:           "episode",
				Title:          ep.Title,
				ShowTitle:      ep.Series.Title,
				Year:           ep.Series.Year,
				Season:         ep.SeasonNumber,
				Episode:        ep.EpisodeNumber,
				AirTime:        ep.AirDateUtc,
				HasFile:        ep.HasFile,
				Monitored:      ep.Monitored,
				IsPremiere:     isPremiere,
				SourceInstance: instanceName,
			}
			items = append(items, item)
		}

		// Check queue for issues
		if queue, err := c.fetchQueue(client); err == nil && queue != nil {
			errorCount := 0
			for _, item := range queue.Records {
				if item.TrackedState == "importFailed" || item.TrackedState == "importPending" ||
					len(item.StatusMessages) > 0 {
					errorCount++
				}
			}
			if errorCount > 0 {
				queueIssues = append(queueIssues, QueueIssue{
					ServiceName: instanceName,
					URL:         client.BaseURL + "/activity/queue",
					Count:       errorCount,
				})
			}
		}
	}

	// Fetch from Radarr instances
	for i, client := range c.radarrClients {
		movies, err := c.fetchRadarrCalendar(client, dateRange)
		if err != nil {
			continue // Skip failed instances, continue with others
		}

		instanceName := fmt.Sprintf("Radarr-%d", i+1)

		for _, movie := range movies {
			// Deduplicate by title + year
			key := fmt.Sprintf("%s-%d", movie.Title, movie.Year)
			if seenMovies[key] {
				continue
			}
			seenMovies[key] = true

			// Determine release date (prefer digital > physical > cinema)
			var releaseTime time.Time
			if movie.DigitalDate != "" {
				releaseTime, _ = time.Parse("2006-01-02T15:04:05Z", movie.DigitalDate)
			} else if movie.ReleaseDate != "" {
				releaseTime, _ = time.Parse("2006-01-02T15:04:05Z", movie.ReleaseDate)
			} else if movie.InCinemas != "" {
				releaseTime, _ = time.Parse("2006-01-02T15:04:05Z", movie.InCinemas)
			}

			if releaseTime.IsZero() {
				continue
			}

			item := CalendarItem{
				Type:           "movie",
				Title:          movie.Title,
				Year:           movie.Year,
				AirTime:        releaseTime,
				HasFile:        movie.HasFile,
				Monitored:      movie.Monitored,
				IsPremiere:     false,
				SourceInstance: instanceName,
			}
			items = append(items, item)
		}

		// Check queue for issues
		if queue, err := c.fetchQueue(client); err == nil && queue != nil {
			errorCount := 0
			for _, item := range queue.Records {
				if item.TrackedState == "importFailed" || item.TrackedState == "importPending" ||
					len(item.StatusMessages) > 0 {
					errorCount++
				}
			}
			if errorCount > 0 {
				queueIssues = append(queueIssues, QueueIssue{
					ServiceName: instanceName,
					URL:         client.BaseURL + "/activity/queue",
					Count:       errorCount,
				})
			}
		}
	}

	return items, queueIssues, nil
}

func (c *APIClient) fetchSonarrCalendar(client *api.Client, dateRange DateRange) ([]SonarrEpisode, error) {
	apiURL := fmt.Sprintf("/api/v3/calendar?start=%s&end=%s&includeSeries=true",
		dateRange.Start.Format("2006-01-02"), dateRange.End.Format("2006-01-02"))

	if c.debug {
		fmt.Printf("DEBUG: Fetching Sonarr calendar from: %s\n", client.BaseURL+apiURL)
	}

	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Sonarr calendar: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var episodes []SonarrEpisode
	if err := json.Unmarshal(body, &episodes); err != nil {
		return nil, err
	}

	return episodes, nil
}

func (c *APIClient) fetchRadarrCalendar(client *api.Client, dateRange DateRange) ([]RadarrMovie, error) {
	apiURL := fmt.Sprintf("/api/v3/calendar?start=%s&end=%s",
		dateRange.Start.Format("2006-01-02"), dateRange.End.Format("2006-01-02"))

	if c.debug {
		fmt.Printf("DEBUG: Fetching Radarr calendar from: %s\n", client.BaseURL+apiURL)
	}

	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Radarr calendar: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var movies []RadarrMovie
	if err := json.Unmarshal(body, &movies); err != nil {
		return nil, err
	}

	return movies, nil
}

func (c *APIClient) fetchQueue(client *api.Client) (*QueueResponse, error) {
	apiURL := "/api/v3/queue"

	if c.debug {
		fmt.Printf("DEBUG: Fetching queue from: %s\n", client.BaseURL+apiURL)
	}

	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch queue: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var queue QueueResponse
	if err := json.Unmarshal(body, &queue); err != nil {
		return nil, err
	}

	return &queue, nil
}
