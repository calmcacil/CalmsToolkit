package main

import (
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
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
	ColorBold    = "\033[1m"
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
	ServerType    string
	PlexURL       string
	PlexToken     string
	JellyfinURL   string
	JellyfinToken string
	Timeout       time.Duration
	JSONOutput    bool
	WatchMode     bool
	WatchSeconds  int
	NoColor       bool
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
		serverType    = flag.String("server", "both", "Server type: plex, jellyfin, or both")
		plexURL       = flag.String("plex-url", "", "Plex server URL")
		plexToken     = flag.String("plex-token", "", "Plex authentication token")
		jellyfinURL   = flag.String("jellyfin-url", "", "Jellyfin server URL")
		jellyfinToken = flag.String("jellyfin-token", "", "Jellyfin API token")
		timeout       = flag.Duration("timeout", 10*time.Second, "Connection timeout")
		noColor       = flag.Bool("no-color", false, "Disable colored output")
		jsonOutput    = flag.Bool("json", false, "Output in JSON format")
		watchMode     = flag.Bool("watch", false, "Continuously monitor streams")
		watchSeconds  = flag.Int("interval", 5, "Watch mode refresh interval in seconds")
	)
	flag.Parse()

	config := loadConfig(*serverType, *plexURL, *plexToken, *jellyfinURL, *jellyfinToken,
		*timeout, *noColor, *jsonOutput, *watchMode, *watchSeconds)

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
		for {
			clearScreen()
			if err := displayAllSessions(config); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			}
			time.Sleep(time.Duration(config.WatchSeconds) * time.Second)
		}
	}

	// Single execution
	if err := displayAllSessions(config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(serverType, plexURL, plexToken, jellyfinURL, jellyfinToken string,
	timeout time.Duration, noColor, jsonOutput, watchMode bool, watchSeconds int) Config {

	config := Config{
		ServerType:    strings.ToLower(serverType),
		PlexURL:       "http://localhost:32400",
		PlexToken:     "",
		JellyfinURL:   "http://localhost:8096",
		JellyfinToken: "",
		Timeout:       timeout,
		JSONOutput:    jsonOutput,
		WatchMode:     watchMode,
		WatchSeconds:  watchSeconds,
		NoColor:       noColor || jsonOutput,
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
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s=== Media Streams Monitor ===%s\n",
		color(ColorBold), color(ColorCyan), color(ColorReset))
	fmt.Printf("Total Sessions: %s%d%s", color(ColorBold), len(streams), color(ColorReset))
	if plexCount > 0 || jellyfinCount > 0 {
		fmt.Printf(" (Plex: %d, Jellyfin: %d)", plexCount, jellyfinCount)
	}
	fmt.Println("\n")

	if len(streams) == 0 {
		fmt.Printf("%sNo active streams%s\n", color(ColorGreen), color(ColorReset))
		return nil
	}

	for _, stream := range streams {
		displayStream(stream, noColor)
	}

	displayStreamSummary(streams, noColor)

	return nil
}

func displayStream(stream StreamInfo, noColor bool) {
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		color(ColorBold), color(ColorBlue), color(ColorReset))

	// Server badge
	serverColor := ColorMagenta
	if stream.Server == "plex" {
		serverColor = ColorYellow
	}
	fmt.Printf("%s[%s]%s ", color(serverColor), strings.ToUpper(stream.Server), color(ColorReset))
	fmt.Printf("%sUser%s: %s%s%s\n", color(ColorBold), color(ColorReset),
		color(ColorYellow), stream.User, color(ColorReset))

	// Media info
	if stream.Type == "episode" && stream.Show != "" {
		fmt.Printf("%sShow%s: %s\n", color(ColorBold), color(ColorReset), stream.Show)
		if stream.Season != "" {
			fmt.Printf("%sSeason%s: %s\n", color(ColorBold), color(ColorReset), stream.Season)
		}
		if stream.Episode != "" {
			fmt.Printf("%sEpisode%s: %s - %s\n", color(ColorBold), color(ColorReset),
				stream.Episode, stream.Title)
		}
	} else {
		fmt.Printf("%sTitle%s: %s", color(ColorBold), color(ColorReset), stream.Title)
		if stream.Year != "" {
			fmt.Printf(" %s(%s)%s", color(ColorCyan), stream.Year, color(ColorReset))
		}
		fmt.Println()
	}

	// Client
	if stream.Device != "" {
		fmt.Printf("%sClient%s: %s (%s)\n", color(ColorBold), color(ColorReset),
			stream.Client, stream.Device)
	} else {
		fmt.Printf("%sClient%s: %s\n", color(ColorBold), color(ColorReset), stream.Client)
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
	fmt.Printf("%sStatus%s: %s%s%s\n", color(ColorBold), color(ColorReset),
		color(statusColor), statusText, color(ColorReset))

	// Bandwidth
	if stream.Bandwidth > 0 {
		fmt.Printf("%sBandwidth%s: %s%.2f Mbps%s\n", color(ColorBold), color(ColorReset),
			color(ColorMagenta), stream.Bandwidth, color(ColorReset))
	}

	// Quality
	if stream.Resolution != "" || stream.VideoCodec != "" {
		fmt.Printf("%sQuality%s: ", color(ColorBold), color(ColorReset))
		if stream.Resolution != "" {
			fmt.Printf("%s ", stream.Resolution)
		}
		if stream.VideoCodec != "" {
			fmt.Printf("(%s", stream.VideoCodec)
			if stream.AudioCodec != "" {
				fmt.Printf("/%s", stream.AudioCodec)
			}
			fmt.Printf(")")
		}
		fmt.Println()
	}
}

func displayStreamSummary(streams []StreamInfo, noColor bool) {
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	fmt.Println()
	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		color(ColorBold), color(ColorCyan), color(ColorReset))

	transcodeCount := 0
	totalBandwidth := 0.0

	for _, stream := range streams {
		if stream.Transcoding {
			transcodeCount++
		}
		totalBandwidth += stream.Bandwidth
	}

	fmt.Printf("%sTotal Streams%s: %d\n", color(ColorBold), color(ColorReset), len(streams))
	fmt.Printf("%sTranscoding%s: %d\n", color(ColorBold), color(ColorReset), transcodeCount)
	fmt.Printf("%sTotal Bandwidth%s: %s%.2f Mbps%s\n", color(ColorBold), color(ColorReset),
		color(ColorMagenta), totalBandwidth, color(ColorReset))
}

func clearScreen() {
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		cmd.Run()
	}
}
