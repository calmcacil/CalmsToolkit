package requests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
)

func fetchServiceInstances(ctx context.Context, cfg ToolConfig, service string) ([]ServiceInstance, error) {
	var endpoint string
	switch service {
	case "radarr":
		endpoint = "/service/radarr"
	case "sonarr":
		endpoint = "/service/sonarr"
	default:
		return nil, nil
	}

	resp, err := makeRequest(ctx, cfg, "GET", endpoint, nil)
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

func fetchServiceDetails(ctx context.Context, cfg ToolConfig, service string, id int) (*ServiceDetails, error) {
	if service != "radarr" && service != "sonarr" {
		return nil, fmt.Errorf("unsupported service type: %s", service)
	}

	endpoint := fmt.Sprintf("/service/%s/%d", service, id)
	resp, err := makeRequest(ctx, cfg, "GET", endpoint, nil)
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

const maxBodySize = 10 * 1024 * 1024

func readBodyLimited(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxBodySize))
}

func makeRequest(ctx context.Context, cfg ToolConfig, method, endpoint string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	fullURL := cfg.ServerURL + "/api/v1" + endpoint

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: cfg.Timeout}
	return client.Do(req)
}

func searchMedia(ctx context.Context, cfg ToolConfig, query string) ([]SearchResult, error) {
	params := url.Values{}
	params.Set("query", query)
	endpoint := "/search?" + params.Encode()

	resp, err := makeRequest(ctx, cfg, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return nil, fmt.Errorf("search failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, err
	}

	return searchResp.Results, nil
}

func getTVDetails(ctx context.Context, cfg ToolConfig, tmdbID int) (*TVDetails, error) {
	endpoint := fmt.Sprintf("/tv/%d", tmdbID)
	resp, err := makeRequest(ctx, cfg, "GET", endpoint, nil)
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

func createRequest(ctx context.Context, cfg ToolConfig, media SearchResult, seasons interface{}, overrides *RequestOverrides) (*MediaRequest, error) {
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

	resp, err := makeRequest(ctx, cfg, "POST", "/request", reqData)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return nil, fmt.Errorf("request creation failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var request MediaRequest
	if err := json.NewDecoder(resp.Body).Decode(&request); err != nil {
		return nil, err
	}

	return &request, nil
}

func checkUserPermissions(ctx context.Context, cfg ToolConfig) (*AuthMe, error) {
	resp, err := makeRequest(ctx, cfg, "GET", "/auth/me", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return nil, fmt.Errorf("failed to get user info: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var authMe AuthMe
	if err := json.NewDecoder(resp.Body).Decode(&authMe); err != nil {
		return nil, err
	}

	return &authMe, nil
}

func getRequestCount(ctx context.Context, cfg ToolConfig) (*RequestCount, error) {
	resp, err := makeRequest(ctx, cfg, "GET", "/request/count", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return nil, fmt.Errorf("failed to get request count: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var count RequestCount
	if err := json.NewDecoder(resp.Body).Decode(&count); err != nil {
		return nil, err
	}

	return &count, nil
}

func getPendingRequests(ctx context.Context, cfg ToolConfig) ([]MediaRequest, error) {
	p := colors.GetPalette(cfg.Theme)
	var expectedPendingCount int

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "\n=== Diagnostic: Checking pending requests ===\n")

		if authMe, err := checkUserPermissions(ctx, cfg); err == nil {
			fmt.Fprintf(os.Stderr, "User ID: %d\n", authMe.ID)
			fmt.Fprintf(os.Stderr, "User Email: %s\n", authMe.Email)
			fmt.Fprintf(os.Stderr, "User Permissions: %d\n", authMe.Permissions)

			const MANAGE_REQUESTS = 16
			const ADMIN = 2
			if (authMe.Permissions & MANAGE_REQUESTS) != 0 {
				fmt.Fprintf(os.Stderr, "✓ Has MANAGE_REQUESTS permission\n")
			} else if (authMe.Permissions & ADMIN) != 0 {
				fmt.Fprintf(os.Stderr, "✓ Has ADMIN permission\n")
			} else {
				fmt.Fprintf(os.Stderr, "⚠ WARNING: May lack MANAGE_REQUESTS (16) or ADMIN (2) permission\n")
			}
		} else {
			fmt.Fprintf(os.Stderr, "⚠ Failed to check permissions: %v\n", err)
		}
	}

	if count, err := getRequestCount(ctx, cfg); err == nil {
		expectedPendingCount = count.Pending
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Request counts - Pending: %d, Approved: %d, Total: %d\n",
				count.Pending, count.Approved, count.Total)
		}
	} else {
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "⚠ Failed to get request count: %v\n", err)
		}
	}

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "===========================================\n\n")
	}

	const pageSize = 50
	skip := 0
	var pending []MediaRequest

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Attempting primary fetch with filter=pending...\n")
	}

	for {
		endpoint := fmt.Sprintf("/request?filter=pending&take=%d&skip=%d", pageSize, skip)

		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Fetching: %s\n", endpoint)
		}

		resp, err := makeRequest(ctx, cfg, "GET", endpoint, nil)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := readBodyLimited(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("failed to get requests: status %d - %s", resp.StatusCode, string(bodyBytes))
		}

		var reqResp RequestsResponse
		if err := json.NewDecoder(resp.Body).Decode(&reqResp); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Page %d: Got %d results (total: %d)\n",
				reqResp.PageInfo.Page, len(reqResp.Results), reqResp.PageInfo.Results)
		}

		pending = append(pending, reqResp.Results...)

		skip += pageSize
		if skip >= reqResp.PageInfo.Results || len(reqResp.Results) == 0 {
			break
		}
	}

	if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Primary fetch complete: %d pending requests fetched\n", len(pending))
	}

	if expectedPendingCount > 0 && len(pending) == 0 {
		clr := colors.ClrFunc(cfg.NoColor)

		if !cfg.Quiet {
			fmt.Fprintf(os.Stderr, "\n%s⚠ WARNING: Overseerr API bug detected!%s\n", clr(p.Warning), clr(p.Reset))
			fmt.Fprintf(os.Stderr, "Expected %d pending request(s) but filter=pending returned 0 results.\n", expectedPendingCount)
			fmt.Fprintf(os.Stderr, "Activating fallback: fetching all requests and filtering client-side...\n\n")
		}

		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "=== Fallback Mode: Fetching filter=all ===\n")
		}

		skip = 0
		var allRequests []MediaRequest

		for {
			endpoint := fmt.Sprintf("/request?filter=all&take=%d&skip=%d", pageSize, skip)

			if cfg.Verbose {
				fmt.Fprintf(os.Stderr, "Fetching: %s\n", endpoint)
			}

			resp, err := makeRequest(ctx, cfg, "GET", endpoint, nil)
			if err != nil {
				return nil, fmt.Errorf("fallback fetch failed: %w", err)
			}

			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := readBodyLimited(resp.Body)
				resp.Body.Close()
				return nil, fmt.Errorf("fallback fetch failed: status %d - %s", resp.StatusCode, string(bodyBytes))
			}

			var reqResp RequestsResponse
			if err := json.NewDecoder(resp.Body).Decode(&reqResp); err != nil {
				resp.Body.Close()
				return nil, fmt.Errorf("fallback decode failed: %w", err)
			}
			resp.Body.Close()

			if cfg.Verbose {
				fmt.Fprintf(os.Stderr, "Fallback page %d: Got %d results (total: %d)\n",
					reqResp.PageInfo.Page, len(reqResp.Results), reqResp.PageInfo.Results)
			}

			allRequests = append(allRequests, reqResp.Results...)

			skip += pageSize
			if skip >= reqResp.PageInfo.Results || len(reqResp.Results) == 0 {
				break
			}
		}

		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Fallback fetch complete: %d total requests retrieved\n", len(allRequests))
			fmt.Fprintf(os.Stderr, "Filtering for status=%d (PENDING)...\n", StatusPending)
		}

		for _, req := range allRequests {
			if req.Status == StatusPending {
				pending = append(pending, req)
			}
		}

		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Client-side filtering complete: %d pending requests found\n", len(pending))
			fmt.Fprintf(os.Stderr, "===========================================\n\n")
		}

		if !cfg.Quiet {
			fmt.Fprintf(os.Stderr, "%s✓ Fallback successful: Found %d pending request(s)%s\n\n",
				clr(p.Success), len(pending), clr(p.Reset))
		}
	} else if cfg.Verbose {
		fmt.Fprintf(os.Stderr, "Primary fetch successful, no fallback needed.\n\n")
	}

	return pending, nil
}

