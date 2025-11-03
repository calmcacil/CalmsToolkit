package mediarequests

import (
	"testing"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestInitialModel(t *testing.T) {
	cfg := &config.Config{
		Overseerr: config.OverseerrConfig{
			URL:     "http://localhost:5055",
			Token:   "test-token",
			Timeout: 30,
		},
		NoColor: false,
	}

	model := InitialModel(cfg)

	assert.Equal(t, StepSearch, model.step)
	assert.Equal(t, "", model.query)
	assert.Equal(t, 0, model.selected)
	assert.False(t, model.loading)
	assert.Equal(t, "", model.error)
	assert.NotNil(t, model.searchInput)
	assert.NotNil(t, model.seasonInput)
	assert.NotNil(t, model.api)
	assert.NotNil(t, model.colors)
}

func TestModelUpdate_SearchInput(t *testing.T) {
	cfg := &config.Config{
		Overseerr: config.OverseerrConfig{
			URL:     "http://localhost:5055",
			Token:   "test-token",
			Timeout: 30,
		},
		NoColor: false,
	}

	model := InitialModel(cfg)

	// Test search input by simulating character input
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
	newModel, _ := model.Update(keyMsg)
	assert.Equal(t, "t", newModel.(Model).query)

	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	newModel, _ = newModel.Update(keyMsg)
	assert.Equal(t, "te", newModel.(Model).query)

	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	newModel, _ = newModel.Update(keyMsg)
	assert.Equal(t, "tes", newModel.(Model).query)

	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}}
	newModel, _ = newModel.Update(keyMsg)
	assert.Equal(t, "test", newModel.(Model).query)
}

func TestModelUpdate_Navigation(t *testing.T) {
	cfg := &config.Config{
		Overseerr: config.OverseerrConfig{
			URL:     "http://localhost:5055",
			Token:   "test-token",
			Timeout: 30,
		},
		NoColor: false,
	}

	model := InitialModel(cfg)

	// Set up select step with results
	model.step = StepSelect
	model.results = []SearchResult{
		{ID: 1, Title: "Movie 1"},
		{ID: 2, Title: "Movie 2"},
	}
	model.selected = 1

	// Test navigation up
	keyMsg := tea.KeyMsg{Type: tea.KeyUp}
	newModel, _ := model.Update(keyMsg)

	assert.Equal(t, 0, newModel.(Model).selected)

	// Test navigation down
	keyMsg = tea.KeyMsg{Type: tea.KeyDown}
	newModel, _ = newModel.Update(keyMsg)

	assert.Equal(t, 1, newModel.(Model).selected)
}

func TestModelUpdate_Quit(t *testing.T) {
	cfg := &config.Config{
		Overseerr: config.OverseerrConfig{
			URL:     "http://localhost:5055",
			Token:   "test-token",
			Timeout: 30,
		},
		NoColor: false,
	}

	model := InitialModel(cfg)

	// Test Ctrl+C
	keyMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := model.Update(keyMsg)

	assert.NotNil(t, cmd)

	// Test Esc
	keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
	_, cmd = model.Update(keyMsg)

	assert.NotNil(t, cmd)
}

func TestModelUpdate_WindowSize(t *testing.T) {
	cfg := &config.Config{
		Overseerr: config.OverseerrConfig{
			URL:     "http://localhost:5055",
			Token:   "test-token",
			Timeout: 30,
		},
		NoColor: false,
	}

	model := InitialModel(cfg)

	windowMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, _ := model.Update(windowMsg)

	assert.Equal(t, 100, newModel.(Model).width)
	assert.Equal(t, 50, newModel.(Model).height)
}

func TestModelUpdate_SearchResultsMsg(t *testing.T) {
	cfg := &config.Config{
		Overseerr: config.OverseerrConfig{
			URL:     "http://localhost:5055",
			Token:   "test-token",
			Timeout: 30,
		},
		NoColor: false,
	}

	model := InitialModel(cfg)
	model.loading = true

	// Test successful search results
	results := []SearchResult{
		{ID: 1, Title: "Movie 1"},
		{ID: 2, Title: "Movie 2"},
	}
	msg := SearchResultsMsg{Results: results}

	newModel, _ := model.Update(msg)

	assert.Equal(t, StepSelect, newModel.(Model).step)
	assert.Equal(t, 0, newModel.(Model).selected)
	assert.Equal(t, results, newModel.(Model).results)
	assert.False(t, newModel.(Model).loading)
	assert.Equal(t, "", newModel.(Model).error)

	// Test search error
	model.loading = true
	errorMsg := SearchResultsMsg{Error: assert.AnError}
	newModel, _ = model.Update(errorMsg)

	assert.Equal(t, StepSearch, newModel.(Model).step)
	assert.False(t, newModel.(Model).loading)
	assert.Equal(t, assert.AnError.Error(), newModel.(Model).error)
}

