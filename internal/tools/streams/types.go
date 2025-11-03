package streams

import (
	"encoding/xml"
	"time"
)

// StreamInfo represents a media stream from either Plex or Jellyfin
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

// NewSessionHistory creates a new session history
func NewSessionHistory() *SessionHistory {
	return &SessionHistory{
		Records: make(map[string]*SessionRecord),
	}
}

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
