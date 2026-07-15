package anisearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"
)

// --- Anibridge mapping constants ---

const (
	defaultAnibridgeHTTPTimeout = 60 * time.Second
	mappingCacheDuration        = 24 * time.Hour
	defaultMappingURL           = "https://github.com/anibridge/anibridge-mappings/releases/download/v3/mappings.json.zst"
)

// mappingMetadata is persisted as a sidecar file next to the cached mapping.
type mappingMetadata struct {
	ETag         string    `json:"etag,omitempty"`
	LastModified string    `json:"last_modified,omitempty"`
	FetchedAt    time.Time `json:"fetched_at"`
	URL          string    `json:"url"`
}

// loadMapping loads the anibridge mapping from cache or downloads it from the
// upstream URL. Returns nil if the mapping cannot be loaded (the caller can
// proceed without TVDB IDs).
func loadMapping(ctx context.Context, path, url string, forceRefresh bool) *AnibridgeMapping {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			slog.Warn("cannot determine home dir for mapping cache", "error", err)
			return nil
		}
		path = home + "/.config/calmstoolkit/anibridge_mappings.json.zst"
	}
	if url == "" {
		url = defaultMappingURL
	}

	metaPath := path + ".meta.json"

	// Try to load from cache first (unless force refresh).
	if !forceRefresh {
		m, err := parseAnibridgeFile(path)
		if err == nil {
			meta, _ := readMappingMeta(metaPath)

			// If cache is fresh enough, use it directly.
			if !meta.FetchedAt.IsZero() && time.Since(meta.FetchedAt) < mappingCacheDuration {
				slog.Debug("using cached anibridge mapping", "path", path)
				return m
			}

			// Cache is stale; try a conditional HEAD.
			if meta.ETag != "" {
				upstreamMeta, headErr := headMapping(ctx, url)
				if headErr == nil && strings.EqualFold(
					strings.TrimSpace(upstreamMeta.ETag),
					strings.TrimSpace(meta.ETag),
				) {
					slog.Debug("anibridge mapping unchanged (ETag match)")
					// Update fetched_at so we don't HEAD again too soon.
					meta.FetchedAt = time.Now()
					_ = writeMappingMeta(metaPath, meta)
					return m
				}
				if headErr != nil {
					slog.Debug("anibridge HEAD failed, using stale cache", "error", headErr)
					return m
				}
				// ETag changed; fall through to download.
			}
		} else if !os.IsNotExist(err) {
			slog.Debug("cached anibridge mapping unreadable, will re-download", "error", err)
		}
	}

	// Download fresh mapping.
	data, newMeta, err := fetchMapping(ctx, url)
	if err != nil {
		// Try fallback to stale cache.
		m, fallbackErr := parseAnibridgeFile(path)
		if fallbackErr == nil {
			slog.Warn("anibridge fetch failed, using cached mapping", "error", err)
			return m
		}
		slog.Warn("anibridge mapping not available", "error", err)
		return nil
	}

	// Write to disk atomically.
	if writeErr := writeFileAtomic(path, data); writeErr != nil {
		slog.Warn("failed to cache anibridge mapping", "error", writeErr)
	}

	// Write metadata sidecar.
	_ = writeMappingMeta(metaPath, newMeta)

	// Parse.
	m, err := parseAnibridgeBytes(data)
	if err != nil {
		slog.Warn("failed to parse anibridge mapping", "error", err)
		return nil
	}

	slog.Debug("loaded anibridge mapping",
		"mal_entries", len(m.byMAL),
		"anilist_entries", len(m.byAniList))
	return m
}

// headMapping performs a HEAD request to check the remote ETag.
func headMapping(ctx context.Context, url string) (mappingMetadata, error) {
	client := &http.Client{Timeout: defaultAnibridgeHTTPTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return mappingMetadata{}, fmt.Errorf("create HEAD request: %w", err)
	}
	req.Header.Set("User-Agent", anilistUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return mappingMetadata{}, fmt.Errorf("HEAD failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return mappingMetadata{}, fmt.Errorf("HEAD returned HTTP %d", resp.StatusCode)
	}

	return mappingMetadata{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		FetchedAt:    time.Now(),
		URL:          url,
	}, nil
}

// fetchMapping downloads the full mapping file.
func fetchMapping(ctx context.Context, url string) ([]byte, mappingMetadata, error) {
	client := &http.Client{Timeout: defaultAnibridgeHTTPTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, mappingMetadata{}, fmt.Errorf("create GET request: %w", err)
	}
	req.Header.Set("User-Agent", anilistUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, mappingMetadata{}, fmt.Errorf("GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, mappingMetadata{}, fmt.Errorf("GET returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, mappingMetadata{}, fmt.Errorf("read body: %w", err)
	}

	meta := mappingMetadata{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		FetchedAt:    time.Now(),
		URL:          url,
	}

	return data, meta, nil
}

// parseAnibridgeFile opens and parses a cached mapping file.
func parseAnibridgeFile(path string) (*AnibridgeMapping, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	zr, err := zstd.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("create zstd reader: %w", err)
	}
	defer zr.Close()

	return parseAnibridgeJSON(zr)
}

// parseAnibridgeBytes parses mapping data from memory.
func parseAnibridgeBytes(data []byte) (*AnibridgeMapping, error) {
	zr, err := zstd.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create zstd reader: %w", err)
	}
	defer zr.Close()
	return parseAnibridgeJSON(zr)
}

