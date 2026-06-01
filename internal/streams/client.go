package streams

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/httputil"
)

func fetchPlexStreams(ctx context.Context, client *httputil.Client, cfg ToolConfig) ([]StreamInfo, error) {
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

func fetchJellyfinStreams(ctx context.Context, client *httputil.Client, cfg ToolConfig) ([]StreamInfo, error) {
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