func approveRequest(ctx context.Context, cfg ToolConfig, requestID int) error {
	endpoint := fmt.Sprintf("/request/%d/approve", requestID)
	resp, err := makeRequest(ctx, cfg, "POST", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return fmt.Errorf("approval failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func approveRequestWithOverrides(ctx context.Context, cfg ToolConfig, requestID int, overrides *RequestOverrides) error {
	if overrides != nil && (overrides.RootFolder != "" || overrides.ServerID > 0) {
		updateData := make(map[string]interface{})
		if overrides.RootFolder != "" {
			updateData["rootFolder"] = overrides.RootFolder
		}
		if overrides.ServerID > 0 {
			updateData["serverId"] = overrides.ServerID
		}

		endpoint := fmt.Sprintf("/request/%d", requestID)
		resp, err := makeRequest(ctx, cfg, "PUT", endpoint, updateData)
		if err != nil {
			return fmt.Errorf("failed to set request overrides before approval: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := readBodyLimited(resp.Body)
			return fmt.Errorf("failed to set request overrides before approval: status %d - %s", resp.StatusCode, string(bodyBytes))
		}
	}

	if err := approveRequest(ctx, cfg, requestID); err != nil {
		return err
	}

	return nil
}

func declineRequest(ctx context.Context, cfg ToolConfig, requestID int) error {
	endpoint := fmt.Sprintf("/request/%d/decline", requestID)
	resp, err := makeRequest(ctx, cfg, "POST", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := readBodyLimited(resp.Body)
		return fmt.Errorf("decline failed: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}
