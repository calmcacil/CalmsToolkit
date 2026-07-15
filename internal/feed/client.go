package feed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/config"
	"github.com/calmcacil/CalmsToolkit/internal/httputil"
)

func fetchAllHistory(ctx context.Context, client *httputil.Client, cfg ToolConfig, since time.Time) ([]HistoryEvent, error) {
	events, _, err := fetchAllHistoryDetailed(ctx, client, cfg, since)
	return events, err
}

func fetchAllHistoryDetailed(ctx context.Context, client *httputil.Client, cfg ToolConfig, since time.Time) ([]HistoryEvent, []string, error) {
	var wg sync.WaitGroup
	eventsChan := make(chan []HistoryEvent, len(cfg.SonarrInstances)+len(cfg.RadarrInstances))
	errorsChan := make(chan error, len(cfg.SonarrInstances)+len(cfg.RadarrInstances))

	for _, inst := range cfg.SonarrInstances {
		wg.Add(1)
		go func(inst config.ArrInstance) {
			defer wg.Done()
			events, err := fetchSonarrHistory(ctx, client, inst, since, cfg.ShowSubtitles)
			if err != nil {
				errorsChan <- fmt.Errorf("Sonarr %s: %v", inst.Name, err)
				return
			}
			eventsChan <- events
		}(inst)
	}

	for _, inst := range cfg.RadarrInstances {
		wg.Add(1)
		go func(inst config.ArrInstance) {
			defer wg.Done()
			events, err := fetchRadarrHistory(ctx, client, inst, since, cfg.ShowSubtitles)
			if err != nil {
				errorsChan <- fmt.Errorf("Radarr %s: %v", inst.Name, err)
				return
			}
			eventsChan <- events
		}(inst)
	}

	wg.Wait()
	close(eventsChan)
	close(errorsChan)

	var allEvents []HistoryEvent
	successes := 0
	for events := range eventsChan {
		successes++
		allEvents = append(allEvents, events...)
	}

	var errors []error
	for err := range errorsChan {
		errors = append(errors, err)
	}

	warnings := make([]string, len(errors))
	for i, err := range errors {
		warnings[i] = err.Error()
	}
	if successes == 0 && len(errors) > 0 {
		return nil, warnings, fmt.Errorf("all instances failed: %v", errors)
	}

	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].When.After(allEvents[j].When)
	})

	return allEvents, warnings, nil
}

func fetchSonarrHistory(ctx context.Context, client *httputil.Client, inst config.ArrInstance, since time.Time, showSubtitles bool) ([]HistoryEvent, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	endpoint := fmt.Sprintf("%s/api/v3/history/since?date=%s&includeEpisode=true&includeSeries=true", inst.URL, sinceStr)

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	body, status, err := client.DoRequest(ctx, "GET", endpoint, headers, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", status, string(body))
	}

	var wrapper struct {
		Records []SonarrHistory `json:"records"`
	}
	var history []SonarrHistory
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Records != nil {
		history = wrapper.Records
	} else if err := json.Unmarshal(body, &history); err != nil {
		return nil, err
	}

	events := make([]HistoryEvent, 0, len(history))
	for _, h := range history {
		events = append(events, sonarrToHistoryEvent(h))
	}

	if showSubtitles {
		enrichSonarrSubtitles(ctx, client, inst, events)
	}

	return events, nil
}

func enrichSonarrSubtitles(ctx context.Context, client *httputil.Client, inst config.ArrInstance, events []HistoryEvent) {
	var ids []int
	seen := make(map[int]bool)
	for _, ev := range events {
		if ev.FileID > 0 && !seen[ev.FileID] {
			seen[ev.FileID] = true
			ids = append(ids, ev.FileID)
		}
	}
	if len(ids) == 0 {
		return
	}

	endpoint := fmt.Sprintf("%s/api/v3/episodefile?", inst.URL)
	for i, fid := range ids {
		if i > 0 {
			endpoint += "&"
		}
		endpoint += fmt.Sprintf("episodeFileIds=%d", fid)
	}

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	body, status, err := client.DoRequest(ctx, "GET", endpoint, headers, nil)
	if err != nil || status != http.StatusOK {
		return
	}

	var files []SonarrEpisodeFileResponse
	if err := json.Unmarshal(body, &files); err != nil {
		return
	}

	subMap := make(map[int]string)
	for _, f := range files {
		if f.MediaInfo != nil && f.MediaInfo.Subtitles != "" {
			subMap[f.ID] = f.MediaInfo.Subtitles
		}
	}

	for i := range events {
		if subs, ok := subMap[events[i].FileID]; ok {
			events[i].Subtitles = subs
		}
	}
}

