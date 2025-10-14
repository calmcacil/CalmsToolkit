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

// XML structures for Plex API response
type MediaContainer struct {
	XMLName xml.Name `xml:"MediaContainer"`
	Size    int      `xml:"size,attr"`
	Videos  []Video  `xml:"Video"`
	Tracks  []Track  `xml:"Track"`
}

type Video struct {
	Title            string            `xml:"title,attr"`
	Year             string            `xml:"year,attr"`
	Type             string            `xml:"type,attr"`
	GrandparentTitle string            `xml:"grandparentTitle,attr"`
	ParentIndex      string            `xml:"parentIndex,attr"`
	Index            string            `xml:"index,attr"`
	User             User              `xml:"User"`
	Player           Player            `xml:"Player"`
	Session          Session           `xml:"Session"`
	Media            []Media           `xml:"Media"`
	TranscodeSession *TranscodeSession `xml:"TranscodeSession"`
}

type Track struct {
	Title            string  `xml:"title,attr"`
	GrandparentTitle string  `xml:"grandparentTitle,attr"`
	User             User    `xml:"User"`
	Player           Player  `xml:"Player"`
	Session          Session `xml:"Session"`
	Media            []Media `xml:"Media"`
}

type User struct {
	Title string `xml:"title,attr"`
}

type Player struct {
	Title  string `xml:"title,attr"`
	Device string `xml:"device,attr"`
}

type Session struct {
	Bandwidth int `xml:"bandwidth,attr"`
}

type Media struct {
	VideoResolution string `xml:"videoResolution,attr"`
	VideoCodec      string `xml:"videoCodec,attr"`
	AudioCodec      string `xml:"audioCodec,attr"`
	Parts           []Part `xml:"Part"`
}

type Part struct {
	VideoDecision string `xml:"videoDecision,attr"`
	AudioDecision string `xml:"audioDecision,attr"`
}

type TranscodeSession struct {
	VideoDecision string `xml:"videoDecision,attr"`
	AudioDecision string `xml:"audioDecision,attr"`
}

// Config holds the application configuration
type Config struct {
	PlexURL      string
	PlexToken    string
	Timeout      time.Duration
	JSONOutput   bool
	WatchMode    bool
	WatchSeconds int
	NoColor      bool
}

// StreamInfo holds formatted stream information for JSON output
type StreamInfo struct {
	User          string  `json:"user"`
	Type          string  `json:"type"` // "movie", "episode", "track"
	Title         string  `json:"title"`
	Year          string  `json:"year,omitempty"`
	Show          string  `json:"show,omitempty"`
	Season        string  `json:"season,omitempty"`
	Episode       string  `json:"episode,omitempty"`
	Artist        string  `json:"artist,omitempty"`
	Player        string  `json:"player"`
	Status        string  `json:"status"` // "Direct" or "Transcoding"
	Transcoding   bool    `json:"transcoding"`
	Bandwidth     float64 `json:"bandwidth_mbps"`
	VideoCodec    string  `json:"video_codec,omitempty"`
	AudioCodec    string  `json:"audio_codec,omitempty"`
	Resolution    string  `json:"resolution,omitempty"`
	VideoDecision string  `json:"video_decision,omitempty"`
	AudioDecision string  `json:"audio_decision,omitempty"`
}

// Summary holds session summary information
type Summary struct {
	TotalStreams     int          `json:"total_streams"`
	TranscodingCount int          `json:"transcoding_count"`
	TotalBandwidth   float64      `json:"total_bandwidth_mbps"`
	Timestamp        time.Time    `json:"timestamp"`
	Streams          []StreamInfo `json:"streams"`
}

func main() {
	// Command line flags
	var (
		plexURL      = flag.String("url", "", "Plex server URL (default: http://localhost:32400)")
		plexToken    = flag.String("token", "", "Plex authentication token")
		timeout      = flag.Duration("timeout", 10*time.Second, "Connection timeout")
		noColor      = flag.Bool("no-color", false, "Disable colored output")
		jsonOutput   = flag.Bool("json", false, "Output in JSON format")
		watchMode    = flag.Bool("watch", false, "Continuously monitor streams")
		watchSeconds = flag.Int("interval", 5, "Watch mode refresh interval in seconds")
	)
	flag.Parse()

	// Load configuration
	config := loadConfig(*plexURL, *plexToken, *timeout, *noColor, *jsonOutput, *watchMode, *watchSeconds)

	// Validate configuration
	if config.PlexToken == "" {
		fmt.Fprintf(os.Stderr, "ERROR: PLEX_TOKEN is not set\n")
		fmt.Fprintf(os.Stderr, "Please set PLEX_TOKEN environment variable, add to /opt/apps/compose/.env, or use -token flag\n")
		os.Exit(1)
	}

	// Watch mode: continuously monitor
	if config.WatchMode {
		for {
			clearScreen()
			if err := displaySessions(config); err != nil {
				fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			}
			time.Sleep(time.Duration(config.WatchSeconds) * time.Second)
		}
	}

	// Single execution
	if err := displaySessions(config); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
}

