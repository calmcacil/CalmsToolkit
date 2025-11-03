package mediarequests

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/api"
)

// APIClient wraps the generic API client for Media Requests operations
type APIClient struct {
	client *api.Client
}

// NewAPIClient creates a new Media Requests API client
func NewAPIClient(baseURL, token string, timeout int) *APIClient {
	return &APIClient{
		client: api.NewClient(baseURL, token, 0),
	}
}

// SearchMedia searches for media by query
func (c *APIClient) SearchMedia(query string) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("query", query)
	endpoint := "/search?" + params.Encode()

	resp, err := c.client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	return searchResp.Results, nil
}

// GetTVDetails gets TV show details by TMDB ID
func (c *APIClient) GetTVDetails(tmdbID int) (*TVDetails, error) {
	endpoint := fmt.Sprintf("/tv/%d", tmdbID)
	resp, err := c.client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get TV details: status %d", resp.StatusCode)
	}

	var details TVDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, err
	}

	return &details, nil
}

// CreateRequest submits a new media request
func (c *APIClient) CreateRequest(media SearchResult, seasons interface{}, overrides *RequestOverrides) (*MediaRequest, error) {
	reqData := CreateRequest{
		MediaType: media.MediaType,
		MediaID:   media.ID,
	}

	if media.MediaType == "tv" && seasons != nil {
		reqData.Seasons = seasons
	}

	if overrides != nil {
		if overrides.ServerID > 0 {
			reqData.ServerID = overrides.ServerID
		}
		if overrides.RootFolder != "" {
			reqData.RootFolder = overrides.RootFolder
		}
	}

	resp, err := c.client.Post("/request", reqData)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request creation failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var request MediaRequest
	if err := json.NewDecoder(resp.Body).Decode(&request); err != nil {
		return nil, err
	}

	return &request, nil
}

// GetPendingRequests retrieves all pending requests
func (c *APIClient) GetPendingRequests() ([]MediaRequest, error) {
	const pageSize = 50
	skip := 0
	var pending []MediaRequest

	for {
		endpoint := fmt.Sprintf("/request?filter=pending&take=%d&skip=%d", pageSize, skip)

		resp, err := c.client.Get(endpoint)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get requests: status %d - %s", resp.StatusCode, string(bodyBytes))
		}

		var reqResp RequestsResponse
		if err := json.NewDecoder(resp.Body).Decode(&reqResp); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		pending = append(pending, reqResp.Results...)

		skip += pageSize
		if skip >= reqResp.PageInfo.Results || len(reqResp.Results) == 0 {
			break
		}
	}

	return pending, nil
}

// ApproveRequest approves a request by ID
func (c *APIClient) ApproveRequest(requestID int) error {
	endpoint := fmt.Sprintf("/request/%d/approve", requestID)
	resp, err := c.client.Post(endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("approval failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// ApproveRequestWithOverrides approves a request and optionally updates it with rootFolder override
func (c *APIClient) ApproveRequestWithOverrides(requestID int, overrides *RequestOverrides) error {
	// First, approve the request
	if err := c.ApproveRequest(requestID); err != nil {
		return err
	}

	// If no overrides or no rootFolder specified, we're done
	if overrides == nil || overrides.RootFolder == "" {
		return nil
	}

	// Update the request with the rootFolder override
	updateData := map[string]interface{}{
		"rootFolder": overrides.RootFolder,
	}

	endpoint := fmt.Sprintf("/request/%d", requestID)
	resp, err := c.client.Put(endpoint, updateData)
	if err != nil {
		return fmt.Errorf("approved but failed to update root folder: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("approved but root folder update failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeclineRequest declines a request by ID
func (c *APIClient) DeclineRequest(requestID int) error {
	endpoint := fmt.Sprintf("/request/%d/decline", requestID)
	resp, err := c.client.Post(endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("decline failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// FetchServiceInstances fetches service instances (Sonarr/Radarr)
func (c *APIClient) FetchServiceInstances(service string) ([]ServiceInstance, error) {
	var endpoint string
	switch service {
	case "radarr":
		endpoint = "/service/radarr"
	case "sonarr":
		endpoint = "/service/sonarr"
	default:
		return nil, fmt.Errorf("unsupported service type: %s", service)
	}

	resp, err := c.client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s servers: status %d", service, resp.StatusCode)
	}

	var servers []ServiceInstance
	if err := json.NewDecoder(resp.Body).Decode(&servers); err != nil {
		return nil, err
	}

	return servers, nil
}

// FetchServiceDetails fetches service details by ID
func (c *APIClient) FetchServiceDetails(service string, id int) (*ServiceDetails, error) {
	if service != "radarr" && service != "sonarr" {
		return nil, fmt.Errorf("unsupported service type: %s", service)
	}

	endpoint := fmt.Sprintf("/service/%s/%d", service, id)
	resp, err := c.client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch %s details: status %d", service, resp.StatusCode)
	}

	var details ServiceDetails
	if err := json.NewDecoder(resp.Body).Decode(&details); err != nil {
		return nil, err
	}

	return &details, nil
}

// TestConnection tests the API connection
func (c *APIClient) TestConnection() error {
	return c.client.TestConnection()
}

// Helper functions

// GetYear extracts year from release date or first air date
func GetYear(result SearchResult) string {
	if result.ReleaseDate != "" && len(result.ReleaseDate) >= 4 {
		return result.ReleaseDate[:4]
	}
	if result.FirstAirDate != "" && len(result.FirstAirDate) >= 4 {
		return result.FirstAirDate[:4]
	}
	return ""
}

// GetTitle returns the title or name from a search result
func GetTitle(result SearchResult) string {
	if result.Title != "" {
		return result.Title
	}
	return result.Name
}

// GetStatusText returns human-readable status text
func GetStatusText(status int) string {
	switch status {
	case StatusPending:
		return "Pending Approval"
	case StatusApproved:
		return "Approved"
	case StatusDeclined:
		return "Declined"
	default:
		return "Unknown"
	}
}

// ParseSeasonInput parses season input from user string
func ParseSeasonInput(input string, maxSeasons int) ([]int, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("no seasons specified")
	}

	parts := strings.Split(input, ",")
	seasons := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		season, err := strconv.Atoi(part)
		if err != nil || season < 1 || season > maxSeasons {
			return nil, fmt.Errorf("invalid season number: %s", part)
		}
		seasons = append(seasons, season)
	}

	if len(seasons) == 0 {
		return nil, fmt.Errorf("no valid seasons specified")
	}

	return seasons, nil
}
