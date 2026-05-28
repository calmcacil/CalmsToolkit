package streams

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
	httpclient "github.com/calmcacil/CalmsToolkit/internal/http"
)

const (
	// ServerPlex indicates the Plex server type.
	ServerPlex = "plex"
	// ServerJellyfin indicates the Jellyfin server type.
	ServerJellyfin = "jellyfin"
	// ServerBoth indicates both Plex and Jellyfin server types.
	ServerBoth = "both"
)

// PlexMediaContainer is the top-level XML response from Plex /status/sessions.
type PlexMediaContainer struct {
	XMLName xml.Name    `xml:"MediaContainer"`
	Size    int         `xml:"size,attr"`
	Videos  []PlexVideo `xml:"Video"`
	Tracks  []PlexTrack `xml:"Track"`
}

// PlexVideo represents a video stream from Plex.
type PlexVideo struct {
	Title            string                `xml:"title,attr"`
	Year             string                `xml:"year,attr"`
	Type             string                `xml:"type,attr"`
	GrandparentTitle string                `xml:"grandparentTitle,attr"`
	ParentIndex      string                `xml:"parentIndex,attr"`
	Index            string                `xml:"index,attr"`
	ViewOffset       int                   `xml:"viewOffset,attr"`
	User             PlexUser              `xml:"User"`
	Player           PlexPlayer            `xml:"Player"`
	Session          PlexSession           `xml:"Session"`
	Media            []PlexMedia           `xml:"Media"`
	TranscodeSession *PlexTranscodeSession `xml:"TranscodeSession"`
}

// PlexTrack represents a music track stream from Plex.
type PlexTrack struct {
	Title            string      `xml:"title,attr"`
	GrandparentTitle string      `xml:"grandparentTitle,attr"`
	User             PlexUser    `xml:"User"`
	Player           PlexPlayer  `xml:"Player"`
	Session          PlexSession `xml:"Session"`
	Media            []PlexMedia `xml:"Media"`
}

// PlexUser represents a Plex user.
type PlexUser struct {
	Title string `xml:"title,attr"`
}

// PlexPlayer represents a Plex playback client.
type PlexPlayer struct {
	Title  string `xml:"title,attr"`
	Device string `xml:"device,attr"`
}

// PlexSession contains session metadata from Plex.
type PlexSession struct {
	Bandwidth int `xml:"bandwidth,attr"`
}

// PlexMedia represents media metadata from Plex.
type PlexMedia struct {
	VideoResolution string     `xml:"videoResolution,attr"`
	VideoCodec      string     `xml:"videoCodec,attr"`
	AudioCodec      string     `xml:"audioCodec,attr"`
	Duration        int        `xml:"duration,attr"`
	Parts           []PlexPart `xml:"Part"`
}

// PlexPart represents a media part from Plex.
type PlexPart struct {
	VideoDecision string `xml:"videoDecision,attr"`
	AudioDecision string `xml:"audioDecision,attr"`
}

// PlexTranscodeSession represents a Plex transcode session.
type PlexTranscodeSession struct {
	VideoDecision string `xml:"videoDecision,attr"`
	AudioDecision string `xml:"audioDecision,attr"`
}

// JellyfinSession represents a Jellyfin playback session.
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

// JellyfinPlayState represents the playback state from Jellyfin.
type JellyfinPlayState struct {
	PositionTicks int64  `json:"PositionTicks"`
	IsPaused      bool   `json:"IsPaused"`
	PlayMethod    string `json:"PlayMethod"`
}

// JellyfinNowPlayingItem represents the currently playing item from Jellyfin.
type JellyfinNowPlayingItem struct {
	Name              string                `json:"Name"`
	SeriesName        string                `json:"SeriesName"`
	Type              string                `json:"Type"`
	ProductionYear    int                   `json:"ProductionYear"`
	IndexNumber       int                   `json:"IndexNumber"`
	ParentIndexNumber int                   `json:"ParentIndexNumber"`
	RunTimeTicks      int64                 `json:"RunTimeTicks"`
	MediaStreams      []JellyfinMediaStream `json:"MediaStreams"`
}

// JellyfinMediaStream represents a media stream from Jellyfin.
type JellyfinMediaStream struct {
	Type   string `json:"Type"`
	Codec  string `json:"Codec"`
	Height int    `json:"Height"`
}

