package airtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/httputil"
)

func TestScoreContainment(t *testing.T) {
	tests := []struct {
		query string
		title string
		year  int
		want  bool
	}{
		{"clarkson", "Clarkson's Farm", 2021, true},
		{"clark", "Clarkson's Farm", 2021, true},
		{"clarkson farm", "Clarkson's Farm", 2021, true},
		{"xyzzy", "Clarkson's Farm", 2021, false},
		{"bob", "Clarkson's Farm", 2021, false},
	}
	for _, tc := range tests {
		c := SeriesOrMovie{Title: tc.title, Year: tc.year, Monitored: true}
		got := scoreCandidate(tc.query, c) > 0
		if got != tc.want {
			t.Errorf("scoreCandidate(%q, %q) > 0 = %v, want %v", tc.query, tc.title, got, tc.want)
		}
	}
}

func TestScoreRanking(t *testing.T) {
	// "clarkson" should score higher for Clarkson's Farm than for "Clarkson's Other Show"
	high := scoreCandidate("clarkson", SeriesOrMovie{Title: "Clarkson's Farm", Year: 2021, Monitored: true})
	low := scoreCandidate("clarkson", SeriesOrMovie{Title: "Bob Clarkson's Other Show", Year: 2020, Monitored: true})
	if high <= low {
		t.Errorf("clarkson's farm (%d) should beat other show (%d)", high, low)
	}
}

func TestScoreExact(t *testing.T) {
	// exact match should score high
	c := SeriesOrMovie{Title: "Clarkson's Farm", Year: 2021, Monitored: true}
	s := scoreCandidate("clarkson's farm", c)
	if s <= 50 {
		t.Errorf("exact match score %d too low", s)
	}
}

func TestScoreWithYear(t *testing.T) {
	// year in query should boost
	withYear := scoreCandidate("clarkson 2021", SeriesOrMovie{Title: "Clarkson's Farm", Year: 2021, Monitored: true})
	withoutYear := scoreCandidate("clarkson", SeriesOrMovie{Title: "Clarkson's Farm", Year: 2021, Monitored: true})
	if withYear <= withoutYear {
		t.Errorf("year boost should increase score: with=%d without=%d", withYear, withoutYear)
	}
}

func TestScoreZeroForNoMatch(t *testing.T) {
	s := scoreCandidate("totally unrelated", SeriesOrMovie{Title: "Clarkson's Farm", Year: 2021})
	if s != 0 {
		t.Errorf("expected score 0, got %d", s)
	}
}

func TestScoreInLibraryBoost(t *testing.T) {
	monitored := scoreCandidate("clarkson", SeriesOrMovie{Title: "Clarkson's Farm", Year: 2021, Monitored: true})
	unmonitored := scoreCandidate("clarkson", SeriesOrMovie{Title: "Clarkson's Farm", Year: 2021, Monitored: false})
	if monitored <= unmonitored {
		t.Errorf("library boost should increase score: monitored=%d unmonitored=%d", monitored, unmonitored)
	}
}