func TestModelUpdate_ConfirmationStep(t *testing.T) {
	cfg := &config.Config{
		Overseerr: config.OverseerrConfig{
			URL:     "http://localhost:5055",
			Token:   "test-token",
			Timeout: 30,
		},
		NoColor: false,
	}

	model := InitialModel(cfg)
	model.step = StepConfirm

	// Set up mock results for testing
	model.results = []SearchResult{
		{
			ID:          1,
			Title:       "Test Movie",
			MediaType:   "movie",
			ReleaseDate: "2023-01-01",
		},
	}
	model.selected = 0

	// Test 'y' key
	keyMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	newModel, cmd := model.Update(keyMsg)

	assert.Equal(t, StepConfirm, newModel.(Model).step)
	assert.True(t, newModel.(Model).loading)
	assert.NotNil(t, cmd)

	// Test 'n' key
	keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	newModel, _ = model.Update(keyMsg)

	assert.Equal(t, StepSelect, newModel.(Model).step)
}

func TestGetTitle(t *testing.T) {
	tests := []struct {
		name     string
		result   SearchResult
		expected string
	}{
		{
			name:     "movie with title",
			result:   SearchResult{Title: "Test Movie"},
			expected: "Test Movie",
		},
		{
			name:     "TV show with name",
			result:   SearchResult{Name: "Test Show"},
			expected: "Test Show",
		},
		{
			name:     "title takes precedence over name",
			result:   SearchResult{Title: "Movie", Name: "Show"},
			expected: "Movie",
		},
		{
			name:     "empty result",
			result:   SearchResult{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetTitle(tt.result))
		})
	}
}

func TestGetYear(t *testing.T) {
	tests := []struct {
		name     string
		result   SearchResult
		expected string
	}{
		{
			name:     "movie with release date",
			result:   SearchResult{ReleaseDate: "2023-01-01"},
			expected: "2023",
		},
		{
			name:     "TV show with first air date",
			result:   SearchResult{FirstAirDate: "2022-05-15"},
			expected: "2022",
		},
		{
			name:     "release date takes precedence",
			result:   SearchResult{ReleaseDate: "2023-01-01", FirstAirDate: "2022-05-15"},
			expected: "2023",
		},
		{
			name:     "no dates",
			result:   SearchResult{},
			expected: "",
		},
		{
			name:     "short date",
			result:   SearchResult{ReleaseDate: "2023"},
			expected: "2023",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetYear(tt.result))
		})
	}
}

func TestGetStatusText(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		expected string
	}{
		{
			name:     "pending status",
			status:   StatusPending,
			expected: "Pending Approval",
		},
		{
			name:     "approved status",
			status:   StatusApproved,
			expected: "Approved",
		},
		{
			name:     "declined status",
			status:   StatusDeclined,
			expected: "Declined",
		},
		{
			name:     "unknown status",
			status:   999,
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetStatusText(tt.status))
		})
	}
}

func TestParseSeasonInput(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		maxSeasons  int
		expected    []int
		expectError bool
	}{
		{
			name:        "single season",
			input:       "1",
			maxSeasons:  5,
			expected:    []int{1},
			expectError: false,
		},
		{
			name:        "multiple seasons",
			input:       "1,2,3",
			maxSeasons:  5,
			expected:    []int{1, 2, 3},
			expectError: false,
		},
		{
			name:        "seasons with spaces",
			input:       "1, 2, 3",
			maxSeasons:  5,
			expected:    []int{1, 2, 3},
			expectError: false,
		},
		{
			name:        "empty input",
			input:       "",
			maxSeasons:  5,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid season number",
			input:       "1,2,999",
			maxSeasons:  5,
			expected:    nil,
			expectError: true,
		},
		{
			name:        "season too high",
			input:       "6",
			maxSeasons:  5,
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseSeasonInput(tt.input, tt.maxSeasons)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestModelHelperMethods(t *testing.T) {
	cfg := &config.Config{
		Overseerr: config.OverseerrConfig{
			URL:     "http://localhost:5055",
			Token:   "test-token",
			Timeout: 30,
		},
		NoColor: false,
	}

	model := InitialModel(cfg)

	// Test SetError and ResetError
	model.SetError(assert.AnError)
	assert.Equal(t, assert.AnError.Error(), model.error)

	model.ResetError()
	assert.Equal(t, "", model.error)

	// Test SetLoading and IsLoading
	model.SetLoading(true)
	assert.True(t, model.loading)
	assert.Equal(t, "", model.error)

	model.SetLoading(false)
	assert.False(t, model.loading)

	// Test GetSelectedMedia with no results
	selected := model.GetSelectedMedia()
	assert.Nil(t, selected)

	// Test GetSelectedMedia with results
	model.results = []SearchResult{
		{ID: 1, Title: "Movie 1"},
		{ID: 2, Title: "Movie 2"},
	}
	model.selected = 1
	selected = model.GetSelectedMedia()
	assert.NotNil(t, selected)
	assert.Equal(t, 2, selected.ID)

	// Test FormatMediaTitle
	result := SearchResult{Title: "Test Movie", ReleaseDate: "2023-01-01"}
	title := model.FormatMediaTitle(result)
	assert.Equal(t, "Test Movie (2023)", title)

	// Test FormatMediaType
	mediaType := model.FormatMediaType("movie")
	assert.Equal(t, "🎬 Movie", mediaType)

	mediaType = model.FormatMediaType("tv")
	assert.Equal(t, "📺 TV Show", mediaType)

	// Test TruncateText
	longText := "This is a very long text that should be truncated"
	truncated := model.TruncateText(longText, 20)
	assert.Equal(t, "This is a very lo...", truncated)

	shortText := "Short text"
	truncated = model.TruncateText(shortText, 20)
	assert.Equal(t, "Short text", truncated)
}