// JellyfinTranscodingInfo contains transcode details from Jellyfin.
type JellyfinTranscodingInfo struct {
	IsVideoDirect    bool     `json:"IsVideoDirect"`
	IsAudioDirect    bool     `json:"IsAudioDirect"`
	Bitrate          int      `json:"Bitrate"`
	VideoCodec       string   `json:"VideoCodec"`
	AudioCodec       string   `json:"AudioCodec"`
	Height           int      `json:"Height"`
	TranscodeReasons []string `json:"TranscodeReasons"`
}

// ToolConfig holds configuration for the media streams tool.
type ToolConfig struct {
	ServerType      string
	PlexURL         string
	PlexToken       string
	JellyfinURL     string
	JellyfinToken   string
	Timeout         time.Duration
	NoColor         bool
	Theme           string
	JSONOutput      bool
	WatchMode       bool
	WatchSeconds    int
	HistoryDuration time.Duration
	Quiet           bool
}

// StreamInfo describes a single media stream.
type StreamInfo struct {
	Server      string  `json:"server"`
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
	Progress    float64 `json:"progress,omitempty"`
}

// SessionRecord tracks a stream session with start and optional end time.
type SessionRecord struct {
	Stream    StreamInfo
	StartTime time.Time
	EndTime   *time.Time
	SessionID string
}

// SessionHistory maintains a map of session records for change detection.
type SessionHistory struct {
	Records map[string]*SessionRecord
}

// Summary provides an overview of current streams.
type Summary struct {
	TotalStreams     int          `json:"total_streams"`
	TranscodingCount int          `json:"transcoding_count"`
	TotalBandwidth   float64      `json:"total_bandwidth_mbps"`
	PlexStreams      int          `json:"plex_streams"`
	JellyfinStreams  int          `json:"jellyfin_streams"`
	Timestamp        time.Time    `json:"timestamp"`
	Streams          []StreamInfo `json:"streams"`
}

// BuildToolConfig constructs a ToolConfig from the global toolkit configuration.
func BuildToolConfig(tk *config.ToolkitConfig) ToolConfig {
	cfg := ToolConfig{}
	if tk == nil {
		cfg.Timeout = 10 * time.Second
		cfg.WatchSeconds = 10
		cfg.HistoryDuration = 15 * time.Minute
		cfg.ServerType = "both"
		return cfg
	}
	dur, _ := time.ParseDuration(tk.General.Timeout)
	if dur <= 0 {
		dur = 10 * time.Second
	}
	cfg.Timeout = dur
	cfg.NoColor = tk.General.NoColor
	cfg.Theme = tk.General.Theme
	cfg.PlexURL = strings.TrimSuffix(tk.MediaStreams.PlexURL, "/")
	cfg.PlexToken = tk.MediaStreams.PlexToken
	cfg.JellyfinURL = strings.TrimSuffix(tk.MediaStreams.JellyfinURL, "/")
	cfg.JellyfinToken = tk.MediaStreams.JellyfinToken
	cfg.ServerType = tk.MediaStreams.ServerType
	if cfg.ServerType == "" {
		cfg.ServerType = "both"
	}
	cfg.WatchSeconds = tk.MediaStreams.WatchInterval
	if cfg.WatchSeconds <= 0 {
		cfg.WatchSeconds = 10
	}
	dur, _ = time.ParseDuration(tk.MediaStreams.HistoryDuration)
	if dur > 0 {
		cfg.HistoryDuration = dur
	} else {
		cfg.HistoryDuration = 15 * time.Minute
	}
	return cfg
}