func loadConfig(urlFlag, tokenFlag string, timeout time.Duration, noColor, jsonOutput, watchMode bool, watchSeconds int) Config {
	config := Config{
		PlexURL:      "http://localhost:32400",
		PlexToken:    "",
		Timeout:      timeout,
		JSONOutput:   jsonOutput,
		WatchMode:    watchMode,
		WatchSeconds: watchSeconds,
		NoColor:      noColor || jsonOutput, // Always disable colors for JSON
	}

	// Try to load from .env file first
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

	// Command line flags override everything
	if urlFlag != "" {
		config.PlexURL = urlFlag
	}
	if tokenFlag != "" {
		config.PlexToken = tokenFlag
	}

	// Ensure URL doesn't have trailing slash
	config.PlexURL = strings.TrimSuffix(config.PlexURL, "/")

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
		}
	}
}

func fetchSessions(config Config) (*MediaContainer, error) {
	url := fmt.Sprintf("%s/status/sessions?X-Plex-Token=%s", config.PlexURL, config.PlexToken)

	client := &http.Client{
		Timeout: config.Timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Plex server at %s: %w", config.PlexURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Plex server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var container MediaContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("failed to parse XML response: %w", err)
	}

	return &container, nil
}

func displaySessions(config Config) error {
	container, err := fetchSessions(config)
	if err != nil {
		return err
	}

	// JSON output mode
	if config.JSONOutput {
		return displayJSON(container)
	}

	// Terminal output mode
	return displayTerminal(container, config.NoColor)
}

