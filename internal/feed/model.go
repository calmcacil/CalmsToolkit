package feed

import (
	"fmt"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/core"
)

// ToolConfig holds configuration for the Arr event feed tool.
type ToolConfig struct {
	core.CommonConfig
	SonarrInstances []config.ArrInstance
	RadarrInstances []config.ArrInstance
	PollInterval    time.Duration
	HistoryWindow   time.Duration
	ShowGrabbed     bool
	ShowImported    bool
	ShowFailed      bool
	ShowDeleted     bool
	ShowIgnored     bool
	ShowSubtitles   bool
	MaxEvents       int
}

// HistoryEvent represents a normalized Sonarr/Radarr history event.
type HistoryEvent struct {
	Server       string    `json:"server"`
	When         time.Time `json:"when"`
	Action       string    `json:"action"`
	Title        string    `json:"title"`
	Episode      string    `json:"episode,omitempty"`
	EpisodeTitle string    `json:"episodeTitle,omitempty"`
	Quality      string    `json:"quality"`
	Formats      []string  `json:"formats,omitempty"`
	SourceTitle  string    `json:"sourceTitle,omitempty"`
	EventType    string    `json:"eventType"`
	ID           int       `json:"id"`
	FileID       int       `json:"fileId,omitempty"`
	Subtitles    string    `json:"subtitles,omitempty"`
}

// SonarrHistory is the raw Sonarr history API response entry.
type SonarrHistory struct {
	EpisodeID     int                    `json:"episodeId"`
	SeriesID      int                    `json:"seriesId"`
	SourceTitle   string                 `json:"sourceTitle"`
	Quality       SonarrQuality          `json:"quality"`
	CustomFormats []CustomFormat         `json:"customFormats,omitempty"`
	QualityCutoff bool                   `json:"qualityCutoffNotMet"`
	Date          string                 `json:"date"`
	EventType     string                 `json:"eventType"`
	Data          map[string]interface{} `json:"data"`
	Episode       *SonarrEpisode         `json:"episode,omitempty"`
	Series        *SonarrSeries          `json:"series,omitempty"`
	ID            int                    `json:"id"`
}

// SonarrQuality wraps the quality info in Sonarr history.
type SonarrQuality struct {
	Quality       SonarrQualityItem `json:"quality"`
	CustomFormats []CustomFormat    `json:"customFormats"`
	Revision      QualityRevision   `json:"revision,omitempty"`
}

// SonarrQualityItem is a single quality definition.
type SonarrQualityItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source,omitempty"`
	Resolution int    `json:"resolution,omitempty"`
}

// CustomFormat represents a Sonarr/Radarr custom format.
type CustomFormat struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// QualityRevision represents a quality revision entry.
type QualityRevision struct {
	Version  int  `json:"version"`
	Real     int  `json:"real"`
	IsRepack bool `json:"isRepack"`
}

// SonarrEpisodeFileResponse is the Sonarr episode file API response.
type SonarrEpisodeFileResponse struct {
	ID        int              `json:"id"`
	MediaInfo *SonarrMediaInfo `json:"mediaInfo,omitempty"`
}

// RadarrMovieFileResponse is the Radarr movie file API response.
type RadarrMovieFileResponse struct {
	ID        int              `json:"id"`
	MediaInfo *RadarrMediaInfo `json:"mediaInfo,omitempty"`
}

// SonarrMediaInfo holds media info including subtitle languages (V3 API).
type SonarrMediaInfo struct {
	Subtitles string `json:"subtitles"`
}

// RadarrMediaInfo holds media info including subtitle languages (V3 API).
type RadarrMediaInfo struct {
	Subtitles string `json:"subtitles"`
}

// SonarrEpisode is an episode reference in Sonarr history.
type SonarrEpisode struct {
	ID            int    `json:"id"`
	SeasonNumber  int    `json:"seasonNumber"`
	EpisodeNumber int    `json:"episodeNumber"`
	Title         string `json:"title"`
}

// SonarrSeries is a series reference in Sonarr history.
type SonarrSeries struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// RadarrHistory is the raw Radarr history API response entry.
type RadarrHistory struct {
	MovieID       int                    `json:"movieId"`
	SourceTitle   string                 `json:"sourceTitle"`
	Quality       RadarrQuality          `json:"quality"`
	CustomFormats []CustomFormat         `json:"customFormats,omitempty"`
	QualityCutoff bool                   `json:"qualityCutoffNotMet"`
	Date          string                 `json:"date"`
	EventType     string                 `json:"eventType"`
	Data          map[string]interface{} `json:"data"`
	Movie         *RadarrMovie           `json:"movie,omitempty"`
	ID            int                    `json:"id"`
}

// RadarrQuality wraps the quality info in Radarr history.
type RadarrQuality struct {
	Quality       RadarrQualityItem `json:"quality"`
	CustomFormats []CustomFormat    `json:"customFormats"`
	Revision      QualityRevision   `json:"revision,omitempty"`
}

// RadarrQualityItem is a single quality definition.
type RadarrQualityItem struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Source     string `json:"source,omitempty"`
	Resolution int    `json:"resolution,omitempty"`
}

// RadarrMovie is a movie reference in Radarr history.
type RadarrMovie struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

func mapSonarrEventType(eventType string) string {
	switch eventType {
	case "grabbed":
		return "Grabbed"
	case "downloadFolderImported":
		return "Imported"
	case "downloadFailed":
		return "Failed"
	case "episodeFileDeleted":
		return "Deleted"
	case "episodeFileRenamed":
		return "Renamed"
	case "downloadIgnored":
		return "Ignored"
	case "seriesFolderImported":
		return "Bulk Import"
	default:
		return eventType
	}
}

func mapRadarrEventType(eventType string) string {
	switch eventType {
	case "grabbed":
		return "Grabbed"
	case "downloadFolderImported":
		return "Imported"
	case "downloadFailed":
		return "Failed"
	case "movieFileDeleted":
		return "Deleted"
	case "movieFileRenamed":
		return "Renamed"
	case "downloadIgnored":
		return "Ignored"
	default:
		return eventType
	}
}

func formatEpisode(season, episode int) string {
	return fmt.Sprintf("S%02dE%02d", season, episode)
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func truncateWithEllipsis(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

func subtitlesDisplay(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func center(s string, width int) string {
	v := colors.VisibleLen(s)
	if v >= width {
		runes := []rune(s)
		if len(runes) > width {
			return string(runes[:width])
		}
		return s
	}
	padding := width - v
	leftPad := padding / 2
	rightPad := padding - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}
