//go:build mediastreams
// +build mediastreams

package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ANSI color codes
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[0;31m"
	ColorGreen   = "\033[0;32m"
	ColorYellow  = "\033[0;33m"
	ColorBlue    = "\033[0;34m"
	ColorMagenta = "\033[0;35m"
	ColorCyan    = "\033[0;36m"
	ColorGray    = "\033[0;90m"
	ColorBold    = "\033[1m"
)

// ANSI control sequences
const (
	AnsiClearScreen = "\033[2J"   // Clear entire screen
	AnsiHomeCursor  = "\033[H"    // Move cursor to home position (0,0)
	AnsiClearToEnd  = "\033[J"    // Clear from cursor to end of screen
	AnsiHideCursor  = "\033[?25l" // Hide cursor
	AnsiShowCursor  = "\033[?25h" // Show cursor
)

// Server types
const (
	ServerPlex     = "plex"
	ServerJellyfin = "jellyfin"
	ServerBoth     = "both"
)

// Plex XML structures
type PlexMediaContainer struct {
	XMLName xml.Name    `xml:"MediaContainer"`
	Size    int         `xml:"size,attr"`
	Videos  []PlexVideo `xml:"Video"`
	Tracks  []PlexTrack `xml:"Track"`
}

type PlexVideo struct {
	Title            string                `xml:"title,attr"`
	Year             string                `xml:"year,attr"`
	Type             string                `xml:"type,attr"`
	GrandparentTitle string                `xml:"grandparentTitle,attr"`
	ParentIndex      string                `xml:"parentIndex,attr"`
	Index            string                `xml:"index,attr"`
	User             PlexUser              `xml:"User"`
	Player           PlexPlayer            `xml:"Player"`
	Session          PlexSession           `xml:"Session"`
	Media            []PlexMedia           `xml:"Media"`
	TranscodeSession *PlexTranscodeSession `xml:"TranscodeSession"`
}

type PlexTrack struct {
	Title            string      `xml:"title,attr"`
	GrandparentTitle string      `xml:"grandparentTitle,attr"`
	User             PlexUser    `xml:"User"`
	Player           PlexPlayer  `xml:"Player"`
	Session          PlexSession `xml:"Session"`
	Media            []PlexMedia `xml:"Media"`
}

type PlexUser struct {
	Title string `xml:"title,attr"`
}

type PlexPlayer struct {
	Title  string `xml:"title,attr"`
	Device string `xml:"device,attr"`
}

type PlexSession struct {
	Bandwidth int `xml:"bandwidth,attr"`
}

type PlexMedia struct {
	VideoResolution string     `xml:"videoResolution,attr"`
	VideoCodec      string     `xml:"videoCodec,attr"`
	AudioCodec      string     `xml:"audioCodec,attr"`
	Parts           []PlexPart `xml:"Part"`
}

type PlexPart struct {
	VideoDecision string `xml:"videoDecision,attr"`
	AudioDecision string `xml:"audioDecision,attr"`
}

type PlexTranscodeSession struct {
	VideoDecision string `xml:"videoDecision,attr"`
	AudioDecision string `xml:"audioDecision,attr"`
}

// Jellyfin JSON structures
type JellyfinSession struct {
	PlayState          JellyfinPlayState        `json:"PlayState"`
	NowPlayingItem     *JellyfinNowPlayingItem  `json:"NowPlayingItem"`
	UserName           string                   `json:"UserName"`
	Client             string                   `json:"Client"`
	DeviceName         string                   `json:"DeviceName"`
	Id                 string                   `json:"Id"`
	TranscodingInfo    *JellyfinTranscodingInfo `json:"TranscodingInfo"`
	PlayableMediaTypes []string                 `json:"PlayableMediaTypes"`
}

type JellyfinPlayState struct {
	PositionTicks int64  `json:"PositionTicks"`
	IsPaused      bool   `json:"IsPaused"`
	PlayMethod    string `json:"PlayMethod"`
}