func displayJSON(container *MediaContainer) error {
	summary := Summary{
		TotalStreams: container.Size,
		Timestamp:    time.Now(),
		Streams:      make([]StreamInfo, 0),
	}

	totalBandwidth := 0
	transcodeCount := 0

	// Process video sessions
	for _, video := range container.Videos {
		stream := videoToStreamInfo(video)
		summary.Streams = append(summary.Streams, stream)
		totalBandwidth += video.Session.Bandwidth
		if stream.Transcoding {
			transcodeCount++
		}
	}

	// Process track sessions
	for _, track := range container.Tracks {
		stream := trackToStreamInfo(track)
		summary.Streams = append(summary.Streams, stream)
		totalBandwidth += track.Session.Bandwidth
		if stream.Transcoding {
			transcodeCount++
		}
	}

	summary.TranscodingCount = transcodeCount
	summary.TotalBandwidth = float64(totalBandwidth) / 1000.0

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

func displayTerminal(container *MediaContainer, noColor bool) error {
	// Helper function to handle color output
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	// Print header
	fmt.Printf("%s%s=== Plex Active Streams ===%s\n",
		color(ColorBold), color(ColorCyan), color(ColorReset))
	fmt.Printf("Total Sessions: %s%d%s\n\n",
		color(ColorBold), container.Size, color(ColorReset))

	if container.Size == 0 {
		fmt.Printf("%sNo active streams%s\n", color(ColorGreen), color(ColorReset))
		return nil
	}

	// Display video sessions
	for _, video := range container.Videos {
		displayVideoSession(video, noColor)
	}

	// Display audio/track sessions
	for _, track := range container.Tracks {
		displayTrackSession(track, noColor)
	}

	// Display summary
	displaySummary(container, noColor)

	return nil
}

func displayVideoSession(video Video, noColor bool) {
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		color(ColorBold), color(ColorBlue), color(ColorReset))

	// User
	user := video.User.Title
	if user == "" {
		user = "Unknown"
	}
	fmt.Printf("%sUser%s: %s%s%s\n",
		color(ColorBold), color(ColorReset), color(ColorYellow), user, color(ColorReset))

	// Title/Show information
	if video.Type == "episode" && video.GrandparentTitle != "" {
		fmt.Printf("%sShow%s: %s\n", color(ColorBold), color(ColorReset), video.GrandparentTitle)
		if video.ParentIndex != "" {
			fmt.Printf("%sSeason%s: %s\n", color(ColorBold), color(ColorReset), video.ParentIndex)
		}
		if video.Index != "" {
			fmt.Printf("%sEpisode%s: %s - %s\n",
				color(ColorBold), color(ColorReset), video.Index, video.Title)
		} else {
			fmt.Printf("%sEpisode%s: %s\n", color(ColorBold), color(ColorReset), video.Title)
		}
	} else {
		title := video.Title
		if title == "" {
			title = "Unknown"
		}
		fmt.Printf("%sTitle%s: %s", color(ColorBold), color(ColorReset), title)
		if video.Year != "" {
			fmt.Printf(" %s(%s)%s", color(ColorCyan), video.Year, color(ColorReset))
		}
		fmt.Println()
	}

	// Player
	player := video.Player.Title
	if player == "" {
		player = video.Player.Device
	}
	if player == "" {
		player = "Unknown"
	}
	fmt.Printf("%sPlayer%s: %s\n", color(ColorBold), color(ColorReset), player)

	// Determine transcoding status
	videoDecision, audioDecision := getDecisions(video.Media, video.TranscodeSession)
	isTranscoding := video.TranscodeSession != nil ||
		videoDecision == "transcode" ||
		audioDecision == "transcode"

	statusColor := ColorGreen
	statusText := "Direct"
	if isTranscoding {
		statusColor = ColorRed
		statusText = "Transcoding"
	}

	fmt.Printf("%sStatus%s: %s%s%s\n",
		color(ColorBold), color(ColorReset), color(statusColor), statusText, color(ColorReset))

	if isTranscoding {
		if videoDecision == "" {
			videoDecision = "copy"
		}
		if audioDecision == "" {
			audioDecision = "copy"
		}
		fmt.Printf("%sDecisions%s: Video: %s, Audio: %s\n",
			color(ColorBold), color(ColorReset), videoDecision, audioDecision)
	}

	// Bandwidth
	bandwidthMbps := float64(video.Session.Bandwidth) / 1000.0
	if video.Session.Bandwidth == 0 {
		fmt.Printf("%sBandwidth%s: %sN/A%s\n",
			color(ColorBold), color(ColorReset), color(ColorMagenta), color(ColorReset))
	} else {
		fmt.Printf("%sBandwidth%s: %s%.2f Mbps%s\n",
			color(ColorBold), color(ColorReset), color(ColorMagenta), bandwidthMbps, color(ColorReset))
	}

	// Quality information
	if len(video.Media) > 0 {
		media := video.Media[0]
		if media.VideoResolution != "" || media.VideoCodec != "" {
			fmt.Printf("%sQuality%s: ", color(ColorBold), color(ColorReset))
			if media.VideoResolution != "" {
				fmt.Printf("%s ", media.VideoResolution)
			}
			if media.VideoCodec != "" {
				fmt.Printf("(%s", media.VideoCodec)
				if media.AudioCodec != "" {
					fmt.Printf("/%s", media.AudioCodec)
				}
				fmt.Printf(")")
			}
			fmt.Println()
		}
	}
}

func displayTrackSession(track Track, noColor bool) {
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		color(ColorBold), color(ColorBlue), color(ColorReset))

	// User
	user := track.User.Title
	if user == "" {
		user = "Unknown"
	}
	fmt.Printf("%sUser%s: %s%s%s\n",
		color(ColorBold), color(ColorReset), color(ColorYellow), user, color(ColorReset))

	// Track info
	artist := track.GrandparentTitle
	if artist == "" {
		artist = "Unknown"
	}
	title := track.Title
	if title == "" {
		title = "Unknown"
	}
	fmt.Printf("%sTrack%s: %s - %s\n",
		color(ColorBold), color(ColorReset), artist, title)

	// Status
	audioDecision := ""
	if len(track.Media) > 0 && len(track.Media[0].Parts) > 0 {
		audioDecision = track.Media[0].Parts[0].AudioDecision
	}

	statusColor := ColorGreen
	statusText := "Direct"
	if audioDecision == "transcode" {
		statusColor = ColorRed
		statusText = "Transcoding"
	}

	fmt.Printf("%sStatus%s: %s%s%s\n",
		color(ColorBold), color(ColorReset), color(statusColor), statusText, color(ColorReset))

	// Bandwidth
	bandwidthMbps := float64(track.Session.Bandwidth) / 1000.0
	if track.Session.Bandwidth == 0 {
		fmt.Printf("%sBandwidth%s: %sN/A%s\n",
			color(ColorBold), color(ColorReset), color(ColorMagenta), color(ColorReset))
	} else {
		fmt.Printf("%sBandwidth%s: %s%.2f Mbps%s\n",
			color(ColorBold), color(ColorReset), color(ColorMagenta), bandwidthMbps, color(ColorReset))
	}
}

