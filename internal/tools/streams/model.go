package streams

import (
	"fmt"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/components"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/pkg/colors"
	tea "github.com/charmbracelet/bubbletea"
)

// Model represents the streams TUI model
type Model struct {
	config  *config.Config
	colors  colors.Colors
	api     *API
	spinner components.Spinner
	history *SessionHistory

	// State
	loading    bool
	error      string
	streams    []StreamInfo
	lastUpdate time.Time

	// UI state
	width  int
	height int
	ready  bool
}

// NewModel creates a new streams model
func NewModel(cfg *config.Config) Model {
	colors := colors.New(cfg.NoColor)
	return Model{
		config:  cfg,
		colors:  colors,
		api:     NewAPI(cfg),
		spinner: components.NewSpinner(colors),
		history: NewSessionHistory(),
		loading: true,
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Start(),
		tea.Tick(m.config.RefreshInterval, func(t time.Time) tea.Msg {
			return TickMsg(t)
		}),
		m.fetchStreams(),
	)
}

// TickMsg is sent on each refresh interval
type TickMsg time.Time

// StreamsFetchedMsg is sent when streams are fetched
type StreamsFetchedMsg struct {
	Streams []StreamInfo
	Error   error
}

// Update handles model updates
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyRunes:
			if len(msg.Runes) > 0 {
				switch msg.Runes[0] {
				case 'r', 'R':
					m.loading = true
					return m, tea.Batch(m.spinner.Start(), m.fetchStreams())
				}
			}
		case tea.KeyF5:
			m.loading = true
			return m, tea.Batch(m.spinner.Start(), m.fetchStreams())
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case TickMsg:
		return m, tea.Tick(m.config.RefreshInterval, func(t time.Time) tea.Msg {
			return TickMsg(t)
		})

	case StreamsFetchedMsg:
		m.loading = false
		if msg.Error != nil {
			m.error = msg.Error.Error()
		} else {
			m.error = ""
			m.streams = msg.Streams
			m.lastUpdate = time.Now()
			m.updateHistory(msg.Streams)
		}
	}

	// Update spinner
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)

	return m, cmd
}

// View renders the model
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	var content string

	// Header
	content += m.renderHeader()

	// Loading or error state
	if m.loading {
		content += m.spinner.View() + " Fetching streams...\n"
		return content
	}

	if m.error != "" {
		content += m.colors.Style(colors.Red).Render("Error: "+m.error) + "\n"
		return content
	}

	// Streams content
	content += m.renderStreams()

	return content
}

// renderHeader renders the header
func (m Model) renderHeader() string {
	header := m.colors.Style(colors.Cyan).Bold(true).Render("=== Media Streams Monitor ===")

	active, ended := m.getActiveAndEndedSessions()

	info := fmt.Sprintf("Active Sessions: %s%d%s",
		m.colors.Style(colors.Bold).Render(""),
		len(active),
		m.colors.Style(colors.Reset).Render(""))

	if len(ended) > 0 {
		info += fmt.Sprintf(" Recently Ended: %s%d%s",
			m.colors.Style(colors.Gray).Render(""),
			len(ended),
			m.colors.Style(colors.Reset).Render(""))
	}

	if !m.lastUpdate.IsZero() {
		info += fmt.Sprintf("\nLast Update: %s",
			m.lastUpdate.Format("15:04:05"))
	}

	return fmt.Sprintf("%s\n%s\n\n", header, info)
}

// renderStreams renders the streams
func (m Model) renderStreams() string {
	if len(m.streams) == 0 {
		return m.colors.Style(colors.Green).Render("No active streams") + "\n"
	}

	var content string
	active, ended := m.getActiveAndEndedSessions()

	// Active streams
	for _, record := range active {
		content += m.renderStream(record.Stream)
	}

	// Recently ended streams
	if len(ended) > 0 {
		content += "\n"
		content += m.colors.Style(colors.Gray).Bold(true).Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") + "\n"
		content += m.colors.Style(colors.Gray).Bold(true).Render("Recently Ended Sessions:") + "\n\n"

		for _, record := range ended {
			content += m.renderEndedStream(record)
		}
	}

	// Summary
	if len(active) > 0 {
		content += m.renderSummary(active)
	}

	return content
}