type JellyfinNowPlayingItem struct {
	Name              string                `json:"Name"`
	SeriesName        string                `json:"SeriesName"`
	Type              string                `json:"Type"`
	ProductionYear    int                   `json:"ProductionYear"`
	IndexNumber       int                   `json:"IndexNumber"`
	ParentIndexNumber int                   `json:"ParentIndexNumber"`
	MediaStreams      []JellyfinMediaStream `json:"MediaStreams"`
}

type JellyfinMediaStream struct {
	Type   string `json:"Type"`
	Codec  string `json:"Codec"`
	Height int    `json:"Height"`
}

type JellyfinTranscodingInfo struct {
	IsVideoDirect    bool     `json:"IsVideoDirect"`
	IsAudioDirect    bool     `json:"IsAudioDirect"`
	Bitrate          int      `json:"Bitrate"`
	VideoCodec       string   `json:"VideoCodec"`
	AudioCodec       string   `json:"AudioCodec"`
	Height           int      `json:"Height"`
	TranscodeReasons []string `json:"TranscodeReasons"`
}

// Unified structures
type Config struct {
	ServerType      string
	PlexURL         string
	PlexToken       string
	JellyfinURL     string
	JellyfinToken   string
	Timeout         time.Duration
	JSONOutput      bool
	WatchMode       bool
	WatchSeconds    int
	NoColor         bool
	HistoryDuration time.Duration
}

type StreamInfo struct {
	Server      string  `json:"server"` // "plex" or "jellyfin"
	User        string  `json:"user"`
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Year        string  `json:"year,omitempty"`
	Show        string  `json:"show,omitempty"`
	Season      string  `json:"season,omitempty"`
	Episode     string  `json:"episode,omitempty"`
	Client      string  `json:"client"`
	Device      string  `json:"device,omitempty"`
	Status      string  `json:"status"`
	Transcoding bool    `json:"transcoding"`
	Bandwidth   float64 `json:"bandwidth_mbps"`
	VideoCodec  string  `json:"video_codec,omitempty"`
	AudioCodec  string  `json:"audio_codec,omitempty"`
	Resolution  string  `json:"resolution,omitempty"`
	IsPaused    bool    `json:"is_paused,omitempty"`
}

// SessionRecord tracks a stream with its lifecycle timestamps
type SessionRecord struct {
	Stream    StreamInfo
	StartTime time.Time
	EndTime   *time.Time // nil if still active
	SessionID string     // unique identifier to track across refreshes
}

// SessionHistory maintains the in-memory history of sessions
type SessionHistory struct {
	Records map[string]*SessionRecord // keyed by SessionID
}

type Summary struct {
	TotalStreams     int          `json:"total_streams"`
	TranscodingCount int          `json:"transcoding_count"`
	TotalBandwidth   float64      `json:"total_bandwidth_mbps"`
	PlexStreams      int          `json:"plex_streams"`
	JellyfinStreams  int          `json:"jellyfin_streams"`
	Timestamp        time.Time    `json:"timestamp"`
	Streams          []StreamInfo `json:"streams"`
}