// Run executes the media streams monitor tool.
func Run(cfg ToolConfig) {
	p := colors.GetPalette(cfg.Theme)

	if cfg.WatchMode {
		fmt.Print(colors.HideCursor)
		defer fmt.Print(colors.ShowCursor)

		history := &SessionHistory{
			Records: make(map[string]*SessionRecord),
		}

		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		fmt.Print(colors.ClearScreen + colors.HomeCursor)

		var lastHash string

		for {
			if err := displayAllSessionsWithHistory(ctx, cfg, history, &lastHash, p); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			}
			select {
			case <-ctx.Done():
				fmt.Println("\nShutting down.")
				return
			case <-time.After(time.Duration(cfg.WatchSeconds) * time.Second):
			}
		}
	}

	ctx := context.Background()
	if err := displayAllSessions(ctx, cfg, p); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func displayAllSessionsWithHistory(ctx context.Context, cfg ToolConfig, history *SessionHistory, lastHash *string, p *colors.Palette) error {
	var allStreams []StreamInfo
	var plexCount, jellyfinCount int
	var plexErr, jellyfinErr error

	client := httpclient.NewClient(cfg.Timeout)

	if cfg.ServerType == ServerPlex || cfg.ServerType == ServerBoth {
		streams, err := fetchPlexStreams(ctx, client, cfg)
		if err == nil {
			allStreams = append(allStreams, streams...)
			plexCount = len(streams)
		} else {
			plexErr = err
		}
	}

	if cfg.ServerType == ServerJellyfin || cfg.ServerType == ServerBoth {
		streams, err := fetchJellyfinStreams(ctx, client, cfg)
		if err == nil {
			allStreams = append(allStreams, streams...)
			jellyfinCount = len(streams)
		} else {
			jellyfinErr = err
		}
	}

	updateHistory(history, allStreams, cfg.HistoryDuration)

	if allFailed(cfg.ServerType, plexErr, jellyfinErr) {
		return fmt.Errorf("all servers failed: plex: %v, jellyfin: %v", maybeError(plexErr), maybeError(jellyfinErr))
	}

	if cfg.JSONOutput {
		return displayJSONOutput(allStreams, plexCount, jellyfinCount)
	}

	newHash := computeStreamsHash(history)
	if *lastHash == newHash {
		return nil
	}
	*lastHash = newHash

	return displayTerminalOutputWithHistory(allStreams, history, plexCount, jellyfinCount, cfg.NoColor, p)
}

func displayAllSessions(ctx context.Context, cfg ToolConfig, p *colors.Palette) error {
	var allStreams []StreamInfo
	var plexCount, jellyfinCount int
	var plexErr, jellyfinErr error

	client := httpclient.NewClient(cfg.Timeout)

	if cfg.ServerType == ServerPlex || cfg.ServerType == ServerBoth {
		streams, err := fetchPlexStreams(ctx, client, cfg)
		if err == nil {
			allStreams = append(allStreams, streams...)
			plexCount = len(streams)
		} else {
			plexErr = err
		}
	}

	if cfg.ServerType == ServerJellyfin || cfg.ServerType == ServerBoth {
		streams, err := fetchJellyfinStreams(ctx, client, cfg)
		if err == nil {
			allStreams = append(allStreams, streams...)
			jellyfinCount = len(streams)
		} else {
			jellyfinErr = err
		}
	}

	if allFailed(cfg.ServerType, plexErr, jellyfinErr) {
		return fmt.Errorf("all servers failed: plex: %v, jellyfin: %v", maybeError(plexErr), maybeError(jellyfinErr))
	}

	if cfg.JSONOutput {
		return displayJSONOutput(allStreams, plexCount, jellyfinCount)
	}

	return displayTerminalOutput(allStreams, plexCount, jellyfinCount, cfg.NoColor, p)
}

func allFailed(serverType string, plexErr, jellyfinErr error) bool {
	needsPlex := serverType == ServerPlex || serverType == ServerBoth
	needsJellyfin := serverType == ServerJellyfin || serverType == ServerBoth
	if serverType == ServerBoth {
		return (needsPlex && plexErr != nil) && (needsJellyfin && jellyfinErr != nil)
	}
	return (needsPlex && plexErr != nil) || (needsJellyfin && jellyfinErr != nil)
}

func maybeError(err error) string {
	if err != nil {
		return err.Error()
	}
	return "not configured"
}

