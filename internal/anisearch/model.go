// Package anisearch searches AniList for anime by name and returns
// information about the show plus TVDB mapping via the anibridge dataset.
package anisearch

import (
	"github.com/calmcacil/CalmsToolkit/internal/core"
)

// ToolConfig holds configuration for the anisearch tool.
type ToolConfig struct {
	core.CommonConfig
	MappingURL   string
	MappingPath  string
	Limit        int
	ForceRefresh bool
	NoTVDB       bool
	Page         int
}

// --- AniList GraphQL types ---

// Title holds the romaji, english, and native titles from AniList.
type Title struct {
	Romaji  *string `json:"romaji"`
	English *string `json:"english"`
	Native  *string `json:"native"`
}

// DisplayTitle returns the English title if available, falling back to romaji.
func (t Title) DisplayTitle() string {
	if t.English != nil && *t.English != "" {
		return *t.English
	}
	if t.Romaji != nil {
		return *t.Romaji
	}
	return ""
}

// Tag represents an AniList content tag.
type Tag struct {
	Name string `json:"name"`
}

// StudioNode holds a studio name.
type StudioNode struct {
	Name string `json:"name"`
}

// Studios holds the list of studios for a show.
type Studios struct {
	Nodes []StudioNode `json:"nodes"`
}

// Show represents an anime entry from AniList search results.
type Show struct {
	ID          int      `json:"id"`
	IDMal       *int     `json:"idMal"`
	Title       Title    `json:"title"`
	Format      string   `json:"format"`
	Episodes    *int     `json:"episodes"`
	Status      string   `json:"status"`
	Season      string   `json:"season"`
	SeasonYear  *int     `json:"seasonYear"`
	Genres      []string `json:"genres"`
	Tags        []Tag    `json:"tags"`
	AverageRank *int     `json:"averageScore"`
	Popularity  *int     `json:"popularity"`
	Description string   `json:"description"`
	Studios     *Studios `json:"studios"`
}

// StudioNames returns a slice of studio names for display.
func (s Show) StudioNames() []string {
	if s.Studios == nil {
		return nil
	}
	names := make([]string, len(s.Studios.Nodes))
	for i, n := range s.Studios.Nodes {
		names[i] = n.Name
	}
	return names
}

// PageInfo holds pagination metadata from AniList.
type PageInfo struct {
	HasNextPage bool `json:"hasNextPage"`
	CurrentPage int  `json:"currentPage"`
	Total       int  `json:"total"`
}

// SearchResult wraps a page of search results from AniList.
type SearchResult struct {
	PageInfo PageInfo `json:"pageInfo"`
	Media    []Show   `json:"media"`
}

// graphqlResponse is the top-level response from AniList.
type graphqlResponse struct {
	Data struct {
		Page SearchResult `json:"Page"`
	} `json:"data"`
	Errors []graphqlError `json:"errors,omitempty"`
}

type graphqlError struct {
	Message string `json:"message"`
}

// --- Anibridge mapping types ---

// AnibridgeMapping holds the TVDB ID lookup tables parsed from an anibridge
// mapping file. Keys are MAL IDs or AniList IDs; values are TVDB show IDs.
type AnibridgeMapping struct {
	byMAL     map[int]int
	byAniList map[int]int
}

// LookupByAniList looks up the TVDB ID for an AniList show ID.
func (m *AnibridgeMapping) LookupByAniList(anilistID int) (int, bool) {
	if m == nil {
		return 0, false
	}
	tvdbID, ok := m.byAniList[anilistID]
	return tvdbID, ok
}
