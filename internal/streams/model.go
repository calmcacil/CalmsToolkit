package streams

import (
	"encoding/xml"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/core"
)

const (
	// ServerPlex indicates the Plex server type.
	ServerPlex = "plex"
	// ServerJellyfin indicates the Jellyfin server type.
	ServerJellyfin = "jellyfin"
	// ServerBoth indicates both Plex and Jellyfin server types.
	ServerBoth = "both"

	streamUserWidth   = 12
	streamShowWidth   = 9
	streamTitleWidth  = 11
	streamClientWidth = 12
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
	core.CommonConfig
	ServerType      string
	PlexURL         string
	PlexToken       string
	JellyfinURL     string
	JellyfinToken   string
	HistoryDuration time.Duration
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
