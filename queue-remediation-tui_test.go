//go:build queueremediation && manual

package main

import (
	"testing"
	"time"
)

func TestTUIModel(t *testing.T) {
	config := Config{
		SonarrURLs:   []string{"http://localhost:8989"},
		SonarrTokens: []string{"test-token"},
		Timeout:      30 * time.Second,
		UseRestAPI:   false,
		Verbose:      false,
		Debug:        false,
	}

	model := InitialModel(config)

	if model.loading != true {
		t.Errorf("Expected initial model to be in loading state")
	}

	if model.currentIndex != 0 {
		t.Errorf("Expected initial current index to be 0, got %d", model.currentIndex)
	}

	if len(model.items) != 0 {
		t.Errorf("Expected initial items to be empty, got %d", len(model.items))
	}
}

func TestFilterItems(t *testing.T) {
	items := []QueueItem{
		{
			ID:           1,
			Title:        "Normal Item",
			Status:       "completed",
			OutputPath:   "/downloads/show.mkv",
			InstanceType: "sonarr",
		},
		{
			ID:           2,
			Title:        "Torrent Item",
			Status:       "completed",
			OutputPath:   "/torrents/show.mkv",
			InstanceType: "sonarr",
		},
		{
			ID:           3,
			Title:        "Failed Item",
			Status:       "failed",
			OutputPath:   "/downloads/failed.mkv",
			InstanceType: "sonarr",
		},
	}

	filtered := filterItems(items)

	// Should skip the /torrents/ item and the normal item, but include the failed item
	if len(filtered) != 1 {
		t.Errorf("Expected 1 item after filtering, got %d", len(filtered))
	}

	// Check that only the failed item remains
	if len(filtered) == 1 && filtered[0].ID != 3 {
		t.Errorf("Expected failed item (ID=3) to remain, got ID=%d", filtered[0].ID)
	}
}