func main() {
	var (
		serverType      = flag.String("server", "both", "Server type: plex, jellyfin, or both")
		plexURL         = flag.String("plex-url", "", "Plex server URL")
		plexToken       = flag.String("plex-token", "", "Plex authentication token")
		jellyfinURL     = flag.String("jellyfin-url", "", "Jellyfin server URL")
		jellyfinToken   = flag.String("jellyfin-token", "", "Jellyfin API token")
		timeout         = flag.Duration("timeout", 10*time.Second, "Connection timeout")
		noColor         = flag.Bool("no-color", false, "Disable colored output")
		jsonOutput      = flag.Bool("json", false, "Output in JSON format")
		watchMode       = flag.Bool("watch", false, "Continuously monitor streams")
		watchSeconds    = flag.Int("interval", 5, "Watch mode refresh interval in seconds")
		historyDuration = flag.Duration("history-duration", 5*time.Minute, "How long to keep session history in watch mode")
	)
	flag.Parse()

	config := loadConfig(*serverType, *plexURL, *plexToken, *jellyfinURL, *jellyfinToken,
		*timeout, *noColor, *jsonOutput, *watchMode, *watchSeconds, *historyDuration)

	// Validate configuration
	if config.ServerType == ServerPlex || config.ServerType == ServerBoth {
		if config.PlexToken == "" {
			fmt.Fprintf(os.Stderr, "ERROR: PLEX_TOKEN is not set\n")
			os.Exit(1)
		}
	}
	if config.ServerType == ServerJellyfin || config.ServerType == ServerBoth {
		if config.JellyfinToken == "" {
			fmt.Fprintf(os.Stderr, "ERROR: JELLYFIN_TOKEN is not set\n")
			os.Exit(1)
		}
	}

	// Watch mode
	if config.WatchMode {
		// Hide cursor for cleaner display
		fmt.Print(AnsiHideCursor)
		fmt.Print(AnsiClearScreen)
		fmt.Print(AnsiHomeCursor)
		// Ensure cursor is shown on exit
		defer fmt.Print(AnsiShowCursor)

		// Initialize session history
		history := &SessionHistory{
			Records: make(map[string]*SessionRecord),
		}

		for {
			fmt.Print(AnsiHomeCursor)
			if err := displayAllSessionsWithHistory(config, history); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			}
			fmt.Print(AnsiClearToEnd)
			time.Sleep(time.Duration(config.WatchSeconds) * time.Second)
		}
	}

	// Single execution (no history tracking)
	if err := displayAllSessions(config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(serverType, plexURL, plexToken, jellyfinURL, jellyfinToken string,
	timeout time.Duration, noColor, jsonOutput, watchMode bool, watchSeconds int, historyDuration time.Duration) Config {

	config := Config{
		ServerType:      strings.ToLower(serverType),
		PlexURL:         "http://localhost:32400",
		PlexToken:       "",
		JellyfinURL:     "http://localhost:8096",
		JellyfinToken:   "",
		Timeout:         timeout,
		JSONOutput:      jsonOutput,
		WatchMode:       watchMode,
		WatchSeconds:    watchSeconds,
		NoColor:         noColor || jsonOutput,
		HistoryDuration: historyDuration,
	}

	// Load from .env file
	envPath := "/opt/apps/compose/.env"
	if _, err := os.Stat(envPath); err == nil {
		loadEnvFile(envPath, &config)
	}

	// Environment variables override .env file
	if envURL := os.Getenv("PLEX_URL"); envURL != "" {
		config.PlexURL = envURL
	}
	if envToken := os.Getenv("PLEX_TOKEN"); envToken != "" {
		config.PlexToken = envToken
	}
	if envURL := os.Getenv("JELLYFIN_URL"); envURL != "" {
		config.JellyfinURL = envURL
	}
	if envToken := os.Getenv("JELLYFIN_TOKEN"); envToken != "" {
		config.JellyfinToken = envToken
	}

	// Command line flags override everything
	if plexURL != "" {
		config.PlexURL = plexURL
	}
	if plexToken != "" {
		config.PlexToken = plexToken
	}
	if jellyfinURL != "" {
		config.JellyfinURL = jellyfinURL
	}
	if jellyfinToken != "" {
		config.JellyfinToken = jellyfinToken
	}

	config.PlexURL = strings.TrimSuffix(config.PlexURL, "/")
	config.JellyfinURL = strings.TrimSuffix(config.JellyfinURL, "/")

	return config
}

func loadEnvFile(path string, config *Config) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"'")

		switch key {
		case "PLEX_URL":
			config.PlexURL = value
		case "PLEX_TOKEN":
			config.PlexToken = value
		case "JELLYFIN_URL":
			config.JellyfinURL = value
		case "JELLYFIN_TOKEN":
			config.JellyfinToken = value
		}
	}
}

// generateSessionID creates a unique identifier for a stream to track it across refreshes
func generateSessionID(stream StreamInfo) string {
	// Combine server, user, and title to create a reasonably unique ID
	// This won't be perfect (same user watching same content twice) but good enough
	return fmt.Sprintf("%s:%s:%s:%s", stream.Server, stream.User, stream.Title, stream.Client)
}