// renderStream renders a single stream
func (m Model) renderStream(stream StreamInfo) string {
	content := m.colors.Style(colors.Blue).Bold(true).Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") + "\n"

	// Server badge
	serverColor := m.colors.ServerColor(stream.Server)
	content += fmt.Sprintf("%s[%s]%s ",
		m.colors.Style(serverColor).Render(""),
		strings.ToUpper(stream.Server),
		m.colors.Style(colors.Reset).Render(""))

	content += fmt.Sprintf("%sUser%s: %s%s%s\n",
		m.colors.Style(colors.Bold).Render(""),
		m.colors.Style(colors.Reset).Render(""),
		m.colors.Style(colors.Yellow).Render(""),
		stream.User,
		m.colors.Style(colors.Reset).Render(""))

	// Media info
	if stream.Type == "episode" && stream.Show != "" {
		content += fmt.Sprintf("%sShow%s: %s\n",
			m.colors.Style(colors.Bold).Render(""),
			m.colors.Style(colors.Reset).Render(""),
			stream.Show)
		if stream.Season != "" {
			content += fmt.Sprintf("%sSeason%s: %s\n",
				m.colors.Style(colors.Bold).Render(""),
				m.colors.Style(colors.Reset).Render(""),
				stream.Season)
		}
		if stream.Episode != "" {
			content += fmt.Sprintf("%sEpisode%s: %s - %s\n",
				m.colors.Style(colors.Bold).Render(""),
				m.colors.Style(colors.Reset).Render(""),
				stream.Episode, stream.Title)
		}
	} else {
		content += fmt.Sprintf("%sTitle%s: %s",
			m.colors.Style(colors.Bold).Render(""),
			m.colors.Style(colors.Reset).Render(""),
			stream.Title)
		if stream.Year != "" {
			content += fmt.Sprintf(" %s(%s)%s",
				m.colors.Style(colors.Cyan).Render(""),
				stream.Year,
				m.colors.Style(colors.Reset).Render(""))
		}
		content += "\n"
	}

	// Client
	if stream.Device != "" {
		content += fmt.Sprintf("%sClient%s: %s (%s)\n",
			m.colors.Style(colors.Bold).Render(""),
			m.colors.Style(colors.Reset).Render(""),
			stream.Client, stream.Device)
	} else {
		content += fmt.Sprintf("%sClient%s: %s\n",
			m.colors.Style(colors.Bold).Render(""),
			m.colors.Style(colors.Reset).Render(""),
			stream.Client)
	}

	// Status
	statusColor := m.colors.StatusColor(stream.Transcoding)
	statusText := stream.Status
	if stream.IsPaused {
		statusText += " (Paused)"
	}
	content += fmt.Sprintf("%sStatus%s: %s%s%s\n",
		m.colors.Style(colors.Bold).Render(""),
		m.colors.Style(colors.Reset).Render(""),
		m.colors.Style(statusColor).Render(""),
		statusText,
		m.colors.Style(colors.Reset).Render(""))

	// Bandwidth
	if stream.Bandwidth > 0 {
		content += fmt.Sprintf("%sBandwidth%s: %s%.2f Mbps%s\n",
			m.colors.Style(colors.Bold).Render(""),
			m.colors.Style(colors.Reset).Render(""),
			m.colors.Style(colors.Magenta).Render(""),
			stream.Bandwidth,
			m.colors.Style(colors.Reset).Render(""))
	}

	// Quality
	if stream.Resolution != "" || stream.VideoCodec != "" {
		content += fmt.Sprintf("%sQuality%s: ",
			m.colors.Style(colors.Bold).Render(""),
			m.colors.Style(colors.Reset).Render(""))
		if stream.Resolution != "" {
			content += fmt.Sprintf("%s ", stream.Resolution)
		}
		if stream.VideoCodec != "" {
			content += fmt.Sprintf("(%s", stream.VideoCodec)
			if stream.AudioCodec != "" {
				content += fmt.Sprintf("/%s", stream.AudioCodec)
			}
			content += ")"
		}
		content += "\n"
	}

	return content
}