func fetchPlexStreams(ctx context.Context, client *httpclient.Client, cfg ToolConfig) ([]StreamInfo, error) {
	url := fmt.Sprintf("%s/status/sessions", cfg.PlexURL)
	headers := map[string]string{
		"X-Plex-Token": cfg.PlexToken,
	}

	var container PlexMediaContainer
	if err := client.DoXML(ctx, "GET", url, headers, nil, &container); err != nil {
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

func fetchJellyfinStreams(ctx context.Context, client *httpclient.Client, cfg ToolConfig) ([]StreamInfo, error) {
	url := fmt.Sprintf("%s/Sessions", cfg.JellyfinURL)
	headers := map[string]string{
		"Accept":    "application/json",
		"X-API-Key": cfg.JellyfinToken,
	}

	var sessions []JellyfinSession
	if err := client.DoJSON(ctx, "GET", url, headers, nil, &sessions); err != nil {
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

func generateSessionID(stream StreamInfo) string {
	return fmt.Sprintf("%s:%s:%s:%s", stream.Server, stream.User, stream.Title, stream.Client)
}

func updateHistory(history *SessionHistory, currentStreams []StreamInfo, historyDuration time.Duration) {
	now := time.Now()
	currentSessionIDs := make(map[string]bool)

	for _, stream := range currentStreams {
		sessionID := generateSessionID(stream)
		currentSessionIDs[sessionID] = true

		if _, exists := history.Records[sessionID]; !exists {
			history.Records[sessionID] = &SessionRecord{
				Stream:    stream,
				StartTime: now,
				EndTime:   nil,
				SessionID: sessionID,
			}
		} else {
			history.Records[sessionID].Stream = stream
			history.Records[sessionID].EndTime = nil
		}
	}

	for sessionID, record := range history.Records {
		if !currentSessionIDs[sessionID] && record.EndTime == nil {
			record.EndTime = &now
		}
	}

	for sessionID, record := range history.Records {
		if record.EndTime != nil && now.Sub(*record.EndTime) > historyDuration {
			delete(history.Records, sessionID)
		}
	}
}

func computeStreamsHash(history *SessionHistory) string {
	records := make([]*SessionRecord, 0, len(history.Records))
	for _, r := range history.Records {
		records = append(records, r)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].SessionID < records[j].SessionID
	})
	data, _ := json.Marshal(records)
	h := sha256.Sum256(data)
	return string(h[:])
}

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

	if len(video.Media) > 0 && video.Media[0].Duration > 0 && video.ViewOffset > 0 {
		pct := float64(video.ViewOffset) / float64(video.Media[0].Duration) * 100.0
		if pct > 100 {
			pct = 100
		}
		stream.Progress = pct
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

	if session.NowPlayingItem != nil && session.NowPlayingItem.RunTimeTicks > 0 && session.PlayState.PositionTicks > 0 {
		pct := float64(session.PlayState.PositionTicks) / float64(session.NowPlayingItem.RunTimeTicks) * 100.0
		if pct > 100 {
			pct = 100
		}
		stream.Progress = pct
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

func renderProgressBar(pct float64, width int) string {
	if pct <= 0 {
		return strings.Repeat(" ", width)
	}
	filled := int(pct * float64(width) / 100.0)
	if filled > width {
		filled = width
	}
	empty := width - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

const maxBoxInnerWidth = 120

func getTermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

func getBoxWidth(termW int) (inner, outer int) {
	outer = termW
	inner = outer - 2
	if inner < 40 {
		inner = 40
		outer = inner + 2
	}
	if inner > maxBoxInnerWidth {
		inner = maxBoxInnerWidth
		outer = inner + 2
	}
	return
}

func buildServerLabel(plexCount, jellyfinCount int) string {
	var parts []string
	if plexCount > 0 {
		parts = append(parts, fmt.Sprintf("Plex: %d", plexCount))
	}
	if jellyfinCount > 0 {
		parts = append(parts, fmt.Sprintf("Jellyfin: %d", jellyfinCount))
	}
	return strings.Join(parts, ", ")
}

func boxStreamTop(bw *bufio.Writer, termW int) {
	fmt.Fprint(bw, "┌")
	fmt.Fprint(bw, strings.Repeat("─", termW-2))
	fmt.Fprint(bw, "┐\n")
}

func boxStreamSep(bw *bufio.Writer, termW int) {
	fmt.Fprint(bw, "├")
	fmt.Fprint(bw, strings.Repeat("─", termW-2))
	fmt.Fprint(bw, "┤\n")
}

func boxStreamBottom(bw *bufio.Writer, termW int) {
	fmt.Fprint(bw, "└")
	fmt.Fprint(bw, strings.Repeat("─", termW-2))
	fmt.Fprint(bw, "┘\n")
}

func displayTerminalOutput(streams []StreamInfo, plexCount, jellyfinCount int, noColor bool, p *colors.Palette) error {
	clr := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	rawW := getTermWidth()
	boxW, termW := getBoxWidth(rawW)

	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	// Title banner
	title := "Media Streams Monitor"
	prefix := "┌── " + title + " ──"
	rc := colors.VisibleLen(prefix)
	padLen := termW - rc - 1
	if padLen < 0 {
		padLen = 0
	}
	fmt.Fprint(bw, prefix)
	fmt.Fprint(bw, strings.Repeat("─", padLen))
	fmt.Fprint(bw, "┐\n")

	// Header
	serverLabel := buildServerLabel(plexCount, jellyfinCount)
	header := "Active Sessions"
	if len(streams) == 0 {
		header = "Active Sessions: 0"
	} else {
		header = fmt.Sprintf("Active Sessions: %s%d%s", clr(p.Bold), len(streams), clr(p.Reset))
		if serverLabel != "" {
			header += fmt.Sprintf(" (%s)", serverLabel)
		}
	}
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(" "+header+" ", boxW))
	fmt.Fprint(bw, "│\n")

	// Empty state
	if len(streams) == 0 {
		boxStreamBottom(bw, termW)
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, clr(p.Success))
		fmt.Fprint(bw, colors.PadRight(" No active streams ", boxW))
		fmt.Fprint(bw, clr(p.Reset))
		fmt.Fprint(bw, "│\n")
		boxStreamBottom(bw, termW)
		bw.Flush()
		os.Stdout.Write(buf.Bytes())
		return nil
	}

	// Close outer box before grid
	boxStreamBottom(bw, termW)

	// Stream grid
	renderStreamGrid(bw, streams, termW, noColor, p)

	// Summary standalone box
	boxStreamTop(bw, termW)
	displayStreamSummaryToBox(bw, streams, boxW, noColor, p)
	boxStreamBottom(bw, termW)

	bw.Flush()
	os.Stdout.Write(buf.Bytes())
	return nil
}

func displayTerminalOutputWithHistory(currentStreams []StreamInfo, history *SessionHistory, plexCount, jellyfinCount int, noColor bool, p *colors.Palette) error {
	clr := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	rawW := getTermWidth()
	boxW, termW := getBoxWidth(rawW)

	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	fmt.Fprint(bw, colors.ClearScreen+colors.HomeCursor)

	// Title
	title := "Media Streams Monitor"
	prefix := "┌── " + title + " ──"
	rc := colors.VisibleLen(prefix)
	padLen := termW - rc - 1
	if padLen < 0 {
		padLen = 0
	}
	fmt.Fprint(bw, prefix)
	fmt.Fprint(bw, strings.Repeat("─", padLen))
	fmt.Fprint(bw, "┐\n")

	active, ended := getActiveAndEndedSessions(history)
	serverLabel := buildServerLabel(plexCount, jellyfinCount)

	// Active count
	header := fmt.Sprintf("Active Sessions: %s%d%s", clr(p.Bold), len(active), clr(p.Reset))
	if serverLabel != "" {
		header += fmt.Sprintf(" (%s)", serverLabel)
	}
	if len(ended) > 0 {
		header += fmt.Sprintf("    Ended: %s%d%s", clr(p.Subdued), len(ended), clr(p.Reset))
	}
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(" "+header+" ", boxW))
	fmt.Fprint(bw, "│\n")

	if len(active) == 0 && len(ended) == 0 {
		boxStreamSep(bw, termW)
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, clr(p.Success))
		fmt.Fprint(bw, colors.PadRight(" No active streams ", boxW))
		fmt.Fprint(bw, clr(p.Reset))
		fmt.Fprint(bw, "│\n")
		boxStreamBottom(bw, termW)
		fmt.Fprint(bw, colors.EraseDown)
		bw.Flush()
		os.Stdout.Write(buf.Bytes())
		return nil
	}

	// Active streams as standalone grid
	if len(active) > 0 {
		sort.Slice(active, func(i, j int) bool {
			return active[i].StartTime.Before(active[j].StartTime)
		})
		boxStreamBottom(bw, termW)
		var activeStreams []StreamInfo
		for _, record := range active {
			activeStreams = append(activeStreams, record.Stream)
		}
		renderStreamGrid(bw, activeStreams, termW, noColor, p)
	}

	// Ended sessions standalone box
	if len(ended) > 0 {
		boxStreamTop(bw, termW)
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, clr(p.Subdued))
		fmt.Fprint(bw, colors.PadRight(" Recently Ended Sessions ", boxW))
		fmt.Fprint(bw, clr(p.Reset))
		fmt.Fprint(bw, "│\n")

		for _, record := range ended {
			displayEndedStreamToBox(bw, record, boxW, noColor, p)
		}
		boxStreamBottom(bw, termW)
	}

	// Summary standalone box
	if len(active) > 0 {
		boxStreamTop(bw, termW)
		var activeStreams []StreamInfo
		for _, record := range active {
			activeStreams = append(activeStreams, record.Stream)
		}
		displayStreamSummaryToBox(bw, activeStreams, boxW, noColor, p)
		boxStreamBottom(bw, termW)
	}

	fmt.Fprint(bw, colors.EraseDown)
	bw.Flush()
	os.Stdout.Write(buf.Bytes())
	return nil
}