// displayAllSessionsWithHistory fetches current sessions and updates history
func displayAllSessionsWithHistory(config Config, history *SessionHistory) error {
	var allStreams []StreamInfo
	var plexCount, jellyfinCount int

	// Fetch Plex sessions
	if config.ServerType == ServerPlex || config.ServerType == ServerBoth {
		if streams, err := fetchPlexStreams(config); err == nil {
			allStreams = append(allStreams, streams...)
			plexCount = len(streams)
		}
	}

	// Fetch Jellyfin sessions
	if config.ServerType == ServerJellyfin || config.ServerType == ServerBoth {
		if streams, err := fetchJellyfinStreams(config); err == nil {
			allStreams = append(allStreams, streams...)
			jellyfinCount = len(streams)
		}
	}

	// Update history with current sessions
	updateHistory(history, allStreams, config.HistoryDuration)

	// JSON output
	if config.JSONOutput {
		return displayJSONOutput(allStreams, plexCount, jellyfinCount)
	}

	// Terminal output with history
	return displayTerminalOutputWithHistory(allStreams, history, plexCount, jellyfinCount, config.NoColor)
}

// updateHistory compares current streams with history and updates records
func updateHistory(history *SessionHistory, currentStreams []StreamInfo, historyDuration time.Duration) {
	now := time.Now()
	currentSessionIDs := make(map[string]bool)

	// Process current streams
	for _, stream := range currentStreams {
		sessionID := generateSessionID(stream)
		currentSessionIDs[sessionID] = true

		// If this is a new session, add it to history
		if _, exists := history.Records[sessionID]; !exists {
			history.Records[sessionID] = &SessionRecord{
				Stream:    stream,
				StartTime: now,
				EndTime:   nil,
				SessionID: sessionID,
			}
		} else {
			// Update existing session (stream info might have changed)
			history.Records[sessionID].Stream = stream
			history.Records[sessionID].EndTime = nil // Mark as active
		}
	}

	// Mark sessions that are no longer active as ended
	for sessionID, record := range history.Records {
		if !currentSessionIDs[sessionID] && record.EndTime == nil {
			// This session ended
			record.EndTime = &now
		}
	}

	// Clean up old ended sessions that exceed history duration
	for sessionID, record := range history.Records {
		if record.EndTime != nil && now.Sub(*record.EndTime) > historyDuration {
			delete(history.Records, sessionID)
		}
	}
}

// getActiveAndEndedSessions separates active and recently ended sessions
func getActiveAndEndedSessions(history *SessionHistory) (active, ended []SessionRecord) {
	for _, record := range history.Records {
		if record.EndTime == nil {
			active = append(active, *record)
		} else {
			ended = append(ended, *record)
		}
	}
	return
}

