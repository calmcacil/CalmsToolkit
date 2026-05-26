package streams

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/config"
)

const (
	ServerPlex     = "plex"
	ServerJellyfin = "jellyfin"
	ServerBoth     = "both"
)

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

type ToolConfig struct {
	ServerType      string
	PlexURL         string
	PlexToken       string
	JellyfinURL     string
	JellyfinToken   string
	Timeout         time.Duration
	NoColor         bool
	JSONOutput      bool
	WatchMode       bool
	WatchSeconds    int
	HistoryDuration time.Duration
	Quiet           bool
}

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
}

type SessionRecord struct {
	Stream    StreamInfo
	StartTime time.Time
	EndTime   *time.Time
	SessionID string
}

type SessionHistory struct {
	Records map[string]*SessionRecord
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

func Run(cfg ToolConfig) {
	if cfg.WatchMode {
		fmt.Print(colors.HideCursor)
		defer fmt.Print(colors.ShowCursor)

		history := &SessionHistory{
			Records: make(map[string]*SessionRecord),
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for {
			fmt.Print(colors.ClearScreen + colors.HomeCursor)
			if err := displayAllSessionsWithHistory(ctx, cfg, history); err != nil {
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
	if err := displayAllSessions(ctx, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func displayAllSessionsWithHistory(ctx context.Context, cfg ToolConfig, history *SessionHistory) error {
	var allStreams []StreamInfo
	var plexCount, jellyfinCount int

	client := &http.Client{Timeout: cfg.Timeout}

	if cfg.ServerType == ServerPlex || cfg.ServerType == ServerBoth {
		if streams, err := fetchPlexStreams(ctx, client, cfg); err == nil {
			allStreams = append(allStreams, streams...)
			plexCount = len(streams)
		}
	}

	if cfg.ServerType == ServerJellyfin || cfg.ServerType == ServerBoth {
		if streams, err := fetchJellyfinStreams(ctx, client, cfg); err == nil {
			allStreams = append(allStreams, streams...)
			jellyfinCount = len(streams)
		}
	}

	updateHistory(history, allStreams, cfg.HistoryDuration)

	if cfg.JSONOutput {
		return displayJSONOutput(allStreams, plexCount, jellyfinCount)
	}

	return displayTerminalOutputWithHistory(allStreams, history, plexCount, jellyfinCount, cfg.NoColor)
}

func displayAllSessions(ctx context.Context, cfg ToolConfig) error {
	var allStreams []StreamInfo
	var plexCount, jellyfinCount int

	client := &http.Client{Timeout: cfg.Timeout}

	if cfg.ServerType == ServerPlex || cfg.ServerType == ServerBoth {
		if streams, err := fetchPlexStreams(ctx, client, cfg); err == nil {
			allStreams = append(allStreams, streams...)
			plexCount = len(streams)
		}
	}

	if cfg.ServerType == ServerJellyfin || cfg.ServerType == ServerBoth {
		if streams, err := fetchJellyfinStreams(ctx, client, cfg); err == nil {
			allStreams = append(allStreams, streams...)
			jellyfinCount = len(streams)
		}
	}

	if cfg.JSONOutput {
		return displayJSONOutput(allStreams, plexCount, jellyfinCount)
	}

	return displayTerminalOutput(allStreams, plexCount, jellyfinCount, cfg.NoColor)
}

func fetchPlexStreams(ctx context.Context, client *http.Client, cfg ToolConfig) ([]StreamInfo, error) {
	url := fmt.Sprintf("%s/status/sessions?X-Plex-Token=%s", cfg.PlexURL, cfg.PlexToken)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
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

func fetchJellyfinStreams(ctx context.Context, client *http.Client, cfg ToolConfig) ([]StreamInfo, error) {
	url := fmt.Sprintf("%s/Sessions?api_key=%s", cfg.JellyfinURL, cfg.JellyfinToken)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
	var buf strings.Builder

	clr := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	buf.WriteString(fmt.Sprintf("%s%s=== Media Streams Monitor ===%s\n",
		clr(colors.Bold), clr(colors.Cyan), clr(colors.Reset)))
	buf.WriteString(fmt.Sprintf("Total Sessions: %s%d%s", clr(colors.Bold), len(streams), clr(colors.Reset)))
	if plexCount > 0 || jellyfinCount > 0 {
		buf.WriteString(fmt.Sprintf(" (Plex: %d, Jellyfin: %d)", plexCount, jellyfinCount))
	}
	buf.WriteString("\n\n")

	if len(streams) == 0 {
		buf.WriteString(fmt.Sprintf("%sNo active streams%s\n", clr(colors.Green), clr(colors.Reset)))
		fmt.Print(buf.String())
		return nil
	}

	for _, stream := range streams {
		displayStreamToBuffer(&buf, stream, noColor)
	}

	displayStreamSummaryToBuffer(&buf, streams, noColor)

	fmt.Print(buf.String())

	return nil
}

func displayTerminalOutputWithHistory(currentStreams []StreamInfo, history *SessionHistory, plexCount, jellyfinCount int, noColor bool) error {
	var buf strings.Builder

	clr := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	buf.WriteString(fmt.Sprintf("%s%s=== Media Streams Monitor ===%s\n",
		clr(colors.Bold), clr(colors.Cyan), clr(colors.Reset)))

	active, ended := getActiveAndEndedSessions(history)

	buf.WriteString(fmt.Sprintf("Active Sessions: %s%d%s", clr(colors.Bold), len(active), clr(colors.Reset)))
	if plexCount > 0 || jellyfinCount > 0 {
		buf.WriteString(fmt.Sprintf(" (Plex: %d, Jellyfin: %d)", plexCount, jellyfinCount))
	}
	buf.WriteString("\n")

	if len(ended) > 0 {
		buf.WriteString(fmt.Sprintf("Recently Ended: %s%d%s\n", clr(colors.Gray), len(ended), clr(colors.Reset)))
	}
	buf.WriteString("\n")

	if len(active) == 0 {
		buf.WriteString(fmt.Sprintf("%sNo active streams%s\n", clr(colors.Green), clr(colors.Reset)))
	} else {
		for _, record := range active {
			displayStreamToBuffer(&buf, record.Stream, noColor)
		}
	}

	if len(ended) > 0 {
		buf.WriteString("\n")
		buf.WriteString(fmt.Sprintf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
			clr(colors.Bold), clr(colors.Gray), clr(colors.Reset)))
		buf.WriteString(fmt.Sprintf("%s%sRecently Ended Sessions:%s\n\n",
			clr(colors.Bold), clr(colors.Gray), clr(colors.Reset)))

		for _, record := range ended {
			displayEndedStreamToBuffer(&buf, record, noColor)
		}
	}

	if len(active) > 0 {
		var activeStreams []StreamInfo
		for _, record := range active {
			activeStreams = append(activeStreams, record.Stream)
		}
		displayStreamSummaryToBuffer(&buf, activeStreams, noColor)
	}

	fmt.Print(buf.String())

	return nil
}

func displayStreamToBuffer(buf *strings.Builder, stream StreamInfo, noColor bool) {
	clr := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	buf.WriteString(fmt.Sprintf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		clr(colors.Bold), clr(colors.Blue), clr(colors.Reset)))

	serverColor := colors.Magenta
	if stream.Server == "plex" {
		serverColor = colors.Yellow
	}
	buf.WriteString(fmt.Sprintf("%s[%s]%s ", clr(serverColor), strings.ToUpper(stream.Server), clr(colors.Reset)))
	buf.WriteString(fmt.Sprintf("%sUser%s: %s%s%s\n", clr(colors.Bold), clr(colors.Reset),
		clr(colors.Yellow), stream.User, clr(colors.Reset)))

	if stream.Type == "episode" && stream.Show != "" {
		buf.WriteString(fmt.Sprintf("%sShow%s: %s\n", clr(colors.Bold), clr(colors.Reset), stream.Show))
		if stream.Season != "" {
			buf.WriteString(fmt.Sprintf("%sSeason%s: %s\n", clr(colors.Bold), clr(colors.Reset), stream.Season))
		}
		if stream.Episode != "" {
			buf.WriteString(fmt.Sprintf("%sEpisode%s: %s - %s\n", clr(colors.Bold), clr(colors.Reset),
				stream.Episode, stream.Title))
		}
	} else {
		buf.WriteString(fmt.Sprintf("%sTitle%s: %s", clr(colors.Bold), clr(colors.Reset), stream.Title))
		if stream.Year != "" {
			buf.WriteString(fmt.Sprintf(" %s(%s)%s", clr(colors.Cyan), stream.Year, clr(colors.Reset)))
		}
		buf.WriteString("\n")
	}

	if stream.Device != "" {
		buf.WriteString(fmt.Sprintf("%sClient%s: %s (%s)\n", clr(colors.Bold), clr(colors.Reset),
			stream.Client, stream.Device))
	} else {
		buf.WriteString(fmt.Sprintf("%sClient%s: %s\n", clr(colors.Bold), clr(colors.Reset), stream.Client))
	}

	statusColor := colors.Green
	if stream.Transcoding {
		statusColor = colors.Red
	}
	statusText := stream.Status
	if stream.IsPaused {
		statusText += " (Paused)"
	}
	buf.WriteString(fmt.Sprintf("%sStatus%s: %s%s%s\n", clr(colors.Bold), clr(colors.Reset),
		clr(statusColor), statusText, clr(colors.Reset)))

	if stream.Bandwidth > 0 {
		buf.WriteString(fmt.Sprintf("%sBandwidth%s: %s%.2f Mbps%s\n", clr(colors.Bold), clr(colors.Reset),
			clr(colors.Magenta), stream.Bandwidth, clr(colors.Reset)))
	}

	if stream.Resolution != "" || stream.VideoCodec != "" {
		buf.WriteString(fmt.Sprintf("%sQuality%s: ", clr(colors.Bold), clr(colors.Reset)))
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
	clr := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	stream := record.Stream

	buf.WriteString(fmt.Sprintf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		clr(colors.Gray), clr(colors.Bold), clr(colors.Reset)))

	serverColor := colors.Gray
	buf.WriteString(fmt.Sprintf("%s[%s]%s ", clr(serverColor), strings.ToUpper(stream.Server), clr(colors.Reset)))
	buf.WriteString(fmt.Sprintf("%sUser%s: %s%s%s ", clr(colors.Gray), clr(colors.Reset),
		clr(colors.Gray), stream.User, clr(colors.Reset)))
	buf.WriteString(fmt.Sprintf("%s[ENDED %s]%s\n", clr(colors.Gray), formatTimeSince(*record.EndTime), clr(colors.Reset)))

	if stream.Type == "episode" && stream.Show != "" {
		buf.WriteString(fmt.Sprintf("%sShow%s: %s", clr(colors.Gray), clr(colors.Reset), stream.Show))
		if stream.Season != "" && stream.Episode != "" {
			buf.WriteString(fmt.Sprintf(" S%sE%s", stream.Season, stream.Episode))
		}
		buf.WriteString("\n")
	} else {
		buf.WriteString(fmt.Sprintf("%sTitle%s: %s", clr(colors.Gray), clr(colors.Reset), stream.Title))
		if stream.Year != "" {
			buf.WriteString(fmt.Sprintf(" (%s)", stream.Year))
		}
		buf.WriteString("\n")
	}

	buf.WriteString(fmt.Sprintf("%sClient%s: %s\n", clr(colors.Gray), clr(colors.Reset), stream.Client))

	if record.EndTime != nil {
		duration := record.EndTime.Sub(record.StartTime)
		buf.WriteString(fmt.Sprintf("%sDuration%s: %s\n", clr(colors.Gray), clr(colors.Reset),
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

func displayStreamSummaryToBuffer(buf *strings.Builder, streams []StreamInfo, noColor bool) {
	clr := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	buf.WriteString("\n")
	buf.WriteString(fmt.Sprintf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		clr(colors.Bold), clr(colors.Cyan), clr(colors.Reset)))

	transcodeCount := 0
	totalBandwidth := 0.0

	for _, stream := range streams {
		if stream.Transcoding {
			transcodeCount++
		}
		totalBandwidth += stream.Bandwidth
	}

	buf.WriteString(fmt.Sprintf("%sTotal Streams%s: %d\n", clr(colors.Bold), clr(colors.Reset), len(streams)))
	buf.WriteString(fmt.Sprintf("%sTranscoding%s: %d\n", clr(colors.Bold), clr(colors.Reset), transcodeCount))
	buf.WriteString(fmt.Sprintf("%sTotal Bandwidth%s: %s%.2f Mbps%s\n", clr(colors.Bold), clr(colors.Reset),
		clr(colors.Magenta), totalBandwidth, clr(colors.Reset)))
}