func TestFetchSonarrLibrary(t *testing.T) {
	mockResponse := []SonarrSeries{
		{
			ID:        1,
			Title:     "Test Show",
			Year:      2024,
			TVDBID:    12345,
			Monitored: true,
			Status:    "continuing",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.Header.Get("X-Api-Key") != "test-token" {
			t.Errorf("Expected X-Api-Key: test-token, got %s", r.Header.Get("X-Api-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
	ctx := context.Background()

	series, err := fetchSonarrLibrary(ctx, client, inst, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("Got %d series, want 1", len(series))
	}
	if series[0].Title != "Test Show" {
		t.Errorf("Title = %q, want %q", series[0].Title, "Test Show")
	}
	if !series[0].Monitored {
		t.Error("Expected Monitored = true")
	}
}

func TestFetchRadarrLibrary(t *testing.T) {
	mockResponse := []RadarrMovie{
		{
			ID:        1,
			Title:     "Test Movie",
			Year:      2024,
			TMDBID:    67890,
			HasFile:   true,
			Status:    "released",
			Monitored: true,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET, got %s", r.Method)
		}
		if r.Header.Get("X-Api-Key") != "test-token" {
			t.Errorf("Expected X-Api-Key: test-token, got %s", r.Header.Get("X-Api-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
	ctx := context.Background()

	movies, err := fetchRadarrLibrary(ctx, client, inst, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(movies) != 1 {
		t.Fatalf("Got %d movies, want 1", len(movies))
	}
	if movies[0].Title != "Test Movie" {
		t.Errorf("Title = %q, want %q", movies[0].Title, "Test Movie")
	}
	if !movies[0].HasFile {
		t.Error("Expected HasFile = true")
	}
}

func TestFetchSonarrEpisodes(t *testing.T) {
	now := time.Now()
	mockResponse := []SonarrEpisode{
		{
			ID:            1,
			Title:         "Pilot",
			AirDateUtc:    now.AddDate(0, 0, -1),
			SeasonNumber:  1,
			EpisodeNumber: 1,
			HasFile:       true,
		},
		{
			ID:            2,
			Title:         "Second",
			AirDateUtc:    now.AddDate(0, 0, 7),
			SeasonNumber:  1,
			EpisodeNumber: 2,
			HasFile:       false,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := httputil.NewClient(5 * time.Second)
	inst := config.ArrInstance{Name: "Test", URL: server.URL, APIKey: "test-token"}
	ctx := context.Background()

	episodes, err := fetchSonarrEpisodes(ctx, client, inst, 1, false)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(episodes) != 2 {
		t.Fatalf("Got %d episodes, want 2", len(episodes))
	}
}

func TestGroupEpisodesBySeason(t *testing.T) {
	eps := []SonarrEpisode{
		{SeasonNumber: 1, EpisodeNumber: 1},
		{SeasonNumber: 1, EpisodeNumber: 2},
		{SeasonNumber: 2, EpisodeNumber: 3},
	}
	m := groupEpisodesBySeason(eps)
	if len(m) != 2 {
		t.Errorf("got %d seasons, want 2", len(m))
	}
	if len(m[1]) != 2 {
		t.Errorf("season 1 has %d episodes, want 2", len(m[1]))
	}
	if len(m[2]) != 1 {
		t.Errorf("season 2 has %d episodes, want 1", len(m[2]))
	}
}

func TestCountOnDisk(t *testing.T) {
	eps := []SonarrEpisode{
		{HasFile: true},
		{HasFile: false},
		{HasFile: true},
	}
	if n := countOnDisk(eps); n != 2 {
		t.Errorf("got %d, want 2", n)
	}
}

func TestResolveMovieStatus(t *testing.T) {
	tests := []struct {
		status  string
		hasFile bool
		want    string
	}{
		{"released", true, "released"},
		{"inCinemas", false, "announced"},
		{"announced", false, "announced"},
	}
	for _, tc := range tests {
		cfg := ToolConfig{}
		c := SeriesOrMovie{Status: tc.status, HasFile: tc.hasFile}
		got := resolveMovieStatus(c, cfg)
		if got != tc.want {
			t.Errorf("resolveMovieStatus(%q, %v) = %q, want %q", tc.status, tc.hasFile, got, tc.want)
		}
	}
}

func TestParseDateFlexible(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"2024-12-25T21:00:00Z", true},
		{"2024-12-25", true},
		{"not-a-date", false},
	}
	for _, tc := range tests {
		_, err := parseDateFlexible(tc.input)
		got := err == nil
		if got != tc.want {
			t.Errorf("parseDateFlexible(%q) ok=%v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestFindCurrentSeason(t *testing.T) {
	now := time.Now()
	seasons := episodeBySeason{
		1: {
			{AirDateUtc: now.AddDate(0, -6, 0)}, // 6 months ago
			{AirDateUtc: now.AddDate(0, -6, 1)},
		},
		2: {
			{AirDateUtc: now.AddDate(0, 0, -3)}, // 3 days ago
			{AirDateUtc: now.AddDate(0, 0, 10)}, // 10 days from now
		},
	}
	sn := findCurrentSeason(seasons)
	if sn != 2 {
		t.Errorf("expected season 2 (closest to now), got %d", sn)
	}
}

func TestGetMovieReleaseTime(t *testing.T) {
	m := RadarrMovie{
		DigitalRelease:  "2024-12-25",
		PhysicalRelease: "2025-01-15",
		InCinemas:       "2024-12-20",
	}
	rt := getMovieReleaseTime(m)
	if rt.IsZero() {
		t.Fatal("expected non-zero release time")
	}
	// DigitalRelease should be preferred
	expected, _ := parseDateFlexible("2024-12-25")
	if !rt.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, rt)
	}
}

func TestGetMovieReleaseTimeFallback(t *testing.T) {
	m := RadarrMovie{
		InCinemas: "2024-12-20",
	}
	rt := getMovieReleaseTime(m)
	if rt.IsZero() {
		t.Fatal("expected non-zero release time from inCinemas fallback")
	}
}

func TestTokenise(t *testing.T) {
	tokens := tokenise("Clarkson's Farm 2021")
	expected := []string{"clarkson", "s", "farm", "2021"}
	if len(tokens) != len(expected) {
		t.Fatalf("got %d tokens, want %d: %v", len(tokens), len(expected), tokens)
	}
	for i := range expected {
		if tokens[i] != expected[i] {
			t.Errorf("token[%d] = %q, want %q", i, tokens[i], expected[i])
		}
	}
}

func TestFormatRelativeDate(t *testing.T) {
	now := time.Date(2024, 12, 25, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		t    time.Time
		want string
	}{
		{now, "now"},
		{now.Add(-30 * time.Second), "now"},
		{now.Add(-2 * time.Minute), "2 minutes ago"},
		{now.Add(-1 * time.Hour), "1 hour ago"},
		{now.Add(-3 * 24 * time.Hour), "3 days ago"},
		{now.Add(-1 * 24 * time.Hour), "yesterday"},
		{now.Add(2 * time.Minute), "in 2 minutes"},
		{now.Add(1 * time.Hour), "in 1 hour"},
		{now.Add(3 * 24 * time.Hour), "in 3 days"},
		{now.Add(1 * 24 * time.Hour), "tomorrow"},
		{now.Add(45 * time.Hour), "in 2 days"},       // 1d21h should be "in 2 days", not "tomorrow"
		{now.Add(36 * time.Hour), "in 2 days"},       // 1d12h should be "in 2 days"
		{now.Add(23 * time.Hour), "in 23 hours"},      // <24h stays in hours
	}
	for _, tc := range tests {
		got := formatRelativeDate(now, tc.t)
		if got != tc.want {
			t.Errorf("formatRelativeDate(%v) = %q, want %q", tc.t, got, tc.want)
		}
	}
}