// formatTimeSince returns a human-readable time difference
func formatTimeSince(t time.Time) string {
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

func displayAllSessions(config Config) error {
	var allStreams []StreamInfo
	var plexCount, jellyfinCount int

	// Fetch Plex sessions
	if config.ServerType == ServerPlex || config.ServerType == ServerBoth {
		if streams, err := fetchPlexStreams(config); err == nil {
			allStreams = append(allStreams, streams...)
			plexCount = len(streams)
		}
	}

	// Fetch Jellyfin sessions
	if config.ServerType == ServerJellyfin || config.ServerType == ServerBoth {
		if streams, err := fetchJellyfinStreams(config); err == nil {
			allStreams = append(allStreams, streams...)
			jellyfinCount = len(streams)
		}
	}

	// JSON output
	if config.JSONOutput {
		return displayJSONOutput(allStreams, plexCount, jellyfinCount)
	}

	// Terminal output
	return displayTerminalOutput(allStreams, plexCount, jellyfinCount, config.NoColor)
}

func fetchPlexStreams(config Config) ([]StreamInfo, error) {
	url := fmt.Sprintf("%s/status/sessions?X-Plex-Token=%s", config.PlexURL, config.PlexToken)

	client := &http.Client{Timeout: config.Timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var container PlexMediaContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, err
	}

	streams := make([]StreamInfo, 0)
	for _, video := range container.Videos {
		streams = append(streams, plexVideoToStream(video))
	}
	for _, track := range container.Tracks {
		streams = append(streams, plexTrackToStream(track))
	}

	return streams, nil
}

func fetchJellyfinStreams(config Config) ([]StreamInfo, error) {
	url := fmt.Sprintf("%s/Sessions?api_key=%s", config.JellyfinURL, config.JellyfinToken)

	client := &http.Client{Timeout: config.Timeout}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellyfin returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var sessions []JellyfinSession
	if err := json.Unmarshal(body, &sessions); err != nil {
		return nil, err
	}

	streams := make([]StreamInfo, 0)
	for _, session := range sessions {
		if session.NowPlayingItem != nil {
			streams = append(streams, jellyfinSessionToStream(session))
		}
	}

	return streams, nil
}

func plexVideoToStream(video PlexVideo) StreamInfo {
	stream := StreamInfo{
		Server:    "plex",
		User:      video.User.Title,
		Title:     video.Title,
		Year:      video.Year,
		Client:    video.Player.Title,
		Device:    video.Player.Device,
		Bandwidth: float64(video.Session.Bandwidth) / 1000.0,
		Type:      video.Type,
		Show:      video.GrandparentTitle,
		Season:    video.ParentIndex,
		Episode:   video.Index,
	}

	if len(video.Media) > 0 {
		stream.VideoCodec = video.Media[0].VideoCodec
		stream.AudioCodec = video.Media[0].AudioCodec
		stream.Resolution = video.Media[0].VideoResolution
	}

	videoDecision, audioDecision := getPlexDecisions(video.Media, video.TranscodeSession)
	stream.Transcoding = video.TranscodeSession != nil ||
		videoDecision == "transcode" || audioDecision == "transcode"

	if stream.Transcoding {
		stream.Status = "Transcoding"
	} else {
		stream.Status = "Direct"
	}

	return stream
}

func plexTrackToStream(track PlexTrack) StreamInfo {
	stream := StreamInfo{
		Server:    "plex",
		User:      track.User.Title,
		Type:      "track",
		Title:     track.Title,
		Show:      track.GrandparentTitle,
		Client:    track.Player.Title,
		Device:    track.Player.Device,
		Bandwidth: float64(track.Session.Bandwidth) / 1000.0,
	}

	if len(track.Media) > 0 {
		stream.AudioCodec = track.Media[0].AudioCodec
		if len(track.Media[0].Parts) > 0 {
			stream.Transcoding = track.Media[0].Parts[0].AudioDecision == "transcode"
		}
	}

	if stream.Transcoding {
		stream.Status = "Transcoding"
	} else {
		stream.Status = "Direct"
	}

	return stream
}

func jellyfinSessionToStream(session JellyfinSession) StreamInfo {
	stream := StreamInfo{
		Server:   "jellyfin",
		User:     session.UserName,
		Client:   session.Client,
		Device:   session.DeviceName,
		IsPaused: session.PlayState.IsPaused,
	}

	if session.NowPlayingItem != nil {
		item := session.NowPlayingItem
		stream.Title = item.Name
		stream.Type = strings.ToLower(item.Type)
		if item.ProductionYear > 0 {
			stream.Year = fmt.Sprintf("%d", item.ProductionYear)
		}
		stream.Show = item.SeriesName
		if item.ParentIndexNumber > 0 {
			stream.Season = fmt.Sprintf("%d", item.ParentIndexNumber)
		}
		if item.IndexNumber > 0 {
			stream.Episode = fmt.Sprintf("%d", item.IndexNumber)
		}
	}

	if session.TranscodingInfo != nil {
		ti := session.TranscodingInfo
		stream.Transcoding = !ti.IsVideoDirect || !ti.IsAudioDirect
		stream.Bandwidth = float64(ti.Bitrate) / 1000000.0
		stream.VideoCodec = ti.VideoCodec
		stream.AudioCodec = ti.AudioCodec
		if ti.Height > 0 {
			stream.Resolution = getResolutionName(ti.Height)
		}
	}

	if stream.Transcoding {
		stream.Status = "Transcoding"
	} else {
		stream.Status = "Direct"
	}

	return stream
}

func getPlexDecisions(media []PlexMedia, transcode *PlexTranscodeSession) (string, string) {
	var videoDecision, audioDecision string

	if len(media) > 0 && len(media[0].Parts) > 0 {
		videoDecision = media[0].Parts[0].VideoDecision
		audioDecision = media[0].Parts[0].AudioDecision
	}

	if transcode != nil {
		if videoDecision == "" {
			videoDecision = transcode.VideoDecision
		}
		if audioDecision == "" {
			audioDecision = transcode.AudioDecision
		}
	}

	return videoDecision, audioDecision
}

func getResolutionName(height int) string {
	switch {
	case height >= 2160:
		return "4K"
	case height >= 1440:
		return "1440p"
	case height >= 1080:
		return "1080p"
	case height >= 720:
		return "720p"
	case height >= 480:
		return "480p"
	default:
		return fmt.Sprintf("%dp", height)
	}
}

func displayJSONOutput(streams []StreamInfo, plexCount, jellyfinCount int) error {
	summary := Summary{
		TotalStreams:    len(streams),
		PlexStreams:     plexCount,
		JellyfinStreams: jellyfinCount,
		Timestamp:       time.Now(),
		Streams:         streams,
	}

	totalBandwidth := 0.0
	transcodeCount := 0

	for _, stream := range streams {
		totalBandwidth += stream.Bandwidth
		if stream.Transcoding {
			transcodeCount++
		}
	}

	summary.TranscodingCount = transcodeCount
	summary.TotalBandwidth = totalBandwidth

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

func displayTerminalOutput(streams []StreamInfo, plexCount, jellyfinCount int, noColor bool) error {
	// Use strings.Builder for double-buffering to prevent flashing
	var buf strings.Builder

	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	buf.WriteString(fmt.Sprintf("%s%s=== Media Streams Monitor ===%s\n",
		color(ColorBold), color(ColorCyan), color(ColorReset)))
	buf.WriteString(fmt.Sprintf("Total Sessions: %s%d%s", color(ColorBold), len(streams), color(ColorReset)))
	if plexCount > 0 || jellyfinCount > 0 {
		buf.WriteString(fmt.Sprintf(" (Plex: %d, Jellyfin: %d)", plexCount, jellyfinCount))
	}
	buf.WriteString("\n\n")

	if len(streams) == 0 {
		buf.WriteString(fmt.Sprintf("%sNo active streams%s\n", color(ColorGreen), color(ColorReset)))
		// Write everything at once
		fmt.Print(buf.String())
		return nil
	}

	for _, stream := range streams {
		displayStreamToBuffer(&buf, stream, noColor)
	}

	displayStreamSummaryToBuffer(&buf, streams, noColor)

	// Write everything at once to prevent flashing
	fmt.Print(buf.String())

	return nil
}

func displayTerminalOutputWithHistory(currentStreams []StreamInfo, history *SessionHistory, plexCount, jellyfinCount int, noColor bool) error {
	var buf strings.Builder

	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	// Header
	buf.WriteString(fmt.Sprintf("%s%s=== Media Streams Monitor ===%s\n",
		color(ColorBold), color(ColorCyan), color(ColorReset)))

	active, ended := getActiveAndEndedSessions(history)

	buf.WriteString(fmt.Sprintf("Active Sessions: %s%d%s", color(ColorBold), len(active), color(ColorReset)))
	if plexCount > 0 || jellyfinCount > 0 {
		buf.WriteString(fmt.Sprintf(" (Plex: %d, Jellyfin: %d)", plexCount, jellyfinCount))
	}
	buf.WriteString("\n")

	if len(ended) > 0 {
		buf.WriteString(fmt.Sprintf("Recently Ended: %s%d%s\n", color(ColorGray), len(ended), color(ColorReset)))
	}
	buf.WriteString("\n")

	// Display active sessions
	if len(active) == 0 {
		buf.WriteString(fmt.Sprintf("%sNo active streams%s\n", color(ColorGreen), color(ColorReset)))
	} else {
		for _, record := range active {
			displayStreamToBuffer(&buf, record.Stream, noColor)
		}
	}

	// Display recently ended sessions
	if len(ended) > 0 {
		buf.WriteString("\n")
		buf.WriteString(fmt.Sprintf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
			color(ColorBold), color(ColorGray), color(ColorReset)))
		buf.WriteString(fmt.Sprintf("%s%sRecently Ended Sessions:%s\n\n",
			color(ColorBold), color(ColorGray), color(ColorReset)))

		for _, record := range ended {
			displayEndedStreamToBuffer(&buf, record, noColor)
		}
	}

	// Display summary (only for active streams)
	if len(active) > 0 {
		var activeStreams []StreamInfo
		for _, record := range active {
			activeStreams = append(activeStreams, record.Stream)
		}
		displayStreamSummaryToBuffer(&buf, activeStreams, noColor)
	}

	// Write everything at once
	fmt.Print(buf.String())

	return nil
}

func displayStream(stream StreamInfo, noColor bool) {
	var buf strings.Builder
	displayStreamToBuffer(&buf, stream, noColor)
	fmt.Print(buf.String())
}

func displayStreamToBuffer(buf *strings.Builder, stream StreamInfo, noColor bool) {
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	buf.WriteString(fmt.Sprintf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		color(ColorBold), color(ColorBlue), color(ColorReset)))

	// Server badge
	serverColor := ColorMagenta
	if stream.Server == "plex" {
		serverColor = ColorYellow
	}
	buf.WriteString(fmt.Sprintf("%s[%s]%s ", color(serverColor), strings.ToUpper(stream.Server), color(ColorReset)))
	buf.WriteString(fmt.Sprintf("%sUser%s: %s%s%s\n", color(ColorBold), color(ColorReset),
		color(ColorYellow), stream.User, color(ColorReset)))

	// Media info
	if stream.Type == "episode" && stream.Show != "" {
		buf.WriteString(fmt.Sprintf("%sShow%s: %s\n", color(ColorBold), color(ColorReset), stream.Show))
		if stream.Season != "" {
			buf.WriteString(fmt.Sprintf("%sSeason%s: %s\n", color(ColorBold), color(ColorReset), stream.Season))
		}
		if stream.Episode != "" {
			buf.WriteString(fmt.Sprintf("%sEpisode%s: %s - %s\n", color(ColorBold), color(ColorReset),
				stream.Episode, stream.Title))
		}
	} else {
		buf.WriteString(fmt.Sprintf("%sTitle%s: %s", color(ColorBold), color(ColorReset), stream.Title))
		if stream.Year != "" {
			buf.WriteString(fmt.Sprintf(" %s(%s)%s", color(ColorCyan), stream.Year, color(ColorReset)))
		}
		buf.WriteString("\n")
	}

	// Client
	if stream.Device != "" {
		buf.WriteString(fmt.Sprintf("%sClient%s: %s (%s)\n", color(ColorBold), color(ColorReset),
			stream.Client, stream.Device))
	} else {
		buf.WriteString(fmt.Sprintf("%sClient%s: %s\n", color(ColorBold), color(ColorReset), stream.Client))
	}

	// Status
	statusColor := ColorGreen
	if stream.Transcoding {
		statusColor = ColorRed
	}
	statusText := stream.Status
	if stream.IsPaused {
		statusText += " (Paused)"
	}
	buf.WriteString(fmt.Sprintf("%sStatus%s: %s%s%s\n", color(ColorBold), color(ColorReset),
		color(statusColor), statusText, color(ColorReset)))

	// Bandwidth
	if stream.Bandwidth > 0 {
		buf.WriteString(fmt.Sprintf("%sBandwidth%s: %s%.2f Mbps%s\n", color(ColorBold), color(ColorReset),
			color(ColorMagenta), stream.Bandwidth, color(ColorReset)))
	}

	// Quality
	if stream.Resolution != "" || stream.VideoCodec != "" {
		buf.WriteString(fmt.Sprintf("%sQuality%s: ", color(ColorBold), color(ColorReset)))
		if stream.Resolution != "" {
			buf.WriteString(fmt.Sprintf("%s ", stream.Resolution))
		}
		if stream.VideoCodec != "" {
			buf.WriteString(fmt.Sprintf("(%s", stream.VideoCodec))
			if stream.AudioCodec != "" {
				buf.WriteString(fmt.Sprintf("/%s", stream.AudioCodec))
			}
			buf.WriteString(")")
		}
		buf.WriteString("\n")
	}
}

