package feed

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/console"
)

func formatRelativeTime(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "Just now"
	}

	if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}

	if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}

	if duration < 7*24*time.Hour {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}

	return t.Local().Format("2006-01-02 15:04")
}

func filterEvents(events []HistoryEvent, cfg ToolConfig) []HistoryEvent {
	filtered := make([]HistoryEvent, 0, len(events))

	for _, event := range events {
		switch event.Action {
		case "Grabbed":
			if !cfg.ShowGrabbed {
				continue
			}
		case "Imported", "Bulk Import":
			if !cfg.ShowImported {
				continue
			}
		case "Failed":
			if !cfg.ShowFailed {
				continue
			}
		case "Deleted":
			if !cfg.ShowDeleted {
				continue
			}
		case "Ignored":
			if !cfg.ShowIgnored {
				continue
			}
		}

		filtered = append(filtered, event)
	}

	return filtered
}

func boxTop(bw *bufio.Writer, colWidths []int, hasSubtitles bool) {
	fmt.Fprint(bw, "┌")
	for i, w := range colWidths {
		fmt.Fprint(bw, strings.Repeat("─", w))
		if i < len(colWidths)-1 {
			fmt.Fprint(bw, "┬")
		}
	}
	fmt.Fprint(bw, "┐\n")
}

func boxSep(bw *bufio.Writer, colWidths []int, hasSubtitles bool) {
	fmt.Fprint(bw, "├")
	for i, w := range colWidths {
		fmt.Fprint(bw, strings.Repeat("─", w))
		if i < len(colWidths)-1 {
			fmt.Fprint(bw, "┼")
		}
	}
	fmt.Fprint(bw, "┤\n")
}

func boxBottom(bw *bufio.Writer, colWidths []int, hasSubtitles bool) {
	fmt.Fprint(bw, "└")
	for i, w := range colWidths {
		fmt.Fprint(bw, strings.Repeat("─", w))
		if i < len(colWidths)-1 {
			fmt.Fprint(bw, "┴")
		}
	}
	fmt.Fprint(bw, "┘\n")
}

func renderTable(events []HistoryEvent, cfg ToolConfig, p *colors.Palette) {
	color := getColorFunc(cfg, p)

	termWidth := 120
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		termWidth = w
	}

	props := []int{12, 8, 28, 8, 20, 12, 15}
	headers := []string{"When", "Action", "Series/Movie", "Episode", "Episode Title", "Quality", "Formats"}
	if cfg.ShowSubtitles {
		props = append(props, 10)
		headers = append(headers, "Subtitles")
	}

	totalCols := len(props)
	sumProp := 0
	for _, p := range props {
		sumProp += p
	}
	availWidth := termWidth - totalCols - 1

	colWidths := make([]int, totalCols)
	for i, p := range props {
		cw := availWidth * p / sumProp
		if cw < 5 {
			cw = 5
		}
		colWidths[i] = cw
	}

	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	if len(events) == 0 {
		boxW := termWidth - 2
		msg := "No events found"
		fmt.Fprint(bw, "┌")
		fmt.Fprint(bw, strings.Repeat("─", boxW))
		fmt.Fprint(bw, "┐\n")
		fmt.Fprint(bw, "│")
		left := (boxW - len(msg)) / 2
		if left < 0 {
			left = 0
		}
		fmt.Fprint(bw, strings.Repeat(" ", left))
		fmt.Fprint(bw, msg)
		fmt.Fprint(bw, strings.Repeat(" ", boxW-left-len(msg)))
		fmt.Fprint(bw, "│\n")
		fmt.Fprint(bw, "└")
		fmt.Fprint(bw, strings.Repeat("─", boxW))
		fmt.Fprint(bw, "┘\n")
		bw.Flush()
		os.Stdout.Write(buf.Bytes())
		return
	}

	boxTop(bw, colWidths, cfg.ShowSubtitles)
	fmt.Fprint(bw, "│")
	for i, h := range headers {
		fmt.Fprint(bw, color(p.Bold))
		fmt.Fprint(bw, center(h, colWidths[i]))
		fmt.Fprint(bw, color(p.Reset))
		fmt.Fprint(bw, "│")
	}
	fmt.Fprintln(bw)

	boxSep(bw, colWidths, cfg.ShowSubtitles)

	for _, event := range events {
		actionColor := getActionColor(event.Action, p)
		timeStr := formatRelativeTime(event.When)
		title := truncateWithEllipsis(event.Title, colWidths[2])
		epiTitle := truncateWithEllipsis(event.EpisodeTitle, colWidths[4])
		quality := truncateWithEllipsis(event.Quality, colWidths[5])
		formats := truncateWithEllipsis(strings.Join(event.Formats, ", "), colWidths[6])

		vals := []string{
			center(timeStr, colWidths[0]),
			center(event.Action, colWidths[1]),
			center(title, colWidths[2]),
			center(event.Episode, colWidths[3]),
			center(epiTitle, colWidths[4]),
			center(quality, colWidths[5]),
			center(formats, colWidths[6]),
		}
		if cfg.ShowSubtitles {
			subs := truncateWithEllipsis(subtitlesDisplay(event.Subtitles), colWidths[7])
			vals = append(vals, center(subs, colWidths[7]))
		}

		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, vals[0])
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, color(actionColor))
		fmt.Fprint(bw, vals[1])
		fmt.Fprint(bw, color(p.Reset))
		fmt.Fprint(bw, "│")
		for i := 2; i < len(vals); i++ {
			fmt.Fprint(bw, vals[i])
			fmt.Fprint(bw, "│")
		}
		fmt.Fprintln(bw)
	}

	boxBottom(bw, colWidths, cfg.ShowSubtitles)

	fmt.Fprintf(bw, "\n%sTotal events: %d%s\n", color(p.Bold), len(events), color(p.Reset))

	bw.Flush()
	os.Stdout.Write(buf.Bytes())
}

func renderJSON(events []HistoryEvent, partial bool, warnings []string) {
	_ = console.WriteEnvelope(os.Stdout, "feed", events, partial, warnings, time.Now())
}

func getActionColor(action string, p *colors.Palette) string {
	switch action {
	case "Imported", "Bulk Import":
		return p.Success
	case "Grabbed":
		return p.Grabbed
	case "Failed":
		return p.Error
	case "Deleted":
		return p.Warning
	case "Ignored":
		return p.Subdued
	case "Renamed":
		return p.Renamed
	default:
		return p.Reset
	}
}

func getColorFunc(cfg ToolConfig, p *colors.Palette) func(string) string {
	if cfg.NoColor || cfg.JSONOutput {
		return colors.ClrFunc(true)
	}
	return colors.ClrFunc(false)
}