func displayStreamToBox(bw *bufio.Writer, stream StreamInfo, boxW int, noColor bool, p *colors.Palette) {
	clr := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	serverColor := p.ServerJellyfin
	if stream.Server == "plex" {
		serverColor = p.ServerPlex
	}

	// Server + User line
	line := fmt.Sprintf(" %s%s%s %sUser%s: %s%s%s",
		clr(serverColor), strings.ToUpper(stream.Server), clr(p.Reset),
		clr(p.Bold), clr(p.Reset),
		clr(p.ServerPlex), stream.User, clr(p.Reset))
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(line, boxW))
	fmt.Fprint(bw, "│\n")

	// Show/Title line
	if stream.Type == "episode" && stream.Show != "" {
		line := fmt.Sprintf(" %sShow%s: %s", clr(p.Bold), clr(p.Reset), stream.Show)
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
		if stream.Season != "" || stream.Episode != "" {
			epStr := ""
			if stream.Season != "" {
				epStr += fmt.Sprintf("S%02s", stream.Season)
			}
			if stream.Episode != "" {
				epStr += fmt.Sprintf("E%02s", stream.Episode)
			}
			line := fmt.Sprintf("  %s - %s", epStr, stream.Title)
			fmt.Fprint(bw, "│")
			fmt.Fprint(bw, colors.PadRight(line, boxW))
			fmt.Fprint(bw, "│\n")
		}
	} else {
		line := fmt.Sprintf(" %sTitle%s: %s", clr(p.Bold), clr(p.Reset), stream.Title)
		if stream.Year != "" {
			line += fmt.Sprintf(" %s(%s)%s", clr(p.Accent), stream.Year, clr(p.Reset))
		}
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	}

	// Client line
	if stream.Client != "" {
		var clientLine string
		if stream.Device != "" {
			clientLine = fmt.Sprintf(" %sClient%s: %s (%s)", clr(p.Bold), clr(p.Reset), stream.Client, stream.Device)
		} else {
			clientLine = fmt.Sprintf(" %sClient%s: %s", clr(p.Bold), clr(p.Reset), stream.Client)
		}
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(clientLine, boxW))
		fmt.Fprint(bw, "│\n")
	}

	// Status line
	statusColor := p.Success
	if stream.Transcoding {
		statusColor = p.Error
	}
	statusText := stream.Status
	if stream.IsPaused {
		statusText += " (Paused)"
	}
	line = fmt.Sprintf(" %sStatus%s: %s%s%s", clr(p.Bold), clr(p.Reset), clr(statusColor), statusText, clr(p.Reset))
	if stream.Transcoding {
		line += " ⚠"
	}
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(line, boxW))
	fmt.Fprint(bw, "│\n")

	// Bandwidth line
	if stream.Bandwidth > 0 {
		line := fmt.Sprintf(" %sBandwidth%s: %s%.2f Mbps%s", clr(p.Bold), clr(p.Reset), clr(p.Bandwidth), stream.Bandwidth, clr(p.Reset))
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	}

	// Quality line
	if stream.Resolution != "" || stream.VideoCodec != "" {
		line := fmt.Sprintf(" %sQuality%s: ", clr(p.Bold), clr(p.Reset))
		if stream.Resolution != "" {
			line += fmt.Sprintf("%s ", stream.Resolution)
		}
		if stream.VideoCodec != "" {
			line += fmt.Sprintf("(%s", stream.VideoCodec)
			if stream.AudioCodec != "" {
				line += fmt.Sprintf("/%s", stream.AudioCodec)
			}
			line += ")"
		}
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	}

	// Progress bar
	if stream.Progress > 0 {
		barW := boxW - 18
		if barW > 30 {
			barW = 30
		}
		if barW < 10 {
			barW = 10
		}
		bar := renderProgressBar(stream.Progress, barW)
		line := fmt.Sprintf(" %sPlayback%s: %s %s%5.1f%%%s",
			clr(p.Bold), clr(p.Reset),
			bar,
			clr(p.Accent), stream.Progress, clr(p.Reset))
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	}
}

