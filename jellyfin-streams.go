package main

import (
	"encoding/json"
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

// JSON structures for Jellyfin API response
type JellyfinSession struct {
	PlayState          PlayState        `json:"PlayState"`
	NowPlayingItem     *NowPlayingItem  `json:"NowPlayingItem"`
	UserName           string           `json:"UserName"`
	Client             string           `json:"Client"`
	DeviceName         string           `json:"DeviceName"`
	Id                 string           `json:"Id"`
	TranscodingInfo    *TranscodingInfo `json:"TranscodingInfo"`
	PlayableMediaTypes []string         `json:"PlayableMediaTypes"`
}

type PlayState struct {
	PositionTicks int64  `json:"PositionTicks"`
	CanSeek       bool   `json:"CanSeek"`
	IsPaused      bool   `json:"IsPaused"`
	IsMuted       bool   `json:"IsMuted"`
	VolumeLevel   int    `json:"VolumeLevel"`
	PlayMethod    string `json:"PlayMethod"`
	RepeatMode    string `json:"RepeatMode"`
}

type NowPlayingItem struct {
	Name              string        `json:"Name"`
	SeriesName        string        `json:"SeriesName"`
	Type              string        `json:"Type"`
	ProductionYear    int           `json:"ProductionYear"`
	IndexNumber       int           `json:"IndexNumber"`
	ParentIndexNumber int           `json:"ParentIndexNumber"`
	RunTimeTicks      int64         `json:"RunTimeTicks"`
	Container         string        `json:"Container"`
	MediaStreams      []MediaStream `json:"MediaStreams"`
}

type MediaStream struct {
	Type         string `json:"Type"`
	Codec        string `json:"Codec"`
	DisplayTitle string `json:"DisplayTitle"`
	Width        int    `json:"Width"`
	Height       int    `json:"Height"`
}

type TranscodingInfo struct {
	IsVideoDirect    bool     `json:"IsVideoDirect"`
	IsAudioDirect    bool     `json:"IsAudioDirect"`
	Bitrate          int      `json:"Bitrate"`
	VideoCodec       string   `json:"VideoCodec"`
	AudioCodec       string   `json:"AudioCodec"`
	Container        string   `json:"Container"`
	Width            int      `json:"Width"`
	Height           int      `json:"Height"`
	TranscodeReasons []string `json:"TranscodeReasons"`
}

// Config holds the application configuration
type Config struct {
	JellyfinURL   string
	JellyfinToken string
	Timeout       time.Duration
	JSONOutput    bool
	WatchMode     bool
	WatchSeconds  int
	NoColor       bool
}

// StreamInfo holds formatted stream information for JSON output
type StreamInfo struct {
	User        string  `json:"user"`
	Type        string  `json:"type"` // "movie", "episode", "audio"
	Title       string  `json:"title"`
	Year        int     `json:"year,omitempty"`
	Show        string  `json:"show,omitempty"`
	Season      int     `json:"season,omitempty"`
	Episode     int     `json:"episode,omitempty"`
	Client      string  `json:"client"`
	Device      string  `json:"device"`
	Status      string  `json:"status"` // "Direct" or "Transcoding"
	Transcoding bool    `json:"transcoding"`
	Bandwidth   float64 `json:"bandwidth_mbps"`
	VideoCodec  string  `json:"video_codec,omitempty"`
	AudioCodec  string  `json:"audio_codec,omitempty"`
	Resolution  string  `json:"resolution,omitempty"`
	PlayMethod  string  `json:"play_method,omitempty"`
	IsPaused    bool    `json:"is_paused"`
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
		jellyfinURL   = flag.String("url", "", "Jellyfin server URL (default: http://localhost:8096)")
		jellyfinToken = flag.String("token", "", "Jellyfin API token")
		timeout       = flag.Duration("timeout", 10*time.Second, "Connection timeout")
		noColor       = flag.Bool("no-color", false, "Disable colored output")
		jsonOutput    = flag.Bool("json", false, "Output in JSON format")
		watchMode     = flag.Bool("watch", false, "Continuously monitor streams")
		watchSeconds  = flag.Int("interval", 5, "Watch mode refresh interval in seconds")
	)
	flag.Parse()

	// Load configuration
	config := loadConfig(*jellyfinURL, *jellyfinToken, *timeout, *noColor, *jsonOutput, *watchMode, *watchSeconds)

	// Validate configuration
	if config.JellyfinToken == "" {
		fmt.Fprintf(os.Stderr, "ERROR: JELLYFIN_TOKEN is not set\n")
		fmt.Fprintf(os.Stderr, "Please set JELLYFIN_TOKEN environment variable, add to /opt/apps/compose/.env, or use -token flag\n")
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
		JellyfinURL:   "http://localhost:8096",
		JellyfinToken: "",
		Timeout:       timeout,
		JSONOutput:    jsonOutput,
		WatchMode:     watchMode,
		WatchSeconds:  watchSeconds,
		NoColor:       noColor || jsonOutput, // Always disable colors for JSON
	}

	// Try to load from .env file first
	envPath := "/opt/apps/compose/.env"
	if _, err := os.Stat(envPath); err == nil {
		loadEnvFile(envPath, &config)
	}

	// Environment variables override .env file
	if envURL := os.Getenv("JELLYFIN_URL"); envURL != "" {
		config.JellyfinURL = envURL
	}
	if envToken := os.Getenv("JELLYFIN_TOKEN"); envToken != "" {
		config.JellyfinToken = envToken
	}

	// Command line flags override everything
	if urlFlag != "" {
		config.JellyfinURL = urlFlag
	}
	if tokenFlag != "" {
		config.JellyfinToken = tokenFlag
	}

	// Ensure URL doesn't have trailing slash
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
		case "JELLYFIN_URL":
			config.JellyfinURL = value
		case "JELLYFIN_TOKEN":
			config.JellyfinToken = value
		}
	}
}

