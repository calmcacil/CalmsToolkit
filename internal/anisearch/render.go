package anisearch

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// terminalWidth returns the current terminal width, defaulting to 80.
func terminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// innerWidth returns the usable inner width for boxed content.
func innerWidth() int {
	w := terminalWidth()
	if w < 40 {
		w = 40
	}
	if w > 80 {
		w = 80
	}
	return w - 2 // subtract box borders
}

// --- Search Results View ---

// renderSearchResults renders the search results interactive list.
// It returns the string to print to stdout.
func renderSearchResults(query string, result *SearchResult, selected int, mapping *AnibridgeMapping, cfg ToolConfig) string {
	p := colors.GetPalette(cfg.Theme)
	clr := colors.ClrFunc(cfg.NoColor)
	iW := innerWidth()

	var buf bytes.Buffer
	bw := bufioWriter(&buf)

	// Top border + title
	title := fmt.Sprintf(" Searching: %q ", query)
	title = truncateVis(title, iW)
	topLine := "┌" + title + strings.Repeat("─", iW-colors.VisibleLen(title)) + "┐\n"
	bw.WriteString(topLine)

	// Content area
	for i, show := range result.Media {
		highlighted := i == selected
		line := formatSearchLine(i+1, show, highlighted, mapping, p, clr, iW)
		if highlighted {
			// Wrap line in inverse video for the highlight
			line = "\033[7m" + line + "\033[27m"
		}
		bw.WriteString("│" + line + "│\n")
	}

	// Spacer if few results
	for i := len(result.Media); i < 5; i++ {
		bw.WriteString("│" + strings.Repeat(" ", iW) + "│\n")
	}

	// Separator
	bw.WriteString("├" + strings.Repeat("─", iW) + "┤\n")

	// Footer with nav hints
	footer := "  Page " + itoa(result.PageInfo.CurrentPage)
	if result.PageInfo.HasNextPage {
		footer += "  ·  N next"
	}
	footer += "  ·  " + arrowUp + arrowDown + " select  ·  Enter detail  ·  Q quit"
	footer = colors.PadRight(footer, iW)
	bw.WriteString("│" + footer + "│\n")

	// Bottom border
	bw.WriteString("└" + strings.Repeat("─", iW) + "┘\n")

	bw.Flush()
	return buf.String()
}

// formatSearchLine formats a single search result line.
func formatSearchLine(num int, show Show, highlighted bool, mapping *AnibridgeMapping,
	p *colors.Palette, clr func(string) string, iW int) string {

	// Prefix: number + title
	prefix := fmt.Sprintf("%s%2d.%s %s", clr(p.Subdued), num, clr(p.Reset), show.Title.DisplayTitle())

	// Format badge
	fmtBadge := ""
	switch show.Format {
	case "TV":
		fmtBadge = "[TV]"
	case "TV_SHORT":
		fmtBadge = "[Short]"
	case "MOVIE":
		fmtBadge = "[Movie]"
	case "OVA":
		fmtBadge = "[OVA]"
	case "ONA":
		fmtBadge = "[ONA]"
	case "SPECIAL":
		fmtBadge = "[Spcl]"
	default:
		fmtBadge = "[" + show.Format + "]"
	}

	// Meta info
	epLabel := "? eps"
	if show.Episodes != nil {
		epLabel = itoa(*show.Episodes) + " eps"
	}

	scoreLabel := ""
	if show.AverageRank != nil {
		scoreLabel = fmt.Sprintf(" ·  %s%d%%%s", clr(p.Accent), *show.AverageRank, clr(p.Reset))
	}

	meta := fmt.Sprintf(" %s%s %s%s", clr(p.Subdued), fmtBadge, epLabel, scoreLabel)

	// Status indicator
	statusLabel := ""
	switch show.Status {
	case "FINISHED":
		statusLabel = clr(p.Subdued) + " [Finished]" + clr(p.Reset)
	case "RELEASING":
		statusLabel = clr(p.Success) + " [Airing]" + clr(p.Reset)
	case "NOT_YET_RELEASED":
		statusLabel = clr(p.Info) + " [Upcoming]" + clr(p.Reset)
	case "CANCELLED":
		statusLabel = clr(p.Error) + " [Cancelled]" + clr(p.Reset)
	}

	visible := colors.VisibleLen(prefix) + colors.VisibleLen(meta) + colors.VisibleLen(statusLabel)
	padding := iW - visible - 1 // -1 for space
	if padding < 1 {
		maxLabel := iW - colors.VisibleLen(meta) - colors.VisibleLen(statusLabel) - 8
		if maxLabel < 4 {
			maxLabel = 4
		}
		prefix = truncateVis(prefix, maxLabel)
		padding = iW - colors.VisibleLen(prefix) - colors.VisibleLen(meta) - colors.VisibleLen(statusLabel) - 1
		if padding < 1 {
			padding = 1
		}
	}

	return fmt.Sprintf(" %s%s%s%s",
		prefix,
		strings.Repeat(" ", padding),
		meta,
		statusLabel)
}