func fetchRadarrHistory(ctx context.Context, client *httputil.Client, inst config.ArrInstance, since time.Time, showSubtitles bool) ([]HistoryEvent, error) {
	sinceStr := since.UTC().Format(time.RFC3339)
	endpoint := fmt.Sprintf("%s/api/v3/history/since?date=%s&includeMovie=true", inst.URL, sinceStr)

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	body, status, err := client.DoRequest(ctx, "GET", endpoint, headers, nil)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", status, string(body))
	}

	var wrapper struct {
		Records []RadarrHistory `json:"records"`
	}
	var history []RadarrHistory
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.Records != nil {
		history = wrapper.Records
	} else if err := json.Unmarshal(body, &history); err != nil {
		return nil, err
	}

	events := make([]HistoryEvent, 0, len(history))
	for _, h := range history {
		events = append(events, radarrToHistoryEvent(h))
	}

	if showSubtitles {
		enrichRadarrSubtitles(ctx, client, inst, events)
	}

	return events, nil
}

func enrichRadarrSubtitles(ctx context.Context, client *httputil.Client, inst config.ArrInstance, events []HistoryEvent) {
	var ids []int
	seen := make(map[int]bool)
	for _, ev := range events {
		if ev.FileID > 0 && !seen[ev.FileID] {
			seen[ev.FileID] = true
			ids = append(ids, ev.FileID)
		}
	}
	if len(ids) == 0 {
		return
	}

	endpoint := fmt.Sprintf("%s/api/v3/moviefile?", inst.URL)
	for i, fid := range ids {
		if i > 0 {
			endpoint += "&"
		}
		endpoint += fmt.Sprintf("movieFileIds=%d", fid)
	}

	headers := map[string]string{"X-Api-Key": inst.APIKey}
	body, status, err := client.DoRequest(ctx, "GET", endpoint, headers, nil)
	if err != nil || status != http.StatusOK {
		return
	}

	var files []RadarrMovieFileResponse
	if err := json.Unmarshal(body, &files); err != nil {
		return
	}

	subMap := make(map[int]string)
	for _, f := range files {
		if f.MediaInfo != nil && f.MediaInfo.Subtitles != "" {
			subMap[f.ID] = f.MediaInfo.Subtitles
		}
	}

	for i := range events {
		if subs, ok := subMap[events[i].FileID]; ok {
			events[i].Subtitles = subs
		}
	}
}

func sonarrToHistoryEvent(sh SonarrHistory) HistoryEvent {
	when, _ := time.Parse(time.RFC3339, sh.Date)

	event := HistoryEvent{
		Server:      "sonarr",
		When:        when,
		Action:      mapSonarrEventType(sh.EventType),
		Quality:     sh.Quality.Quality.Name,
		SourceTitle: sh.SourceTitle,
		EventType:   sh.EventType,
		ID:          sh.ID,
	}

	if sh.Data != nil {
		for _, key := range []string{"fileId", "FileId"} {
			if fileIDVal, ok := sh.Data[key]; ok {
				if fid, err := parseInt(fmt.Sprintf("%v", fileIDVal)); err == nil {
					event.FileID = fid
				}
				break
			}
		}
	}

	if sh.Series != nil {
		event.Title = sh.Series.Title
	}

	if sh.Episode != nil {
		event.Episode = formatEpisode(sh.Episode.SeasonNumber, sh.Episode.EpisodeNumber)
		event.EpisodeTitle = sh.Episode.Title
	}

	var cf []CustomFormat
	if len(sh.CustomFormats) > 0 {
		cf = sh.CustomFormats
	} else {
		cf = sh.Quality.CustomFormats
	}
	if len(cf) > 0 {
		event.Formats = make([]string, len(cf))
		for i, f := range cf {
			event.Formats[i] = f.Name
		}
	}

	return event
}

func radarrToHistoryEvent(rh RadarrHistory) HistoryEvent {
	when, _ := time.Parse(time.RFC3339, rh.Date)

	event := HistoryEvent{
		Server:      "radarr",
		When:        when,
		Action:      mapRadarrEventType(rh.EventType),
		Quality:     rh.Quality.Quality.Name,
		SourceTitle: rh.SourceTitle,
		EventType:   rh.EventType,
		ID:          rh.ID,
	}

	if rh.Data != nil {
		for _, key := range []string{"fileId", "FileId"} {
			if fileIDVal, ok := rh.Data[key]; ok {
				if fid, err := parseInt(fmt.Sprintf("%v", fileIDVal)); err == nil {
					event.FileID = fid
				}
				break
			}
		}
	}

	if rh.Movie != nil {
		event.Title = rh.Movie.Title
		if rh.Movie.Year > 0 {
			event.Title = fmt.Sprintf("%s (%d)", rh.Movie.Title, rh.Movie.Year)
		}
	}

	var cf []CustomFormat
	if len(rh.CustomFormats) > 0 {
		cf = rh.CustomFormats
	} else {
		cf = rh.Quality.CustomFormats
	}
	if len(cf) > 0 {
		event.Formats = make([]string, len(cf))
		for i, f := range cf {
			event.Formats[i] = f.Name
		}
	}

	return event
}