// streamContentLines returns the padded interior content of a stream box
// (without border characters), one element per line.
func streamContentLines(stream StreamInfo, boxW int, noColor bool, p *colors.Palette) []string {
	clr := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	trunc := func(s string, maxW int) string {
		return runewidth.Truncate(s, maxW, "…")
	}

	var lines []string

	serverColor := p.ServerJellyfin
	if stream.Server == "plex" {
		serverColor = p.ServerPlex
	}

	userW := boxW - 12 // " PLEX User: "
	user := trunc(stream.User, userW)
	line := fmt.Sprintf(" %s%s%s %sUser%s: %s%s%s",
		clr(serverColor), strings.ToUpper(stream.Server), clr(p.Reset),
		clr(p.Bold), clr(p.Reset),
		clr(p.ServerPlex), user, clr(p.Reset))
	lines = append(lines, colors.PadRight(line, boxW))

	if stream.Type == "episode" && stream.Show != "" {
		showW := boxW - 9 // " Show: "
		show := trunc(stream.Show, showW)
		line := fmt.Sprintf(" %sShow%s: %s", clr(p.Bold), clr(p.Reset), show)
		lines = append(lines, colors.PadRight(line, boxW))
		if stream.Season != "" || stream.Episode != "" {
			epStr := ""
			if stream.Season != "" {
				epStr += fmt.Sprintf("S%02s", stream.Season)
			}
			if stream.Episode != "" {
				epStr += fmt.Sprintf("E%02s", stream.Episode)
			}
			line := fmt.Sprintf(" %sEpisode%s: %s", clr(p.Bold), clr(p.Reset), epStr)
			lines = append(lines, colors.PadRight(line, boxW))
		}
	} else {
		titleW := boxW - 11 // " Title:  (YYYY)"
		title := trunc(stream.Title, titleW)
		line := fmt.Sprintf(" %sTitle%s: %s", clr(p.Bold), clr(p.Reset), title)
		if stream.Year != "" {
			line += fmt.Sprintf(" %s(%s)%s", clr(p.Accent), stream.Year, clr(p.Reset))
		}
		lines = append(lines, colors.PadRight(line, boxW))
	}

	if stream.Client != "" {
		var clientLine string
		if stream.Device != "" && stream.Device != stream.Client {
			// " Client: X (Y)" — overhead: " Client:  ()" = 12
			availW := boxW - 12
			devW := runewidth.StringWidth(stream.Device)
			cliW := availW - devW
			if cliW < 4 {
				cliW = availW * 2 / 5
				if cliW < 4 {
					cliW = 4
				}
				devW = availW - cliW
			}
			client := trunc(stream.Client, cliW)
			dev := trunc(stream.Device, devW)
			clientLine = fmt.Sprintf(" %sClient%s: %s (%s)", clr(p.Bold), clr(p.Reset), client, dev)
		} else {
			client := trunc(stream.Client, boxW-11)
			clientLine = fmt.Sprintf(" %sClient%s: %s", clr(p.Bold), clr(p.Reset), client)
		}
		lines = append(lines, colors.PadRight(clientLine, boxW))
	}

	statusColor := p.Success
	if stream.Transcoding {
		statusColor = p.Error
	}
	statusText := stream.Status
	if stream.IsPaused {
		statusText += " (Paused)"
	}
	line = fmt.Sprintf(" %sStatus%s: %s%s%s", clr(p.Bold), clr(p.Reset), clr(statusColor), statusText, clr(p.Reset))
	if stream.Transcoding {
		line += " ⚠"
	}
	lines = append(lines, colors.PadRight(line, boxW))

	if stream.Bandwidth > 0 {
		line := fmt.Sprintf(" %sBandwidth%s: %s%.2f Mbps%s", clr(p.Bold), clr(p.Reset), clr(p.Bandwidth), stream.Bandwidth, clr(p.Reset))
		lines = append(lines, colors.PadRight(line, boxW))
	}

	if stream.Resolution != "" || stream.VideoCodec != "" {
		line := fmt.Sprintf(" %sQuality%s: ", clr(p.Bold), clr(p.Reset))
		if stream.Resolution != "" {
			line += fmt.Sprintf("%s ", stream.Resolution)
		}
		if stream.VideoCodec != "" {
			line += fmt.Sprintf("(%s", stream.VideoCodec)
			if stream.AudioCodec != "" {
				line += fmt.Sprintf("/%s", stream.AudioCodec)
			}
			line += ")"
		}
		lines = append(lines, colors.PadRight(line, boxW))
	}

	if stream.Progress > 0 {
		barW := boxW - 18
		if barW > 30 {
			barW = 30
		}
		if barW < 10 {
			barW = 10
		}
		bar := renderProgressBar(stream.Progress, barW)
		line := fmt.Sprintf(" %sPlayback%s: %s %s%5.1f%%%s",
			clr(p.Bold), clr(p.Reset),
			bar,
			clr(p.Accent), stream.Progress, clr(p.Reset))
		lines = append(lines, colors.PadRight(line, boxW))
	}

	return lines
}

