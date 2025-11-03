package calendar

import "time"

// ViewMode represents different calendar view modes
type ViewMode int

const (
	DayView ViewMode = iota
	WeekView
	MonthView
)

// String returns the string representation of ViewMode
func (v ViewMode) String() string {
	switch v {
	case DayView:
		return "Day"
	case WeekView:
		return "Week"
	case MonthView:
		return "Month"
	default:
		return "Unknown"
	}
}

// SonarrEpisode represents an episode from Sonarr
type SonarrEpisode struct {
	SeriesID      int       `json:"seriesId"`
	EpisodeID     int       `json:"id"`
	Title         string    `json:"title"`
	AirDate       string    `json:"airDate"`
	AirDateUtc    time.Time `json:"airDateUtc"`
	SeasonNumber  int       `json:"seasonNumber"`
	EpisodeNumber int       `json:"episodeNumber"`
	Monitored     bool      `json:"monitored"`
	HasFile       bool      `json:"hasFile"`
	Series        *Series   `json:"series"`
}

// Series represents a TV series
type Series struct {
	Title       string `json:"title"`
	Year        int    `json:"year"`
	SeasonCount int    `json:"seasonCount"`
}

// RadarrMovie represents a movie from Radarr
type RadarrMovie struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Year        int    `json:"year"`
	ReleaseDate string `json:"physicalRelease"`
	DigitalDate string `json:"digitalRelease"`
	InCinemas   string `json:"inCinemas"`
	Monitored   bool   `json:"monitored"`
	HasFile     bool   `json:"hasFile"`
}

// QueueResponse represents queue response from Sonarr/Radarr
type QueueResponse struct {
	TotalRecords int         `json:"totalRecords"`
	Records      []QueueItem `json:"records"`
}

// QueueItem represents an item in the queue
type QueueItem struct {
	ID             int             `json:"id"`
	Status         string          `json:"status"`
	TrackedState   string          `json:"trackedDownloadState"`
	StatusMessages []StatusMessage `json:"statusMessages"`
}

// StatusMessage represents a status message in the queue
type StatusMessage struct {
	Title    string   `json:"title"`
	Messages []string `json:"messages"`
}

// CalendarItem represents a unified calendar item
type CalendarItem struct {
	Type           string // "episode" or "movie"
	Title          string // Episode or movie title
	ShowTitle      string // For episodes: series name
	Year           int
	Season         int
	Episode        int
	AirTime        time.Time
	HasFile        bool
	Monitored      bool
	IsPremiere     bool
	SourceInstance string // Which Sonarr/Radarr instance
}

// QueueIssue represents a service with queue problems
type QueueIssue struct {
	ServiceName string
	URL         string
	Count       int
}

// DateRange represents a date range for calendar queries
type DateRange struct {
	Start time.Time
	End   time.Time
}

// NewDateRange creates a new date range based on view mode and reference date
func NewDateRange(mode ViewMode, reference time.Time, daysPast int) DateRange {
	// Start from beginning of day
	start := time.Date(reference.Year(), reference.Month(), reference.Day(), 0, 0, 0, 0, reference.Location())

	// Adjust for past days
	if daysPast > 0 {
		start = start.AddDate(0, 0, -daysPast)
	}

	// Calculate end based on the adjusted start
	var end time.Time
	switch mode {
	case DayView:
		end = start.AddDate(0, 0, 1)
	case WeekView:
		end = start.AddDate(0, 0, 7)
	case MonthView:
		end = start.AddDate(0, 1, 0)
	default:
		end = start.AddDate(0, 0, 1)
	}

	return DateRange{Start: start, End: end}
}

// Navigate moves the date range forward or backward
func (dr DateRange) Navigate(direction int, mode ViewMode) DateRange {
	var days int
	switch mode {
	case DayView:
		days = direction
	case WeekView:
		days = direction * 7
	case MonthView:
		return DateRange{
			Start: dr.Start.AddDate(0, direction, 0),
			End:   dr.End.AddDate(0, direction, 0),
		}
	}

	return DateRange{
		Start: dr.Start.AddDate(0, 0, days),
		End:   dr.End.AddDate(0, 0, days),
	}
}