func displayEndedStreamToBuffer(buf *strings.Builder, record SessionRecord, noColor bool) {
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	stream := record.Stream

	buf.WriteString(fmt.Sprintf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		color(ColorGray), color(ColorBold), color(ColorReset)))

	// Server badge with gray coloring
	serverColor := ColorGray
	buf.WriteString(fmt.Sprintf("%s[%s]%s ", color(serverColor), strings.ToUpper(stream.Server), color(ColorReset)))
	buf.WriteString(fmt.Sprintf("%sUser%s: %s%s%s ", color(ColorGray), color(ColorReset),
		color(ColorGray), stream.User, color(ColorReset)))
	buf.WriteString(fmt.Sprintf("%s[ENDED %s]%s\n", color(ColorGray), formatTimeSince(*record.EndTime), color(ColorReset)))

	// Media info (shortened for ended sessions)
	if stream.Type == "episode" && stream.Show != "" {
		buf.WriteString(fmt.Sprintf("%sShow%s: %s", color(ColorGray), color(ColorReset), stream.Show))
		if stream.Season != "" && stream.Episode != "" {
			buf.WriteString(fmt.Sprintf(" S%sE%s", stream.Season, stream.Episode))
		}
		buf.WriteString("\n")
	} else {
		buf.WriteString(fmt.Sprintf("%sTitle%s: %s", color(ColorGray), color(ColorReset), stream.Title))
		if stream.Year != "" {
			buf.WriteString(fmt.Sprintf(" (%s)", stream.Year))
		}
		buf.WriteString("\n")
	}

	// Client
	buf.WriteString(fmt.Sprintf("%sClient%s: %s\n", color(ColorGray), color(ColorReset), stream.Client))

	// Duration
	if record.EndTime != nil {
		duration := record.EndTime.Sub(record.StartTime)
		buf.WriteString(fmt.Sprintf("%sDuration%s: %s\n", color(ColorGray), color(ColorReset),
			formatDuration(duration)))
	}
}

func formatDuration(d time.Duration) string {
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

func displayStreamSummary(streams []StreamInfo, noColor bool) {
	var buf strings.Builder
	displayStreamSummaryToBuffer(&buf, streams, noColor)
	fmt.Print(buf.String())
}

func displayStreamSummaryToBuffer(buf *strings.Builder, streams []StreamInfo, noColor bool) {
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		color(ColorBold), color(ColorCyan), color(ColorReset)))

	transcodeCount := 0
	totalBandwidth := 0.0

	for _, stream := range streams {
		if stream.Transcoding {
			transcodeCount++
		}
		totalBandwidth += stream.Bandwidth
	}

	buf.WriteString(fmt.Sprintf("%sTotal Streams%s: %d\n", color(ColorBold), color(ColorReset), len(streams)))
	buf.WriteString(fmt.Sprintf("%sTranscoding%s: %d\n", color(ColorBold), color(ColorReset), transcodeCount))
	buf.WriteString(fmt.Sprintf("%sTotal Bandwidth%s: %s%.2f Mbps%s\n", color(ColorBold), color(ColorReset),
		color(ColorMagenta), totalBandwidth, color(ColorReset)))
}