func fetchSessions(config Config) ([]JellyfinSession, error) {
	url := fmt.Sprintf("%s/Sessions?api_key=%s", config.JellyfinURL, config.JellyfinToken)

	client := &http.Client{
		Timeout: config.Timeout,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Jellyfin server at %s: %w", config.JellyfinURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jellyfin server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var sessions []JellyfinSession
	if err := json.Unmarshal(body, &sessions); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return sessions, nil
}

func displaySessions(config Config) error {
	sessions, err := fetchSessions(config)
	if err != nil {
		return err
	}

	// Filter sessions with active playback
	activeSessions := make([]JellyfinSession, 0)
	for _, session := range sessions {
		if session.NowPlayingItem != nil {
			activeSessions = append(activeSessions, session)
		}
	}

	// JSON output mode
	if config.JSONOutput {
		return displayJSON(activeSessions)
	}

	// Terminal output mode
	return displayTerminal(activeSessions, config.NoColor)
}

func displayJSON(sessions []JellyfinSession) error {
	summary := Summary{
		TotalStreams: len(sessions),
		Timestamp:    time.Now(),
		Streams:      make([]StreamInfo, 0),
	}

	totalBandwidth := 0
	transcodeCount := 0

	for _, session := range sessions {
		stream := sessionToStreamInfo(session)
		summary.Streams = append(summary.Streams, stream)
		totalBandwidth += int(stream.Bandwidth * 1000)
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

func displayTerminal(sessions []JellyfinSession, noColor bool) error {
	// Helper function to handle color output
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	// Print header
	fmt.Printf("%s%s=== Jellyfin Active Streams ===%s\n",
		color(ColorBold), color(ColorCyan), color(ColorReset))
	fmt.Printf("Total Sessions: %s%d%s\n\n",
		color(ColorBold), len(sessions), color(ColorReset))

	if len(sessions) == 0 {
		fmt.Printf("%sNo active streams%s\n", color(ColorGreen), color(ColorReset))
		return nil
	}

	// Display sessions
	for _, session := range sessions {
		displaySession(session, noColor)
	}

	// Display summary
	displaySummary(sessions, noColor)

	return nil
}

func displaySession(session JellyfinSession, noColor bool) {
	color := func(code string) string {
		if noColor {
			return ""
		}
		return code
	}

	fmt.Printf("%s%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n",
		color(ColorBold), color(ColorBlue), color(ColorReset))

	// User
	user := session.UserName
	if user == "" {
		user = "Unknown"
	}
	fmt.Printf("%sUser%s: %s%s%s\n",
		color(ColorBold), color(ColorReset), color(ColorYellow), user, color(ColorReset))

	// Media information
	if session.NowPlayingItem != nil {
		item := session.NowPlayingItem

		if item.Type == "Episode" && item.SeriesName != "" {
			fmt.Printf("%sShow%s: %s\n", color(ColorBold), color(ColorReset), item.SeriesName)
			if item.ParentIndexNumber > 0 {
				fmt.Printf("%sSeason%s: %d\n", color(ColorBold), color(ColorReset), item.ParentIndexNumber)
			}
			if item.IndexNumber > 0 {
				fmt.Printf("%sEpisode%s: %d - %s\n",
					color(ColorBold), color(ColorReset), item.IndexNumber, item.Name)
			} else {
				fmt.Printf("%sEpisode%s: %s\n", color(ColorBold), color(ColorReset), item.Name)
			}
		} else {
			title := item.Name
			if title == "" {
				title = "Unknown"
			}
			fmt.Printf("%sTitle%s: %s", color(ColorBold), color(ColorReset), title)
			if item.ProductionYear > 0 {
				fmt.Printf(" %s(%d)%s", color(ColorCyan), item.ProductionYear, color(ColorReset))
			}
			fmt.Println()
		}
	}

	// Client/Device
	client := session.Client
	if client == "" {
		client = "Unknown"
	}
	device := session.DeviceName
	if device != "" {
		fmt.Printf("%sClient%s: %s (%s)\n", color(ColorBold), color(ColorReset), client, device)
	} else {
		fmt.Printf("%sClient%s: %s\n", color(ColorBold), color(ColorReset), client)
	}

	// Transcoding status
	isTranscoding := false
	statusColor := ColorGreen
	statusText := "Direct Play"

	if session.TranscodingInfo != nil {
		if !session.TranscodingInfo.IsVideoDirect || !session.TranscodingInfo.IsAudioDirect {
			isTranscoding = true
			statusColor = ColorRed
			statusText = "Transcoding"
		}
	}

	// Check play method as fallback
	if session.PlayState.PlayMethod == "Transcode" {
		isTranscoding = true
		statusColor = ColorRed
		statusText = "Transcoding"
	}

	// Check for paused state
	if session.PlayState.IsPaused {
		statusText += " (Paused)"
	}

	fmt.Printf("%sStatus%s: %s%s%s\n",
		color(ColorBold), color(ColorReset), color(statusColor), statusText, color(ColorReset))

	// Transcoding details
	if isTranscoding && session.TranscodingInfo != nil {
		ti := session.TranscodingInfo
		fmt.Printf("%sDecisions%s: ", color(ColorBold), color(ColorReset))

		videoStatus := "copy"
		if !ti.IsVideoDirect {
			videoStatus = "transcode"
		}
		audioStatus := "copy"
		if !ti.IsAudioDirect {
			audioStatus = "transcode"
		}

		fmt.Printf("Video: %s, Audio: %s\n", videoStatus, audioStatus)

		if len(ti.TranscodeReasons) > 0 {
			fmt.Printf("%sReasons%s: %s\n",
				color(ColorBold), color(ColorReset), strings.Join(ti.TranscodeReasons, ", "))
		}
	}

	// Bandwidth
	if session.TranscodingInfo != nil && session.TranscodingInfo.Bitrate > 0 {
		bandwidthMbps := float64(session.TranscodingInfo.Bitrate) / 1000000.0
		fmt.Printf("%sBandwidth%s: %s%.2f Mbps%s\n",
			color(ColorBold), color(ColorReset), color(ColorMagenta), bandwidthMbps, color(ColorReset))
	}

	// Quality information
	if session.NowPlayingItem != nil {
		var videoCodec, audioCodec, resolution string

		if session.TranscodingInfo != nil {
			videoCodec = session.TranscodingInfo.VideoCodec
			audioCodec = session.TranscodingInfo.AudioCodec
			if session.TranscodingInfo.Height > 0 {
				resolution = getResolutionName(session.TranscodingInfo.Height)
			}
		} else {
			// Get from media streams
			for _, stream := range session.NowPlayingItem.MediaStreams {
				if stream.Type == "Video" && videoCodec == "" {
					videoCodec = stream.Codec
					if stream.Height > 0 {
						resolution = getResolutionName(stream.Height)
					}
				}
				if stream.Type == "Audio" && audioCodec == "" {
					audioCodec = stream.Codec
				}
			}
		}

		if resolution != "" || videoCodec != "" {
			fmt.Printf("%sQuality%s: ", color(ColorBold), color(ColorReset))
			if resolution != "" {
				fmt.Printf("%s ", resolution)
			}
			if videoCodec != "" {
				fmt.Printf("(%s", videoCodec)
				if audioCodec != "" {
					fmt.Printf("/%s", audioCodec)
				}
				fmt.Printf(")")
			}
			fmt.Println()
		}
	}
}

func displaySummary(sessions []JellyfinSession, noColor bool) {
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

	for _, session := range sessions {
		if session.TranscodingInfo != nil {
			if !session.TranscodingInfo.IsVideoDirect || !session.TranscodingInfo.IsAudioDirect {
				transcodeCount++
			}
			totalBandwidth += session.TranscodingInfo.Bitrate
		}
	}

	totalBandwidthMbps := float64(totalBandwidth) / 1000000.0

	fmt.Printf("%sTotal Streams%s: %d\n", color(ColorBold), color(ColorReset), len(sessions))
	fmt.Printf("%sTranscoding%s: %d\n", color(ColorBold), color(ColorReset), transcodeCount)
	fmt.Printf("%sTotal Bandwidth%s: %s%.2f Mbps%s\n",
		color(ColorBold), color(ColorReset), color(ColorMagenta), totalBandwidthMbps, color(ColorReset))
}

func sessionToStreamInfo(session JellyfinSession) StreamInfo {
	stream := StreamInfo{
		User:       session.UserName,
		Client:     session.Client,
		Device:     session.DeviceName,
		IsPaused:   session.PlayState.IsPaused,
		PlayMethod: session.PlayState.PlayMethod,
	}

	if session.NowPlayingItem != nil {
		item := session.NowPlayingItem
		stream.Title = item.Name
		stream.Type = strings.ToLower(item.Type)
		stream.Year = item.ProductionYear
		stream.Show = item.SeriesName
		stream.Season = item.ParentIndexNumber
		stream.Episode = item.IndexNumber
	}

	// Transcoding info
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