// --- Detail View ---

// renderDetail renders the full detail view for a single show.
func renderDetail(show Show, tvdbID int, cfg ToolConfig) string {
	p := colors.GetPalette(cfg.Theme)
	clr := colors.ClrFunc(cfg.NoColor)
	iW := innerWidth()

	var buf bytes.Buffer
	bw := bufioWriter(&buf)

	// Top border + title
	title := " " + show.Title.DisplayTitle() + " "
	title = truncateVis(title, iW)
	topLine := "┌" + title + strings.Repeat("─", iW-colors.VisibleLen(title)) + "┐\n"
	bw.WriteString(topLine)

	// Native title (if different from display title)
	if show.Title.Native != nil && *show.Title.Native != "" &&
		(show.Title.Romaji == nil || *show.Title.Romaji != *show.Title.Native) &&
		(show.Title.English == nil || *show.Title.English != *show.Title.Native) {
		nativeTitle := truncateVis(*show.Title.Native, iW-1)
		nativeLine := " " + clr(p.Subdued) + nativeTitle + clr(p.Reset)
		nativeLine = colors.PadRight(nativeLine, iW)
		bw.WriteString("│" + nativeLine + "│\n")
	}

	bw.WriteString("│" + strings.Repeat(" ", iW) + "│\n")

	// Two-column info: (Format/Status, Episodes/Season, Score/Popularity, Studios/TVDB)
	infoRows := [][2]string{
		{"Format", show.Format},
		{"Episodes", formatEpisodes(show.Episodes)},
		{"Score", formatScore(show.AverageRank)},
		{"Studios", strings.Join(show.StudioNames(), ", ")},
		{"Status", formatStatus(show.Status)},
		{"Season", formatSeason(show.Season, show.SeasonYear)},
		{"Popularity", formatPopularity(show.Popularity)},
		{"TVDB ID", formatTVDB(tvdbID)},
	}

	// Render info in a 2-column grid (4 rows × 2 cols each)
	for row := 0; row < 4; row++ {
		left := infoRows[row*2]
		right := infoRows[row*2+1]

		colWidth := iW / 2
		rightWidth := iW - colWidth
		leftPrefix := fmt.Sprintf(" %s%s:%s ", clr(p.Bold), left[0], clr(p.Reset))
		rightPrefix := fmt.Sprintf("%s%s:%s ", clr(p.Bold), right[0], clr(p.Reset))
		leftValue := truncateVis(left[1], colWidth-colors.VisibleLen(leftPrefix))
		rightValue := truncateVis(right[1], rightWidth-colors.VisibleLen(rightPrefix))
		leftStr := colors.PadRight(leftPrefix+leftValue, colWidth)
		rightStr := colors.PadRight(rightPrefix+rightValue, rightWidth)
		line := leftStr + rightStr
		bw.WriteString("│" + line + "│\n")
	}

	// Genres
	if len(show.Genres) > 0 {
		genres := truncateVis(strings.Join(show.Genres, ", "), iW-1)
		genreLine := " " + clr(p.Subdued) + genres + clr(p.Reset)
		genreLine = colors.PadRight(genreLine, iW)
		bw.WriteString("│" + genreLine + "│\n")
	}

	// Tags (only show first 8)
	if len(show.Tags) > 0 {
		tagNames := make([]string, 0, 8)
		for _, t := range show.Tags {
			tagNames = append(tagNames, t.Name)
			if len(tagNames) >= 8 {
				break
			}
		}
		if len(tagNames) > 0 {
			tags := truncateVis("Tags: "+strings.Join(tagNames, ", "), iW-1)
			tagLine := " " + clr(p.Subdued) + tags + clr(p.Reset)
			tagLine = colors.PadRight(tagLine, iW)
			bw.WriteString("│" + tagLine + "│\n")
		}
	}

	// Description (word-wrapped)
	desc := show.Description
	if desc != "" {
		bw.WriteString("│" + strings.Repeat(" ", iW) + "│\n")

		// Strip HTML tags from description
		desc = stripHTML(desc)

		// Word wrap
		lines := wordWrap(desc, iW-2)
		maxDescLines := 8
		if len(lines) > maxDescLines {
			lines = lines[:maxDescLines]
			lines = append(lines, clr(p.Subdued)+"... (truncated)"+clr(p.Reset))
		}
		for _, line := range lines {
			line = truncateVis(line, iW-2)
			line = " " + line
			line = colors.PadRight(line, iW)
			bw.WriteString("│" + line + "│\n")
		}
	}

	// Separator
	bw.WriteString("├" + strings.Repeat("─", iW) + "┤\n")

	// Footer
	footer := "  B back  ·  Q quit"
	footer = colors.PadRight(footer, iW)
	bw.WriteString("│" + footer + "│\n")

	// Bottom border
	bw.WriteString("└" + strings.Repeat("─", iW) + "┘\n")

	bw.Flush()
	return buf.String()
}

