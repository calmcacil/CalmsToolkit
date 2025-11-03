package streams

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/calmcacil/CalmsToolkit/internal/config"
)

// API handles fetching streams from Plex and Jellyfin
type API struct {
	client *http.Client
	config *config.Config
}

// NewAPI creates a new API client
func NewAPI(cfg *config.Config) *API {
	return &API{
		client: &http.Client{Timeout: cfg.Timeout},
		config: cfg,
	}
}

// FetchStreams fetches all streams based on configuration
func (a *API) FetchStreams() ([]StreamInfo, error) {
	var allStreams []StreamInfo

	// Fetch Plex sessions
	if a.config.ServerType == "plex" || a.config.ServerType == "both" {
		if streams, err := a.fetchPlexStreams(); err == nil {
			allStreams = append(allStreams, streams...)
		}
	}

	// Fetch Jellyfin sessions
	if a.config.ServerType == "jellyfin" || a.config.ServerType == "both" {
		if streams, err := a.fetchJellyfinStreams(); err == nil {
			allStreams = append(allStreams, streams...)
		}
	}

	return allStreams, nil
}

// fetchPlexStreams fetches streams from Plex
func (a *API) fetchPlexStreams() ([]StreamInfo, error) {
	url := fmt.Sprintf("%s/status/sessions?X-Plex-Token=%s", a.config.PlexURL, a.config.PlexToken)

	resp, err := a.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("plex request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read plex response: %w", err)
	}

	var container PlexMediaContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("failed to parse plex XML: %w", err)
	}

	streams := make([]StreamInfo, 0)
	for _, video := range container.Videos {
		streams = append(streams, a.plexVideoToStream(video))
	}
	for _, track := range container.Tracks {
		streams = append(streams, a.plexTrackToStream(track))
	}

	return streams, nil
}

// fetchJellyfinStreams fetches streams from Jellyfin
func (a *API) fetchJellyfinStreams() ([]StreamInfo, error) {
	url := fmt.Sprintf("%s/Sessions?api_key=%s", a.config.JellyfinURL, a.config.JellyfinToken)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create jellyfin request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jellyfin request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jellyfin returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read jellyfin response: %w", err)
	}

	var sessions []JellyfinSession
	if err := json.Unmarshal(body, &sessions); err != nil {
		return nil, fmt.Errorf("failed to parse jellyfin JSON: %w", err)
	}

	streams := make([]StreamInfo, 0)
	for _, session := range sessions {
		if session.NowPlayingItem != nil {
			streams = append(streams, a.jellyfinSessionToStream(session))
		}
	}

	return streams, nil
}

// plexVideoToStream converts Plex video to StreamInfo
func (a *API) plexVideoToStream(video PlexVideo) StreamInfo {
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

	videoDecision, audioDecision := a.getPlexDecisions(video.Media, video.TranscodeSession)
	stream.Transcoding = video.TranscodeSession != nil ||
		videoDecision == "transcode" || audioDecision == "transcode"

	if stream.Transcoding {
		stream.Status = "Transcoding"
	} else {
		stream.Status = "Direct"
	}

	return stream
}

// plexTrackToStream converts Plex track to StreamInfo
func (a *API) plexTrackToStream(track PlexTrack) StreamInfo {
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

// jellyfinSessionToStream converts Jellyfin session to StreamInfo
func (a *API) jellyfinSessionToStream(session JellyfinSession) StreamInfo {
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
			stream.Year = strconv.Itoa(item.ProductionYear)
		}
		stream.Show = item.SeriesName
		if item.ParentIndexNumber > 0 {
			stream.Season = strconv.Itoa(item.ParentIndexNumber)
		}
		if item.IndexNumber > 0 {
			stream.Episode = strconv.Itoa(item.IndexNumber)
		}
	}

	if session.TranscodingInfo != nil {
		ti := session.TranscodingInfo
		stream.Transcoding = !ti.IsVideoDirect || !ti.IsAudioDirect
		stream.Bandwidth = float64(ti.Bitrate) / 1000000.0
		stream.VideoCodec = ti.VideoCodec
		stream.AudioCodec = ti.AudioCodec
		if ti.Height > 0 {
			stream.Resolution = a.getResolutionName(ti.Height)
		}
	}

	if stream.Transcoding {
		stream.Status = "Transcoding"
	} else {
		stream.Status = "Direct"
	}

	return stream
}

// getPlexDecisions extracts transcode decisions from Plex media
func (a *API) getPlexDecisions(media []PlexMedia, transcode *PlexTranscodeSession) (string, string) {
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

// getResolutionName converts height to resolution name
func (a *API) getResolutionName(height int) string {
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

// generateSessionID creates a unique identifier for a stream to track it across refreshes
func generateSessionID(stream StreamInfo) string {
	return fmt.Sprintf("%s:%s:%s:%s", stream.Server, stream.User, stream.Title, stream.Client)
}
