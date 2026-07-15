package anisearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// --- Constants ---

const (
	anilistUserAgent = "calmstoolkit-anisearch/1.0"
	maxPerPage       = 50
)

var (
	aniListEndpoint  = "https://graphql.anilist.co"
	maxRetry         = 5
	rateLimitDelay   = 700 * time.Millisecond
	rateLimitBackoff = 5 * time.Second
)

// searchQuery fetches anime matching a text search, sorted by relevance.
// AniList's search parameter provides fuzzy/partial matching out of the box.
const searchQuery = `query($search: String, $page: Int, $perPage: Int) {
	Page(page: $page, perPage: $perPage) {
		pageInfo {
			hasNextPage
			currentPage
			total
		}
		media(search: $search, type: ANIME, sort: SEARCH_MATCH) {
			id
			idMal
			title { romaji english native }
			format
			episodes
			status
			season
			seasonYear
			genres
			tags { name }
			averageScore
			popularity
			description(asHtml: false)
			studios { nodes { name } }
		}
	}
}`

// --- AniList HTTP Client ---

// AniListClient fetches data from the AniList GraphQL API with built-in
// rate limiting and retries.
type AniListClient struct {
	http    *http.Client
	limiter *rate.Limiter

	rateLimitMu   sync.Mutex
	lastRateLimit time.Time
}

// NewAniListClient creates a new AniList client with the given HTTP timeout.
func NewAniListClient(timeout time.Duration) *AniListClient {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &AniListClient{
		http:    &http.Client{Timeout: timeout},
		limiter: rate.NewLimiter(rate.Every(rateLimitDelay), 1),
	}
}

// jitter returns d randomly varied by ±25% to prevent synchronized retry storms.
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	quarter := d / 4
	offset := time.Duration(rand.Int64N(int64(2*quarter+1))) - quarter
	return d + offset
}

// throttle ensures we don't exceed AniList rate limits. After a 429 response,
// the limit is tightened to 5s between requests for 30 seconds.
func (c *AniListClient) throttle(ctx context.Context) error {
	c.rateLimitMu.Lock()
	limit := rate.Every(rateLimitDelay)
	if time.Since(c.lastRateLimit) < 30*time.Second {
		limit = rate.Every(rateLimitBackoff)
	}
	c.limiter.SetLimit(limit)
	c.rateLimitMu.Unlock()

	return c.limiter.Wait(ctx)
}

// Search performs a text search on AniList, paginating through all pages and
// returning all matching results. Use perPage to control the page size (max 50).
func (c *AniListClient) Search(ctx context.Context, q string, perPage int) (*SearchResult, error) {
	if perPage <= 0 || perPage > maxPerPage {
		perPage = maxPerPage
	}

	var allMedia []Show
	page := 1
	var firstPageInfo *PageInfo

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		payload := map[string]any{
			"query": searchQuery,
			"variables": map[string]any{
				"search":  q,
				"page":    page,
				"perPage": perPage,
			},
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}

		var resp graphqlResponse
		if err := c.doRequest(ctx, body, &resp); err != nil {
			return nil, fmt.Errorf("search %q (page %d): %w", q, page, err)
		}

		if len(resp.Errors) > 0 {
			msgs := make([]string, len(resp.Errors))
			for i, e := range resp.Errors {
				msgs[i] = e.Message
			}
			return nil, fmt.Errorf("AniList errors: %s", strings.Join(msgs, "; "))
		}

		shows := resp.Data.Page.Media
		if shows == nil {
			shows = []Show{}
		}

		if firstPageInfo == nil {
			pi := resp.Data.Page.PageInfo
			firstPageInfo = &pi
		}

		allMedia = append(allMedia, shows...)

		if !resp.Data.Page.PageInfo.HasNextPage {
			break
		}
		page++
	}

	result := &SearchResult{
		Media: allMedia,
	}
	if firstPageInfo != nil {
		result.PageInfo = *firstPageInfo
	}
	result.PageInfo.CurrentPage = 1
	result.PageInfo.HasNextPage = page > 1 && page < result.PageInfo.Total
	return result, nil
}

// SearchPage fetches a single page of AniList search results, useful for
// paginated interactive browsing.
func (c *AniListClient) SearchPage(ctx context.Context, q string, page, perPage int) (*SearchResult, error) {
	if perPage <= 0 || perPage > maxPerPage {
		perPage = maxPerPage
	}
	if page < 1 {
		page = 1
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	payload := map[string]any{
		"query": searchQuery,
		"variables": map[string]any{
			"search":  q,
			"page":    page,
			"perPage": perPage,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	var resp graphqlResponse
	if err := c.doRequest(ctx, body, &resp); err != nil {
		return nil, fmt.Errorf("search %q (page %d): %w", q, page, err)
	}

	if len(resp.Errors) > 0 {
		msgs := make([]string, len(resp.Errors))
		for i, e := range resp.Errors {
			msgs[i] = e.Message
		}
		return nil, fmt.Errorf("AniList errors: %s", strings.Join(msgs, "; "))
	}

	result := &SearchResult{
		PageInfo: resp.Data.Page.PageInfo,
		Media:    resp.Data.Page.Media,
	}
	if result.Media == nil {
		result.Media = []Show{}
	}
	return result, nil
}

// doRequest sends a POST request with retries and exponential backoff.
func (c *AniListClient) doRequest(ctx context.Context, payload []byte, dst any) error {
	var lastErr error
	for attempt := range maxRetry {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(jitter(time.Duration(1<<attempt) * time.Second)):
			}
		}

		if err := c.throttle(ctx); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, aniListEndpoint,
			bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", anilistUserAgent)

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http request: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			c.rateLimitMu.Lock()
			c.lastRateLimit = time.Now()
			c.rateLimitMu.Unlock()
			retryAfter := resp.Header.Get("Retry-After")
			_ = resp.Body.Close()
			if retryAfter != "" {
				if sec, err := strconv.Atoi(retryAfter); err == nil && sec > 0 {
					slog.Warn("rate limited, waiting retry-after", "seconds", sec, "attempt", attempt+1)
					select {
					case <-ctx.Done():
						return ctx.Err()
					case <-time.After(time.Duration(sec) * time.Second):
					}
				}
			}
			lastErr = fmt.Errorf("rate limited (attempt %d)", attempt+1)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			respBody, readErr := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if readErr != nil {
				lastErr = fmt.Errorf("API error (HTTP %d): failed to read body: %w", resp.StatusCode, readErr)
			} else {
				lastErr = fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(respBody))
			}
			// Client errors (4xx except 429) won't self-heal; break.
			if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != 429 {
				break
			}
			continue
		}

		if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("decode response: %w", err)
			break
		}
		_ = resp.Body.Close()
		return nil
	}

	return fmt.Errorf("giving up after %d retries: %w", maxRetry, lastErr)
}