// renderStreamGrid renders a set of stream boxes in a side-by-side grid that
// wraps to new rows when the terminal width is exhausted.
func renderStreamGrid(bw *bufio.Writer, streams []StreamInfo, termW int, noColor bool, p *colors.Palette) {
	if len(streams) == 0 {
		return
	}

	const prefBoxInnerW = 48
	boxFullW := prefBoxInnerW + 2
	numCols := termW / boxFullW
	if numCols < 1 {
		numCols = 1
	}
	if numCols > len(streams) {
		numCols = len(streams)
	}

	innerW := (termW - 1 - (numCols - 1)) / numCols
	if innerW > prefBoxInnerW {
		innerW = prefBoxInnerW
	}
	if innerW < 20 {
		innerW = 20
	}
	innerW-- // account for border chars

	type box struct {
		lines []string
	}
	boxes := make([]box, len(streams))
	maxLines := 0
	for i, s := range streams {
		l := streamContentLines(s, innerW, noColor, p)
		boxes[i] = box{lines: l}
		if len(l) > maxLines {
			maxLines = len(l)
		}
	}

	for i := 0; i < len(boxes); i += numCols {
		end := i + numCols
		if end > len(boxes) {
			end = len(boxes)
		}
		row := boxes[i:end]

		for range row {
			fmt.Fprint(bw, "┌"+strings.Repeat("─", innerW)+"┐")
		}
		fmt.Fprintln(bw)

		for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
			for _, b := range row {
				fmt.Fprint(bw, "│")
				if lineIdx < len(b.lines) {
					fmt.Fprint(bw, b.lines[lineIdx])
				} else {
					fmt.Fprint(bw, strings.Repeat(" ", innerW))
				}
				fmt.Fprint(bw, "│")
			}
			fmt.Fprintln(bw)
		}

		for range row {
			fmt.Fprint(bw, "└"+strings.Repeat("─", innerW)+"┘")
		}
		fmt.Fprintln(bw)
	}
}