// --- Formatting helpers ---

func formatEpisodes(ep *int) string {
	if ep == nil {
		return "?"
	}
	return itoa(*ep)
}

func formatScore(score *int) string {
	if score == nil {
		return "—"
	}
	return itoa(*score) + "%"
}

func formatStatus(s string) string {
	switch s {
	case "FINISHED":
		return "Finished"
	case "RELEASING":
		return "Airing"
	case "NOT_YET_RELEASED":
		return "Not Yet Released"
	case "CANCELLED":
		return "Cancelled"
	case "HIATUS":
		return "Hiatus"
	default:
		return s
	}
}

func formatSeason(season string, year *int) string {
	s := ""
	switch season {
	case "WINTER":
		s = "Winter"
	case "SPRING":
		s = "Spring"
	case "SUMMER":
		s = "Summer"
	case "FALL":
		s = "Fall"
	default:
		s = season
	}
	if year != nil {
		s += " " + itoa(*year)
	}
	return s
}

func formatPopularity(p *int) string {
	if p == nil {
		return "—"
	}
	return "#" + itoa(*p)
}

func formatTVDB(tvdbID int) string {
	if tvdbID <= 0 {
		return "—"
	}
	return itoa(tvdbID)
}

// --- Utilities ---

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// arrowUp and arrowDown are Unicode arrows for the footer hints.
const (
	arrowUp   = "\u2191"
	arrowDown = "\u2193"
)

// bufioWriter creates a convenience writer that matches the bufio.Writer
// interface but wraps a bytes.Buffer.
func bufioWriter(buf *bytes.Buffer) *simpleWriter {
	return &simpleWriter{buf: buf}
}

type simpleWriter struct {
	buf *bytes.Buffer
}

func (w *simpleWriter) WriteString(s string) (int, error) {
	return w.buf.WriteString(s)
}

func (w *simpleWriter) Flush() error {
	return nil
}

// truncateVis truncates a string to a visible width, appending "..." if truncated.
func truncateVis(s string, maxLen int) string {
	visLen := colors.VisibleLen(s)
	if visLen <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	// Remove runes from end until we fit with ellipsis
	runes := []rune(s)
	for colors.VisibleLen(string(runes)+"...") > maxLen && len(runes) > 0 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}

// stripHTML removes HTML tags from a string.
func stripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	// Decode common HTML entities
	resultStr := result.String()
	resultStr = strings.ReplaceAll(resultStr, "&amp;", "&")
	resultStr = strings.ReplaceAll(resultStr, "&lt;", "<")
	resultStr = strings.ReplaceAll(resultStr, "&gt;", ">")
	resultStr = strings.ReplaceAll(resultStr, "&quot;", "\"")
	resultStr = strings.ReplaceAll(resultStr, "&#039;", "'")
	resultStr = strings.ReplaceAll(resultStr, "&#39;", "'")
	resultStr = strings.ReplaceAll(resultStr, "&nbsp;", " ")
	return resultStr
}

// wordWrap wraps text to a maximum width, respecting word boundaries.
func wordWrap(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	var current strings.Builder
	currentWriteable := 0

	for _, word := range words {
		wordLen := runewidth.StringWidth(word)
		if currentWriteable == 0 {
			current.WriteString(word)
			currentWriteable = wordLen
		} else if currentWriteable+1+wordLen <= width {
			current.WriteString(" ")
			current.WriteString(word)
			currentWriteable += 1 + wordLen
		} else {
			lines = append(lines, current.String())
			current.Reset()
			current.WriteString(word)
			currentWriteable = wordLen
		}
	}

	if currentWriteable > 0 {
		lines = append(lines, current.String())
	}

	return lines
}

// formatRelativeTime returns a human-friendly relative time string.
func formatRelativeTime(t time.Time) string {
	diff := time.Since(t)
	if diff < 0 {
		diff = -diff
	}
	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		m := int(diff.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	}
	if diff < 24*time.Hour {
		h := int(diff.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	}
	d := int(diff.Hours() / 24)
	if d == 1 {
		return "yesterday"
	}
	return fmt.Sprintf("%d days ago", d)
}