func displaySummary(container *MediaContainer, noColor bool) {
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	fmt.Println()
	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		color(ColorBold), color(ColorCyan), color(ColorReset))

	// Count transcoding sessions
	transcodeCount := 0
	totalBandwidth := 0

	for _, video := range container.Videos {
		if video.TranscodeSession != nil {
			transcodeCount++
		}
		totalBandwidth += video.Session.Bandwidth
	}

	for _, track := range container.Tracks {
		totalBandwidth += track.Session.Bandwidth
	}

	totalBandwidthMbps := float64(totalBandwidth) / 1000.0

	fmt.Printf("%sTotal Streams%s: %d\n", color(ColorBold), color(ColorReset), container.Size)
	fmt.Printf("%sTranscoding%s: %d\n", color(ColorBold), color(ColorReset), transcodeCount)
	fmt.Printf("%sTotal Bandwidth%s: %s%.2f Mbps%s\n",
		color(ColorBold), color(ColorReset), color(ColorMagenta), totalBandwidthMbps, color(ColorReset))
}

func videoToStreamInfo(video Video) StreamInfo {
	stream := StreamInfo{
		User:      video.User.Title,
		Title:     video.Title,
		Year:      video.Year,
		Player:    video.Player.Title,
		Bandwidth: float64(video.Session.Bandwidth) / 1000.0,
		Type:      video.Type,
		Show:      video.GrandparentTitle,
		Season:    video.ParentIndex,
		Episode:   video.Index,
	}

	if stream.Player == "" {
		stream.Player = video.Player.Device
	}

	if len(video.Media) > 0 {
		stream.VideoCodec = video.Media[0].VideoCodec
		stream.AudioCodec = video.Media[0].AudioCodec
		stream.Resolution = video.Media[0].VideoResolution
	}

	videoDecision, audioDecision := getDecisions(video.Media, video.TranscodeSession)
	stream.VideoDecision = videoDecision
	stream.AudioDecision = audioDecision

	stream.Transcoding = video.TranscodeSession != nil ||
		videoDecision == "transcode" ||
		audioDecision == "transcode"

	if stream.Transcoding {
		stream.Status = "Transcoding"
	} else {
		stream.Status = "Direct"
	}

	return stream
}

func trackToStreamInfo(track Track) StreamInfo {
	stream := StreamInfo{
		User:      track.User.Title,
		Type:      "track",
		Title:     track.Title,
		Artist:    track.GrandparentTitle,
		Player:    track.Player.Title,
		Bandwidth: float64(track.Session.Bandwidth) / 1000.0,
	}

	if stream.Player == "" {
		stream.Player = track.Player.Device
	}

	if len(track.Media) > 0 {
		stream.AudioCodec = track.Media[0].AudioCodec
		if len(track.Media[0].Parts) > 0 {
			stream.AudioDecision = track.Media[0].Parts[0].AudioDecision
		}
	}

	stream.Transcoding = stream.AudioDecision == "transcode"

	if stream.Transcoding {
		stream.Status = "Transcoding"
	} else {
		stream.Status = "Direct"
	}

	return stream
}

func getDecisions(media []Media, transcode *TranscodeSession) (videoDecision, audioDecision string) {
	// First try to get from media parts
	if len(media) > 0 && len(media[0].Parts) > 0 {
		videoDecision = media[0].Parts[0].VideoDecision
		audioDecision = media[0].Parts[0].AudioDecision
	}

	// If not found and transcode session exists, get from there
	if transcode != nil {
		if videoDecision == "" {
			videoDecision = transcode.VideoDecision
		}
		if audioDecision == "" {
			audioDecision = transcode.AudioDecision
		}
	}

	return
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
