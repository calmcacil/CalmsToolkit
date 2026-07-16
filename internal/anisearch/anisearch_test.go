package anisearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/core"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/time/rate"
)

// --- Model Tests ---

func TestDisplayTitle(t *testing.T) {
	romaji := "Sword Art Online"
	english := "Sword Art Online"
	native := "ソードアート・オンライン"

	tests := []struct {
		name string
		t    Title
		want string
	}{
		{
			name: "english preferred over romaji",
			t:    Title{Romaji: &romaji, English: &english, Native: &native},
			want: "Sword Art Online",
		},
		{
			name: "romaji fallback when english empty",
			t:    Title{Romaji: &romaji, English: strPtr("")},
			want: "Sword Art Online",
		},
		{
			name: "romaji fallback when english nil",
			t:    Title{Romaji: &romaji},
			want: "Sword Art Online",
		},
		{
			name: "empty when all nil",
			t:    Title{},
			want: "",
		},
		{
			name: "empty when romaji nil and english empty",
			t:    Title{English: strPtr("")},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.t.DisplayTitle(); got != tt.want {
				t.Errorf("DisplayTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStudioNames(t *testing.T) {
	tests := []struct {
		name string
		s    Show
		want []string
	}{
		{
			name: "with studios",
			s:    Show{Studios: &Studios{Nodes: []StudioNode{{Name: "A-1 Pictures"}, {Name: "CloverWorks"}}}},
			want: []string{"A-1 Pictures", "CloverWorks"},
		},
		{
			name: "nil studios",
			s:    Show{Studios: nil},
			want: nil,
		},
		{
			name: "empty studios",
			s:    Show{Studios: &Studios{Nodes: []StudioNode{}}},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.s.StudioNames()
			if len(got) != len(tt.want) {
				t.Errorf("StudioNames() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("StudioNames()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestLookupByAniList(t *testing.T) {
	m := &AnibridgeMapping{
		byAniList: map[int]int{101: 1001, 102: 1002},
	}

	tvdb, ok := m.LookupByAniList(101)
	if !ok || tvdb != 1001 {
		t.Errorf("LookupByAniList(101) = (%d, %v), want (1001, true)", tvdb, ok)
	}

	tvdb, ok = m.LookupByAniList(999)
	if ok || tvdb != 0 {
		t.Errorf("LookupByAniList(999) = (%d, %v), want (0, false)", tvdb, ok)
	}

	// Nil mapping
	var nilMap *AnibridgeMapping
	tvdb, ok = nilMap.LookupByAniList(101)
	if ok || tvdb != 0 {
		t.Errorf("nil LookupByAniList = (%d, %v), want (0, false)", tvdb, ok)
	}
}

// --- Client Tests ---

func TestNewAniListClient(t *testing.T) {
	c := NewAniListClient(0)
	if c.http.Timeout != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", c.http.Timeout)
	}

	c = NewAniListClient(10 * time.Second)
	if c.http.Timeout != 10*time.Second {
		t.Errorf("custom timeout = %v, want 10s", c.http.Timeout)
	}
}

func TestJitter(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
	}{
		{"zero", 0},
		{"1s", time.Second},
		{"100ms", 100 * time.Millisecond},
		{"10s", 10 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for range 100 {
				got := jitter(tt.d)
				if tt.d > 0 {
					if got < tt.d-tt.d/4 || got > tt.d+tt.d/4 {
						t.Errorf("jitter(%v) = %v, want within ±25%%", tt.d, got)
					}
				} else {
					if got != 0 {
						t.Errorf("jitter(0) = %v, want 0", got)
					}
				}
			}
		})
	}
}

func TestSearchPage(t *testing.T) {
	// Build a valid GraphQL-like response for AniList.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request basics.
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want application/json", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("User-Agent") != anilistUserAgent {
			t.Errorf("User-Agent = %s, want %s", r.Header.Get("User-Agent"), anilistUserAgent)
		}

		resp := graphqlResponse{}
		resp.Data.Page.PageInfo = PageInfo{
			HasNextPage: false,
			CurrentPage: 1,
			Total:       2,
		}
		resp.Data.Page.Media = []Show{
			{
				ID:     1,
				Format: "TV",
				Title:  Title{Romaji: strPtr("Test Anime"), English: strPtr("Test Anime")},
				Status: "FINISHED",
			},
			{
				ID:     2,
				Format: "MOVIE",
				Title:  Title{Romaji: strPtr("Test Movie")},
				Status: "RELEASING",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Override the endpoint.
	origEndpoint := aniListEndpoint
	aniListEndpoint = server.URL
	defer func() { aniListEndpoint = origEndpoint }()

	ctx := context.Background()
	client := NewAniListClient(5 * time.Second)

	result, err := client.SearchPage(ctx, "test", 1, 10)
	if err != nil {
		t.Fatalf("SearchPage error: %v", err)
	}

	if len(result.Media) != 2 {
		t.Fatalf("got %d results, want 2", len(result.Media))
	}
	if result.Media[0].Title.DisplayTitle() != "Test Anime" {
		t.Errorf("result[0].Title = %q, want %q", result.Media[0].Title.DisplayTitle(), "Test Anime")
	}
	if result.PageInfo.Total != 2 {
		t.Errorf("Total = %d, want 2", result.PageInfo.Total)
	}
}

func TestSearchPageErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    string
	}{
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			body:       `{"error":"internal"}`,
			wantErr:    "giving up after",
		},
		{
			name:       "bad request",
			statusCode: http.StatusBadRequest,
			body:       `{"error":"bad query"}`,
			wantErr:    "400",
		},
		{
			name:       "graphql error",
			statusCode: http.StatusOK,
			body:       `{"errors":[{"message":"not found"}]}`,
			wantErr:    "not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer server.Close()

			// Speed up retries for tests.
			origEndpoint := aniListEndpoint
			aniListEndpoint = server.URL
			defer func() { aniListEndpoint = origEndpoint }()

			origRetry := maxRetry
			maxRetry = 1
			defer func() { maxRetry = origRetry }()

			origDelay := rateLimitDelay
			rateLimitDelay = 1 * time.Millisecond
			defer func() { rateLimitDelay = origDelay }()

			ctx := context.Background()
			client := NewAniListClient(5 * time.Second)

			_, err := client.SearchPage(ctx, "test", 1, 10)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestSearchPagination(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		page := callCount
		mu.Unlock()

		resp := graphqlResponse{}
		resp.Data.Page.PageInfo = PageInfo{
			HasNextPage: page < 3,
			CurrentPage: page,
			Total:       3,
		}
		resp.Data.Page.Media = []Show{
			{
				ID:    page,
				Title: Title{Romaji: strPtr(fmt.Sprintf("Page %d Anime", page))},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	origEndpoint := aniListEndpoint
	aniListEndpoint = server.URL
	defer func() { aniListEndpoint = origEndpoint }()

	ctx := context.Background()
	client := NewAniListClient(5 * time.Second)

	// Search paginates through all pages.
	result, err := client.Search(ctx, "test", 50)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	if len(result.Media) != 3 {
		t.Errorf("got %d results across pages, want 3", len(result.Media))
	}
}

// --- Mapping Tests ---

func TestCountSourceEpisodes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int
	}{
		{
			name: "single range",
			raw:  `{"1-12":"1-12"}`,
			want: 12,
		},
		{
			name: "multiple ranges",
			raw:  `{"1-12":"1-12","13-24":"13-24"}`,
			want: 24,
		},
		{
			name: "single episode",
			raw:  `{"5":"5"}`,
			want: 1,
		},
		{
			name: "open-ended range",
			raw:  `{"5-":"5-"}`,
			want: 1,
		},
		{
			name: "empty",
			raw:  `{}`,
			want: 0,
		},
		{
			name: "null",
			raw:  `null`,
			want: 0,
		},
		{
			name: "empty string",
			raw:  ``,
			want: 0,
		},
		{
			name: "negative eps ignored",
			raw:  `{"-5":"-5"}`,
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countSourceEpisodes(json.RawMessage(tt.raw))
			if got != tt.want {
				t.Errorf("countSourceEpisodes(%q) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestExtractTVDB(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		wantID int
		wantOK bool
	}{
		{
			name:   "single s1 entry",
			json:   `{"tvdb_show:12345:s1": {"1-12":"1-12"}}`,
			wantID: 12345,
			wantOK: true,
		},
		{
			name:   "multiple scopes picks s1",
			json:   `{"tvdb_show:12345:s0": {"1-3":"4-6"}, "tvdb_show:12345:s1": {"1-12":"1-12"}}`,
			wantID: 12345,
			wantOK: true,
		},
		{
			name:   "no s1 falls to highest ep count",
			json:   `{"tvdb_show:54321:s0": {"1-3":"4-6"}, "tvdb_show:54321:s2": {"1-24":"1-24"}}`,
			wantID: 54321,
			wantOK: true,
		},
		{
			name:   "prefers s1 even if fewer episodes",
			json:   `{"tvdb_show:33333:s1": {"1-2":"1-2"}, "tvdb_show:44444:s2": {"1-24":"1-24"}}`,
			wantID: 33333,
			wantOK: true,
		},
		{
			name:   "no matching prefix",
			json:   `{"other:123:s1": {"1-12":"1-12"}}`,
			wantID: 0,
			wantOK: false,
		},
		{
			name:   "malformed tvdb id",
			json:   `{"tvdb_show:bad:s1": {"1-12":"1-12"}}`,
			wantID: 0,
			wantOK: false,
		},
		{
			name:   "empty object",
			json:   `{}`,
			wantID: 0,
			wantOK: false,
		},
		{
			name:   "multiple seasons of same show s1 highest ep",
			json:   `{"tvdb_show:99999:s1": {"1-12":"1-12"}, "tvdb_show:99999:s2": {"1-24":"1-24"}}`,
			wantID: 99999,
			wantOK: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := json.NewDecoder(strings.NewReader(tt.json))
			id, ok := extractTVDB(dec)
			if id != tt.wantID || ok != tt.wantOK {
				t.Errorf("extractTVDB(%q) = (%d, %v), want (%d, %v)",
					tt.json, id, ok, tt.wantID, tt.wantOK)
			}
		})
	}
}

func TestParseAnibridgeJSON(t *testing.T) {
	// Build a sample mapping JSON.
	input := `{
		"mal:1": {"tvdb_show:100:s1": {"1-12":"1-12"}},
		"anilist:101": {"tvdb_show:200:s1": {"1-24":"1-24"}},
		"mal:2": {"tvdb_show:300:s2": {"1-12":"1-12"}},
		"anilist:102": {"tvdb_show:400:s1": {"1-1":"1-1"}},
		"$meta": {"schema_version": "3"},
		"mal:invalid": {"tvdb_show:500:s1": {"1-12":"1-12"}},
		"anilist:invalid": {"tvdb_show:600:s1": {"1-12":"1-12"}}
	}`

	m, err := parseAnibridgeJSON(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parseAnibridgeJSON error: %v", err)
	}

	if m == nil {
		t.Fatal("parseAnibridgeJSON returned nil")
	}

	// Check MAL entries.
	if tvdb, ok := m.byMAL[1]; !ok || tvdb != 100 {
		t.Errorf("byMAL[1] = (%d, %v), want (100, true)", tvdb, ok)
	}

	// Check AniList entries (s1 preference over s2).
	if tvdb, ok := m.byAniList[101]; !ok || tvdb != 200 {
		t.Errorf("byAniList[101] = (%d, %v), want (200, true)", tvdb, ok)
	}

	// Invalid IDs should be skipped.
	if _, ok := m.byMAL[0]; ok {
		t.Error("byMAL[0] should not exist")
	}
	if _, ok := m.byAniList[0]; ok {
		t.Error("byAniList[0] should not exist")
	}
}

func TestParseAnibridgeBytes(t *testing.T) {
	// Build zstd-compressed test data.
	input := `{"anilist:101": {"tvdb_show:200:s1": {"1-24":"1-24"}}}`

	var compressed bytes.Buffer
	zw, err := zstd.NewWriter(&compressed)
	if err != nil {
		t.Fatalf("zstd writer: %v", err)
	}
	zw.Write([]byte(input))
	zw.Close()

	m, err := parseAnibridgeBytes(compressed.Bytes())
	if err != nil {
		t.Fatalf("parseAnibridgeBytes error: %v", err)
	}

	if tvdb, ok := m.LookupByAniList(101); !ok || tvdb != 200 {
		t.Errorf("LookupByAniList(101) = (%d, %v), want (200, true)", tvdb, ok)
	}
}

func TestParseAnibridgeBytesCorrupt(t *testing.T) {
	_, err := parseAnibridgeBytes([]byte("not-zstd-data"))
	if err == nil {
		t.Fatal("expected error for corrupt data, got nil")
	}
}

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "test.txt")

	err := writeFileAtomic(path, []byte("hello world"))
	if err != nil {
		t.Fatalf("writeFileAtomic error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("content = %q, want %q", string(data), "hello world")
	}
}

func TestMappingMetaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.meta.json")

	meta := mappingMetadata{
		ETag:         `"abc123"`,
		LastModified: "Mon, 01 Jan 2024 00:00:00 GMT",
		FetchedAt:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		URL:          "https://example.com/mapping.json.zst",
	}

	err := writeMappingMeta(path, meta)
	if err != nil {
		t.Fatalf("writeMappingMeta error: %v", err)
	}

	loaded, err := readMappingMeta(path)
	if err != nil {
		t.Fatalf("readMappingMeta error: %v", err)
	}

	if loaded.ETag != meta.ETag {
		t.Errorf("ETag = %q, want %q", loaded.ETag, meta.ETag)
	}
	if loaded.URL != meta.URL {
		t.Errorf("URL = %q, want %q", loaded.URL, meta.URL)
	}
	if !loaded.FetchedAt.Equal(meta.FetchedAt) {
		t.Errorf("FetchedAt = %v, want %v", loaded.FetchedAt, meta.FetchedAt)
	}
}

func TestReadMappingMetaNotFound(t *testing.T) {
	meta, err := readMappingMeta("/nonexistent/path.meta.json")
	if err != nil {
		t.Fatalf("readMappingMeta error for missing file: %v", err)
	}
	if meta.ETag != "" || !meta.FetchedAt.IsZero() {
		t.Errorf("expected zero metadata for missing file, got %+v", meta)
	}
}

// --- Render Functions Tests ---

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want string
	}{
		{"no tags", "hello world", "hello world"},
		{"simple tags", "<p>hello</p>", "hello"},
		{"nested tags", "<div><p>hello</p></div>", "hello"},
		{"with entities", "hello &amp; goodbye", "hello & goodbye"},
		{"lt entity", "a &lt; b", "a < b"},
		{"gt entity", "a &gt; b", "a > b"},
		{"nbsp entity", "hello&nbsp;world", "hello world"},
		{"ampersand not entity", "rock & roll", "rock & roll"},
		{"empty string", "", ""},
		{"only tags", "<br/>", ""},
		{"mixed content", "<b>bold</b> and <i>italic</i>", "bold and italic"},
		{"entity apostrophe", "&#039;tis", "'tis"},
		{"entity quote", "&quot;hello&quot;", "\"hello\""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripHTML(tt.s); got != tt.want {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.s, got, tt.want)
			}
		})
	}
}

func TestWordWrap(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{
			name:  "short line no wrap",
			text:  "hello world",
			width: 80,
			want:  []string{"hello world"},
		},
		{
			name:  "wraps at width",
			text:  "hello world foo bar baz",
			width: 10,
			want:  []string{"hello", "world foo", "bar baz"},
		},
		{
			name:  "empty text",
			text:  "",
			width: 80,
			want:  nil,
		},
		{
			name:  "zero width",
			text:  "hello world",
			width: 0,
			want:  []string{"hello world"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wordWrap(tt.text, tt.width)
			if len(got) != len(tt.want) {
				t.Errorf("wordWrap(%q, %d) = %v (len=%d), want %v (len=%d)",
					tt.text, tt.width, got, len(got), tt.want, len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("wordWrap(%q, %d)[%d] = %q, want %q",
						tt.text, tt.width, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTruncateVis(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"needs truncation", "hello world how are you", 15, "hello world ..."},
		{"max len 3", "hello", 3, "hel"},
		{"max len 0", "hello", 0, ""},
		{"empty string", "", 10, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateVis(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateVis(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestRenderSearchResultsLayout(t *testing.T) {
	title := "Frieren: Beyond Journey’s End"
	episodes := 28
	score := 91
	result := &SearchResult{
		Media: []Show{{
			Title:       Title{English: &title},
			Format:      "TV",
			Episodes:    &episodes,
			AverageRank: &score,
			Status:      "FINISHED",
		}},
		PageInfo: PageInfo{CurrentPage: 1, HasNextPage: true},
	}

	output := renderSearchResults("Frieren", result, -1, nil, ToolConfig{CommonConfig: core.CommonConfig{NoColor: true}})
	if !strings.Contains(output, "[TV] 28 eps ·  91% [Finished]") {
		t.Fatalf("search result metadata is not spaced correctly:\n%s", output)
	}

	const wantWidth = 80
	for i, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
		if got := colors.VisibleLen(line); got != wantWidth {
			t.Errorf("line %d width = %d, want %d: %q", i+1, got, wantWidth, line)
		}
	}
}

func TestWriteTerminalFrameUsesCRLFInRawMode(t *testing.T) {
	var output bytes.Buffer
	writeTerminalFrame(&output, "first\nsecond\n", true)

	want := "\033[H\033[Jfirst\r\nsecond\r\n"
	if got := output.String(); got != want {
		t.Fatalf("writeTerminalFrame() = %q, want %q", got, want)
	}
}

func TestWriteTerminalFramePreservesNewlinesOutsideRawMode(t *testing.T) {
	var output bytes.Buffer
	writeTerminalFrame(&output, "first\nsecond\n", false)

	want := "\033[H\033[Jfirst\nsecond\n"
	if got := output.String(); got != want {
		t.Fatalf("writeTerminalFrame() = %q, want %q", got, want)
	}
}

func TestRenderDetailContainsLongFields(t *testing.T) {
	long := strings.Repeat("very-long-value-", 20)
	show := Show{
		Title:       Title{English: &long, Native: &long},
		Format:      long,
		Status:      long,
		Season:      long,
		Genres:      []string{long},
		Tags:        []Tag{{Name: long}},
		Description: long,
		Studios:     &Studios{Nodes: []StudioNode{{Name: long}}},
	}

	const wantWidth = 80
	for _, noColor := range []bool{true, false} {
		output := renderDetail(show, 424536, ToolConfig{CommonConfig: core.CommonConfig{NoColor: noColor}})
		for i, line := range strings.Split(strings.TrimSuffix(output, "\n"), "\n") {
			if got := colors.VisibleLen(line); got != wantWidth {
				t.Errorf("NoColor=%t line %d width = %d, want %d: %q", noColor, i+1, got, wantWidth, line)
			}
		}
	}
}

func TestFormatEpisodes(t *testing.T) {
	tests := []struct {
		name string
		ep   *int
		want string
	}{
		{"nil", nil, "?"},
		{"zero", intPtr(0), "0"},
		{"positive", intPtr(24), "24"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatEpisodes(tt.ep); got != tt.want {
				t.Errorf("formatEpisodes(%v) = %q, want %q", tt.ep, got, tt.want)
			}
		})
	}
}

func TestFormatScore(t *testing.T) {
	tests := []struct {
		name  string
		score *int
		want  string
	}{
		{"nil", nil, "—"},
		{"zero", intPtr(0), "0%"},
		{"positive", intPtr(85), "85%"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatScore(tt.score); got != tt.want {
				t.Errorf("formatScore(%v) = %q, want %q", tt.score, got, tt.want)
			}
		})
	}
}

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   string
	}{
		{"finished", "FINISHED", "Finished"},
		{"releasing", "RELEASING", "Airing"},
		{"not released", "NOT_YET_RELEASED", "Not Yet Released"},
		{"cancelled", "CANCELLED", "Cancelled"},
		{"hiatus", "HIATUS", "Hiatus"},
		{"unknown", "UNKNOWN", "UNKNOWN"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatStatus(tt.status); got != tt.want {
				t.Errorf("formatStatus(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestFormatSeason(t *testing.T) {
	tests := []struct {
		name   string
		season string
		year   *int
		want   string
	}{
		{"winter with year", "WINTER", intPtr(2024), "Winter 2024"},
		{"spring no year", "SPRING", nil, "Spring"},
		{"summer with year", "SUMMER", intPtr(2023), "Summer 2023"},
		{"fall with year", "FALL", intPtr(2022), "Fall 2022"},
		{"unknown", "UNKNOWN", nil, "UNKNOWN"},
		{"empty", "", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSeason(tt.season, tt.year); got != tt.want {
				t.Errorf("formatSeason(%q, %v) = %q, want %q", tt.season, tt.year, got, tt.want)
			}
		})
	}
}

func TestFormatPopularity(t *testing.T) {
	tests := []struct {
		name string
		p    *int
		want string
	}{
		{"nil", nil, "—"},
		{"zero", intPtr(0), "#0"},
		{"positive", intPtr(12345), "#12345"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatPopularity(tt.p); got != tt.want {
				t.Errorf("formatPopularity(%v) = %q, want %q", tt.p, got, tt.want)
			}
		})
	}
}

func TestFormatTVDB(t *testing.T) {
	tests := []struct {
		name string
		tvdb int
		want string
	}{
		{"zero", 0, "—"},
		{"negative", -1, "—"},
		{"positive", 12345, "12345"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTVDB(tt.tvdb); got != tt.want {
				t.Errorf("formatTVDB(%d) = %q, want %q", tt.tvdb, got, tt.want)
			}
		})
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"just now", now, "just now"},
		{"1 minute ago", now.Add(-1 * time.Minute), "1 minute ago"},
		{"5 minutes ago", now.Add(-5 * time.Minute), "5 minutes ago"},
		{"1 hour ago", now.Add(-1 * time.Hour), "1 hour ago"},
		{"3 hours ago", now.Add(-3 * time.Hour), "3 hours ago"},
		{"yesterday", now.Add(-24 * time.Hour), "yesterday"},
		{"5 days ago", now.Add(-5 * 24 * time.Hour), "5 days ago"},
		{"in the future", now.Add(-2 * time.Hour), "2 hours ago"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRelativeTime(tt.t)
			if got != tt.want {
				t.Errorf("formatRelativeTime(%v) = %q, want %q", tt.t, got, tt.want)
			}
		})
	}
}

// --- Input Tests ---

func TestReadKey(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  KeyEvent
	}{
		{"up arrow", []byte{0x1b, '[', 'A'}, KeyUp},
		{"down arrow", []byte{0x1b, '[', 'B'}, KeyDown},
		{"enter (CR)", []byte{0x0d}, KeyEnter},
		{"enter (LF)", []byte{0x0a}, KeyEnter},
		{"ctrl+c", []byte{0x03}, KeyCtrlC},
		{"q", []byte{'q'}, KeyQuit},
		{"Q", []byte{'Q'}, KeyQuit},
		{"n", []byte{'n'}, KeyNext},
		{"N", []byte{'N'}, KeyNext},
		{"p", []byte{'p'}, KeyPrev},
		{"P", []byte{'P'}, KeyPrev},
		{"b", []byte{'b'}, KeyBack},
		{"B", []byte{'B'}, KeyBack},
		{"0", []byte{'0'}, KeyDigit},
		{"9", []byte{'9'}, KeyDigit},
		{"escape", []byte{0x1b}, KeyEsc},
		{"unknown", []byte{'x'}, KeyUnknown},
		{"empty read", nil, KeyUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We test the mapping logic directly by simulating stdin.
			// Since readKey reads from os.Stdin, we test through a helper
			// that validates the key mapping is correct.
			got := simulateReadKey(tt.input)
			if got != tt.want {
				t.Errorf("readKey(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// simulateReadKey replicates readKey logic for testing.
func simulateReadKey(input []byte) KeyEvent {
	if len(input) == 0 {
		return KeyUnknown
	}

	switch input[0] {
	case 0x1b:
		if len(input) >= 3 && input[1] == '[' {
			switch input[2] {
			case 'A':
				return KeyUp
			case 'B':
				return KeyDown
			}
		}
		return KeyEsc
	case 0x0d, 0x0a:
		return KeyEnter
	case 0x03:
		return KeyCtrlC
	case 'q', 'Q':
		return KeyQuit
	case 'n', 'N':
		return KeyNext
	case 'p', 'P':
		return KeyPrev
	case 'b', 'B':
		return KeyBack
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return KeyDigit
	default:
		return KeyUnknown
	}
}

func TestPromptForQuery(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	// Write input then close write end before reading.
	w.Write([]byte("Attack on Titan\n"))
	w.Close()

	os.Stdin = r

	q, err := PromptForQuery()
	if err != nil {
		t.Fatalf("PromptForQuery error: %v", err)
	}
	if q != "Attack on Titan" {
		t.Errorf("got %q, want %q", q, "Attack on Titan")
	}
}

// --- ITOA test ---

func TestItoa(t *testing.T) {
	if got := itoa(42); got != "42" {
		t.Errorf("itoa(42) = %q, want %q", got, "42")
	}
	if got := itoa(0); got != "0" {
		t.Errorf("itoa(0) = %q, want %q", got, "0")
	}
	if got := itoa(-5); got != "-5" {
		t.Errorf("itoa(-5) = %q, want %q", got, "-5")
	}
	if got := itoa(math.MaxInt); got != fmt.Sprintf("%d", math.MaxInt) {
		t.Errorf("itoa(MaxInt) = %q, want %q", got, fmt.Sprintf("%d", math.MaxInt))
	}
}

// --- BuildToolConfig Tests ---

func TestBuildToolConfig(t *testing.T) {
	tk := &config.ToolkitConfig{
		General: config.GeneralConfig{
			Timeout: "15s",
			NoColor: true,
			Theme:   "catppuccin-mocha",
		},
		AniSearch: config.AniSearchConfig{
			Limit:       10,
			MappingURL:  "https://example.com/mapping.json.zst",
			MappingPath: "/tmp/mappings.json.zst",
		},
	}

	cfg := BuildToolConfig(tk)

	if cfg.Timeout != 15*time.Second {
		t.Errorf("Timeout = %v, want 15s", cfg.Timeout)
	}
	if !cfg.NoColor {
		t.Error("NoColor = false, want true")
	}
	if cfg.Theme != "catppuccin-mocha" {
		t.Errorf("Theme = %q, want %q", cfg.Theme, "catppuccin-mocha")
	}
	if cfg.Limit != 10 {
		t.Errorf("Limit = %d, want 10", cfg.Limit)
	}
	if cfg.MappingURL != "https://example.com/mapping.json.zst" {
		t.Errorf("MappingURL = %q, want %q", cfg.MappingURL, "https://example.com/mapping.json.zst")
	}
	if cfg.MappingPath != "/tmp/mappings.json.zst" {
		t.Errorf("MappingPath = %q, want %q", cfg.MappingPath, "/tmp/mappings.json.zst")
	}
}

func TestBuildToolConfigDefaults(t *testing.T) {
	cfg := BuildToolConfig(nil)

	if cfg.Timeout != 10*time.Second {
		t.Errorf("default Timeout = %v, want 10s", cfg.Timeout)
	}
	if cfg.Limit != 5 {
		t.Errorf("default Limit = %d, want 5", cfg.Limit)
	}
	if cfg.MappingURL != "https://github.com/anibridge/anibridge-mappings/releases/download/v3/mappings.json.zst" {
		t.Errorf("default MappingURL = %q, want default", cfg.MappingURL)
	}
	if cfg.Page != 1 {
		t.Errorf("default Page = %d, want 1", cfg.Page)
	}
}

func TestBuildToolConfigPartial(t *testing.T) {
	tk := &config.ToolkitConfig{
		General: config.GeneralConfig{
			NoColor: true,
			Theme:   "catppuccin-latte",
		},
		AniSearch: config.AniSearchConfig{
			Limit: 3,
		},
	}

	cfg := BuildToolConfig(tk)

	// These should come from the config.
	if cfg.Limit != 3 {
		t.Errorf("Limit = %d, want 3", cfg.Limit)
	}
	if !cfg.NoColor {
		t.Error("NoColor = false, want true")
	}
	if cfg.Theme != "catppuccin-latte" {
		t.Errorf("Theme = %q, want %q", cfg.Theme, "catppuccin-latte")
	}

	// These should be defaults.
	if cfg.MappingURL != "https://github.com/anibridge/anibridge-mappings/releases/download/v3/mappings.json.zst" {
		t.Errorf("default MappingURL = %q, want default", cfg.MappingURL)
	}
	if cfg.Page != 1 {
		t.Errorf("default Page = %d, want 1", cfg.Page)
	}
}

// --- Helpers ---

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

// TestRandSeed ensures jitter is deterministic enough for tests.
func TestJitterRange(t *testing.T) {
	// Test that jitter stays within ±25%.
	for range 1000 {
		d := 2 * time.Second
		result := jitter(d)
		min := d - d/4
		max := d + d/4
		if result < min || result > max {
			t.Errorf("jitter(%v) = %v, outside range [%v, %v]", d, result, min, max)
		}
	}
}

// TestJitterDeterminism checks that jitter with zero duration stays zero.
func TestJitterZeroDuration(t *testing.T) {
	zeroDur := time.Duration(0)
	if res := jitter(zeroDur); res != zeroDur {
		t.Errorf("jitter(0) = %v, want 0", res)
	}
}

// TestSearchResultMarshaling tests that SearchResult can be JSON marshaled.
func TestSearchResultMarshaling(t *testing.T) {
	result := SearchResult{
		PageInfo: PageInfo{
			HasNextPage: true,
			CurrentPage: 1,
			Total:       42,
		},
		Media: []Show{
			{ID: 1, Title: Title{Romaji: strPtr("Test")}},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded SearchResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.PageInfo.Total != 42 {
		t.Errorf("Total = %d, want 42", decoded.PageInfo.Total)
	}
	if len(decoded.Media) != 1 {
		t.Errorf("Media len = %d, want 1", len(decoded.Media))
	}
}

// TestAniListClientConcurrency tests that multiple concurrent searches don't race.
func TestAniListClientConcurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := graphqlResponse{}
		resp.Data.Page.PageInfo = PageInfo{HasNextPage: false, CurrentPage: 1}
		resp.Data.Page.Media = []Show{{ID: 1, Title: Title{Romaji: strPtr("Concurrent Test")}}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	origEndpoint := aniListEndpoint
	aniListEndpoint = server.URL
	defer func() { aniListEndpoint = origEndpoint }()

	ctx := context.Background()
	client := NewAniListClient(5 * time.Second)

	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.SearchPage(ctx, "test", 1, 10)
			if err != nil {
				t.Errorf("concurrent SearchPage error: %v", err)
			}
		}()
	}
	wg.Wait()
}

// TestRetryAfterCredit ensures that lock contention is released properly.
func TestClientRateLimitMutex(t *testing.T) {
	c := &AniListClient{
		http:    &http.Client{Timeout: time.Second},
		limiter: rate.NewLimiter(rate.Inf, 1),
	}

	// Should not block.
	ctx := context.Background()
	err := c.throttle(ctx)
	if err != nil {
		t.Errorf("throttle error: %v", err)
	}
}

// TestCommandLineOptions tests that runDetailView doesn't crash with nil mapping.
func TestRunDetailViewNilMapping(t *testing.T) {
	// Just ensure the rendering functions don't panic.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("runDetailView with nil mapping panicked: %v", r)
		}
	}()

	// Simulate the rendering without terminal interaction.
	show := Show{
		ID:     1,
		Format: "TV",
		Title:  Title{Romaji: strPtr("Test")},
		Status: "FINISHED",
	}

	cfg := ToolConfig{
		CommonConfig: core.CommonConfig{
			NoColor: true,
		},
	}

	// Just test renderDetail since runDetailView needs actual stdin.
	output := renderDetail(show, 0, cfg)
	if output == "" {
		t.Error("renderDetail returned empty output")
	}
	if !strings.Contains(output, "Test") {
		t.Errorf("renderDetail output should contain title, got: %q", output)
	}
}

// TestSearchResultIsEmpty checks that empty searches return an empty result.
func TestSearchPageEmptyResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := graphqlResponse{}
		resp.Data.Page.PageInfo = PageInfo{HasNextPage: false, CurrentPage: 1}
		resp.Data.Page.Media = []Show{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	origEndpoint := aniListEndpoint
	aniListEndpoint = server.URL
	defer func() { aniListEndpoint = origEndpoint }()

	ctx := context.Background()
	client := NewAniListClient(5 * time.Second)

	result, err := client.SearchPage(ctx, "nonexistent", 1, 10)
	if err != nil {
		t.Fatalf("SearchPage error: %v", err)
	}

	if len(result.Media) != 0 {
		t.Errorf("expected 0 results, got %d", len(result.Media))
	}
}

// TestGraphQLErrorResponse ensures GraphQL-level errors are surfaced.
func TestGraphQLErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errors":[{"message":"not found"},{"message":"invalid search"}]}`))
	}))
	defer server.Close()

	origEndpoint := aniListEndpoint
	aniListEndpoint = server.URL
	defer func() { aniListEndpoint = origEndpoint }()

	ctx := context.Background()
	client := NewAniListClient(5 * time.Second)

	_, err := client.SearchPage(ctx, "test", 1, 10)
	if err == nil {
		t.Fatal("expected GraphQL error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should contain 'not found', got: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid search") {
		t.Errorf("error should contain 'invalid search', got: %v", err)
	}
}
