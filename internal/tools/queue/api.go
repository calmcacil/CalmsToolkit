package queue

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/api"
	"github.com/calmcacil/CalmsToolkit/internal/config"
)

// Client represents the queue API client
type Client struct {
	config *config.Config
}

// NewClient creates a new queue API client
func NewClient(config *config.Config) *Client {
	return &Client{config: config}
}

// FetchAllQueues fetches queue items from all configured instances
func (c *Client) FetchAllQueues() ([]QueueItem, error) {
	var allItems []QueueItem
	var successCount int
	var failedInstances int
	var totalInstances int

	// Fetch from Sonarr instances
	for _, instance := range c.config.Sonarr {
		totalInstances++
		items, err := c.fetchQueue(instance.URL, instance.Token)
		if err != nil {
			failedInstances++
			continue
		}
		successCount++
		for _, item := range items {
			item.InstanceURL = instance.URL
			item.InstanceType = "sonarr"
			item.InstanceName = c.config.GetInstanceName(instance.URL, "sonarr")
			allItems = append(allItems, item)
		}
	}

	// Fetch from Radarr instances
	for _, instance := range c.config.Radarr {
		totalInstances++
		items, err := c.fetchQueue(instance.URL, instance.Token)
		if err != nil {
			failedInstances++
			continue
		}
		successCount++
		for _, item := range items {
			item.InstanceURL = instance.URL
			item.InstanceType = "radarr"
			item.InstanceName = c.config.GetInstanceName(instance.URL, "radarr")
			allItems = append(allItems, item)
		}
	}

	if totalInstances > 0 && successCount == 0 {
		return nil, fmt.Errorf("all instances failed to fetch queue")
	}

	return allItems, nil
}

// fetchQueue fetches queue from a single instance
func (c *Client) fetchQueue(baseURL, token string) ([]QueueItem, error) {
	client := api.NewClient(baseURL, token, c.config.Global.Timeout)

	resp, err := client.Get("/api/v3/queue?pageSize=100")
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

// DeleteQueueItem deletes a queue item
func (c *Client) DeleteQueueItem(item QueueItem, removeFromClient bool, blocklist bool) error {
	client := api.NewClient(item.InstanceURL, "", c.config.Global.Timeout)

	// Get token for this instance
	token, err := c.getTokenForInstance(item.InstanceURL, item.InstanceType)
	if err != nil {
		return err
	}
	client.Token = token

	endpoint := fmt.Sprintf("/api/v3/queue/%d", item.ID)
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

	resp, err := client.Delete(endpoint)
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

// TriggerManualImport triggers manual import for a download path
func (c *Client) TriggerManualImport(item QueueItem) error {
	client := api.NewClient(item.InstanceURL, "", c.config.Global.Timeout)

	// Get token for this instance
	token, err := c.getTokenForInstance(item.InstanceURL, item.InstanceType)
	if err != nil {
		return err
	}
	client.Token = token

	var commandName string
	if item.InstanceType == "sonarr" {
		commandName = "DownloadedEpisodesScan"
	} else {
		commandName = "DownloadedMoviesScan"
	}

	commandData := map[string]interface{}{
		"name": commandName,
		"path": item.OutputPath,
	}

	jsonData, err := json.Marshal(commandData)
	if err != nil {
		return err
	}

	resp, err := client.Post("/api/v3/command", jsonData)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("failed to trigger manual import: status %d (error reading response body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("failed to trigger manual import: status %d - %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// getTokenForInstance retrieves the API token for a specific instance
func (c *Client) getTokenForInstance(instanceURL, instanceType string) (string, error) {
	instances := c.config.Sonarr
	if instanceType == "radarr" {
		instances = c.config.Radarr
	}

	for _, instance := range instances {
		if instance.URL == instanceURL {
			return instance.Token, nil
		}
	}
	return "", fmt.Errorf("no token found for %s instance %s", instanceType, instanceURL)
}