func displayEndedStreamToBox(bw *bufio.Writer, record SessionRecord, boxW int, noColor bool, p *colors.Palette) {
	clr := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}
	stream := record.Stream

	endedStr := fmt.Sprintf("%s[ENDED %s]%s", clr(p.Subdued), formatTimeSince(*record.EndTime), clr(p.Reset))
	line := fmt.Sprintf(" %s%s%s %sUser%s: %s %s",
		clr(p.Subdued), strings.ToUpper(stream.Server), clr(p.Reset),
		clr(p.Bold), clr(p.Reset), stream.User, endedStr)
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(line, boxW))
	fmt.Fprint(bw, "│\n")

	if stream.Type == "episode" && stream.Show != "" {
		line := fmt.Sprintf(" %sShow%s: %s", clr(p.Subdued), clr(p.Reset), stream.Show)
		if stream.Season != "" && stream.Episode != "" {
			line += fmt.Sprintf(" S%sE%s", stream.Season, stream.Episode)
		}
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	} else {
		line := fmt.Sprintf(" %sTitle%s: %s", clr(p.Subdued), clr(p.Reset), stream.Title)
		if stream.Year != "" {
			line += fmt.Sprintf(" (%s)", stream.Year)
		}
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	}

	line = fmt.Sprintf(" %sClient%s: %s", clr(p.Subdued), clr(p.Reset), stream.Client)
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(line, boxW))
	fmt.Fprint(bw, "│\n")

	if record.EndTime != nil {
		duration := record.EndTime.Sub(record.StartTime)
		line = fmt.Sprintf(" %sDuration%s: %s", clr(p.Subdued), clr(p.Reset), formatDuration(duration))
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
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

func displayStreamSummaryToBox(bw *bufio.Writer, streams []StreamInfo, boxW int, noColor bool, p *colors.Palette) {
	clr := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	transcodeCount := 0
	totalBandwidth := 0.0
	for _, stream := range streams {
		if stream.Transcoding {
			transcodeCount++
		}
		totalBandwidth += stream.Bandwidth
	}

	line := fmt.Sprintf(" %sTotal Streams%s: %d    %sTranscoding%s: %d    %sBandwidth%s: %.2f Mbps",
		clr(p.Bold), clr(p.Reset), len(streams),
		clr(p.Bold), clr(p.Reset), transcodeCount,
		clr(p.Bold), clr(p.Reset), totalBandwidth)
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(line, boxW))
	fmt.Fprint(bw, "│\n")
}