// parseAnibridgeJSON reads the JSON mapping and extracts MAL and AniList → TVDB mappings.
func parseAnibridgeJSON(r io.Reader) (*AnibridgeMapping, error) {
	dec := json.NewDecoder(r)

	t, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("expected opening brace: %w", err)
	}
	if t != json.Delim('{') {
		return nil, fmt.Errorf("expected '{', got %T(%v)", t, t)
	}

	byMAL := map[int]int{}
	byAniList := map[int]int{}

	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("key token: %w", err)
		}
		key, ok := t.(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %T", t)
		}

		switch {
		case strings.HasPrefix(key, "mal:"):
			id, convErr := strconv.Atoi(key[4:])
			if convErr != nil || id <= 0 {
				skipJSONValue(dec)
				continue
			}
			if tvdbID, ok := extractTVDB(dec); ok {
				byMAL[id] = tvdbID
			}

		case strings.HasPrefix(key, "anilist:"):
			id, convErr := strconv.Atoi(key[8:])
			if convErr != nil || id <= 0 {
				skipJSONValue(dec)
				continue
			}
			if tvdbID, ok := extractTVDB(dec); ok {
				byAniList[id] = tvdbID
			}

		default:
			skipJSONValue(dec)
		}
	}

	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("expected closing brace: %w", err)
	}

	return &AnibridgeMapping{byMAL: byMAL, byAniList: byAniList}, nil
}

// extractTVDB parses the `tvdb_show:<ID>:s<N>` entries from a mapping value,
// preferring s1 season scope and falling back to the scope with the most episodes.
func extractTVDB(dec *json.Decoder) (int, bool) {
	var targets map[string]json.RawMessage
	if err := dec.Decode(&targets); err != nil {
		return 0, false
	}

	bestTVDB := 0
	bestEpCount := -1
	bestIsS1 := false

	for descriptor, rawValue := range targets {
		if !strings.HasPrefix(descriptor, "tvdb_show:") {
			continue
		}

		parts := strings.SplitN(descriptor, ":", 3)
		if len(parts) < 3 {
			continue
		}
		tvdbID, convErr := strconv.Atoi(parts[1])
		if convErr != nil || tvdbID <= 0 {
			continue
		}
		scope := parts[2]

		epCount := countSourceEpisodes(rawValue)

		isS1 := scope == "s1"
		// Season 1 is an absolute preference.  Compare episode counts only
		// within the same preference class so map iteration order cannot let a
		// larger non-s1 mapping displace an s1 mapping.
		if (isS1 && !bestIsS1) || (isS1 == bestIsS1 && (epCount > bestEpCount || (epCount == bestEpCount && (bestTVDB == 0 || tvdbID < bestTVDB)))) {
			bestTVDB = tvdbID
			bestEpCount = epCount
			bestIsS1 = isS1
		}
	}

	if bestTVDB > 0 {
		return bestTVDB, true
	}
	return 0, false
}

// countSourceEpisodes counts the total number of episode ranges in a mapping value.
func countSourceEpisodes(raw json.RawMessage) int {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}

	var ranges map[string]string
	if err := json.Unmarshal(raw, &ranges); err != nil {
		return 0
	}

	var total int
	for srcRange := range ranges {
		if srcRange == "" {
			continue
		}
		parts := strings.SplitN(srcRange, "-", 2)
		if len(parts) == 1 {
			if ep, err := strconv.Atoi(parts[0]); err == nil && ep > 0 {
				total++
			}
			continue
		}
		if parts[1] == "" {
			if start, err := strconv.Atoi(parts[0]); err == nil && start > 0 {
				total++
			}
			continue
		}
		start, startErr := strconv.Atoi(parts[0])
		end, endErr := strconv.Atoi(parts[1])
		if startErr == nil && endErr == nil && start > 0 && end >= start {
			total += end - start + 1
		}
	}
	return total
}

// skipJSONValue reads and discards one JSON value from the decoder stream.
func skipJSONValue(dec *json.Decoder) {
	var raw json.RawMessage
	_ = dec.Decode(&raw) // best-effort skip
}

// --- File I/O helpers ---

func readMappingMeta(path string) (mappingMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mappingMetadata{}, nil
		}
		return mappingMetadata{}, err
	}
	var m mappingMetadata
	if err := json.Unmarshal(data, &m); err != nil {
		return mappingMetadata{}, err
	}
	return m, nil
}

func writeMappingMeta(path string, m mappingMetadata) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

func writeFileAtomic(path string, data []byte) error {
	dir := path[:strings.LastIndex(path, "/")+1]
	if dir != "" && dir != "/" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create directory: %w", err)
		}
	}

	tmp, err := os.CreateTemp(dir, "anisearch-mapping-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close() //nolint:errcheck
		return fmt.Errorf("write file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	cleanup = false
	return nil
}