// renderEndedStream renders a recently ended stream
func (m Model) renderEndedStream(record SessionRecord) string {
	stream := record.Stream
	content := m.colors.Style(colors.Gray).Bold(true).Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") + "\n"

	// Server badge
	content += fmt.Sprintf("%s[%s]%s ",
		m.colors.Style(colors.Gray).Render(""),
		strings.ToUpper(stream.Server),
		m.colors.Style(colors.Reset).Render(""))

	content += fmt.Sprintf("%sUser%s: %s%s%s ",
		m.colors.Style(colors.Gray).Render(""),
		m.colors.Style(colors.Reset).Render(""),
		m.colors.Style(colors.Gray).Render(""),
		stream.User,
		m.colors.Style(colors.Reset).Render(""))

	content += fmt.Sprintf("%s[ENDED %s]%s\n",
		m.colors.Style(colors.Gray).Render(""),
		m.formatTimeSince(*record.EndTime),
		m.colors.Style(colors.Reset).Render(""))

	// Media info (shortened)
	if stream.Type == "episode" && stream.Show != "" {
		content += fmt.Sprintf("%sShow%s: %s",
			m.colors.Style(colors.Gray).Render(""),
			m.colors.Style(colors.Reset).Render(""),
			stream.Show)
		if stream.Season != "" && stream.Episode != "" {
			content += fmt.Sprintf(" S%sE%s", stream.Season, stream.Episode)
		}
		content += "\n"
	} else {
		content += fmt.Sprintf("%sTitle%s: %s",
			m.colors.Style(colors.Gray).Render(""),
			m.colors.Style(colors.Reset).Render(""),
			stream.Title)
		if stream.Year != "" {
			content += fmt.Sprintf(" (%s)", stream.Year)
		}
		content += "\n"
	}

	// Client
	content += fmt.Sprintf("%sClient%s: %s\n",
		m.colors.Style(colors.Gray).Render(""),
		m.colors.Style(colors.Reset).Render(""),
		stream.Client)

	// Duration
	if record.EndTime != nil {
		duration := record.EndTime.Sub(record.StartTime)
		content += fmt.Sprintf("%sDuration%s: %s\n",
			m.colors.Style(colors.Gray).Render(""),
			m.colors.Style(colors.Reset).Render(""),
			m.formatDuration(duration))
	}

	return content
}

// renderSummary renders the summary statistics
func (m Model) renderSummary(active []SessionRecord) string {
	content := "\n" + m.colors.Style(colors.Cyan).Bold(true).Render("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━") + "\n"

	transcodeCount := 0
	totalBandwidth := 0.0

	for _, record := range active {
		if record.Stream.Transcoding {
			transcodeCount++
		}
		totalBandwidth += record.Stream.Bandwidth
	}

	content += fmt.Sprintf("%sTotal Streams%s: %d\n",
		m.colors.Style(colors.Bold).Render(""),
		m.colors.Style(colors.Reset).Render(""),
		len(active))
	content += fmt.Sprintf("%sTranscoding%s: %d\n",
		m.colors.Style(colors.Bold).Render(""),
		m.colors.Style(colors.Reset).Render(""),
		transcodeCount)
	content += fmt.Sprintf("%sTotal Bandwidth%s: %s%.2f Mbps%s\n",
		m.colors.Style(colors.Bold).Render(""),
		m.colors.Style(colors.Reset).Render(""),
		m.colors.Style(colors.Magenta).Render(""),
		totalBandwidth,
		m.colors.Style(colors.Reset).Render(""))

	return content
}

// fetchStreams creates a command to fetch streams
func (m Model) fetchStreams() tea.Cmd {
	return func() tea.Msg {
		streams, err := m.api.FetchStreams()
		return StreamsFetchedMsg{
			Streams: streams,
			Error:   err,
		}
	}
}

// updateHistory updates the session history
func (m Model) updateHistory(currentStreams []StreamInfo) {
	now := time.Now()
	currentSessionIDs := make(map[string]bool)

	// Process current streams
	for _, stream := range currentStreams {
		sessionID := generateSessionID(stream)
		currentSessionIDs[sessionID] = true

		// If this is a new session, add it to history
		if _, exists := m.history.Records[sessionID]; !exists {
			m.history.Records[sessionID] = &SessionRecord{
				Stream:    stream,
				StartTime: now,
				EndTime:   nil,
				SessionID: sessionID,
			}
		} else {
			// Update existing session
			m.history.Records[sessionID].Stream = stream
			m.history.Records[sessionID].EndTime = nil // Mark as active
		}
	}

	// Mark sessions that are no longer active as ended
	for sessionID, record := range m.history.Records {
		if !currentSessionIDs[sessionID] && record.EndTime == nil {
			record.EndTime = &now
		}
	}

	// Clean up old ended sessions
	for sessionID, record := range m.history.Records {
		if record.EndTime != nil && now.Sub(*record.EndTime) > m.config.HistoryDuration {
			delete(m.history.Records, sessionID)
		}
	}
}

// getActiveAndEndedSessions separates active and recently ended sessions
func (m Model) getActiveAndEndedSessions() (active, ended []SessionRecord) {
	for _, record := range m.history.Records {
		if record.EndTime == nil {
			active = append(active, *record)
		} else {
			ended = append(ended, *record)
		}
	}
	return
}

// formatTimeSince returns a human-readable time difference
func (m Model) formatTimeSince(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		seconds := int(duration.Seconds())
		if seconds <= 1 {
			return "1 second ago"
		}
		return fmt.Sprintf("%d seconds ago", seconds)
	}

	if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}

	hours := int(duration.Hours())
	if hours == 1 {
		return "1 hour ago"
	}
	return fmt.Sprintf("%d hours ago", hours)
}

// formatDuration formats a duration for display
func (m Model) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds > 0 {
			return fmt.Sprintf("%dm %ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}
