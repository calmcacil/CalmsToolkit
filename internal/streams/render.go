package streams

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"github.com/calmcacil/CalmsToolkit/internal/console"
)

func formatTimeSince(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		seconds := int(duration.Seconds())
		if seconds <= 1 {
			return "1 second ago"
		}
		return fmt.Sprintf("%d seconds ago", seconds)
	}

	if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	}

	hours := int(duration.Hours())
	if hours == 1 {
		return "1 hour ago"
	}
	return fmt.Sprintf("%d hours ago", hours)
}

func displayJSONOutput(streams []StreamInfo, plexCount, jellyfinCount int, partial bool, warnings []string) error {
	summary := Summary{
		TotalStreams:    len(streams),
		PlexStreams:     plexCount,
		JellyfinStreams: jellyfinCount,
		Timestamp:       time.Now(),
		Streams:         streams,
	}

	totalBandwidth := 0.0
	transcodeCount := 0

	for _, stream := range streams {
		totalBandwidth += stream.Bandwidth
		if stream.Transcoding {
			transcodeCount++
		}
	}

	summary.TranscodingCount = transcodeCount
	summary.TotalBandwidth = totalBandwidth

	return console.WriteEnvelope(os.Stdout, "streams", summary, partial, warnings, time.Now())
}

func renderProgressBar(pct float64, width int) string {
	if pct <= 0 {
		return strings.Repeat(" ", width)
	}
	filled := int(pct * float64(width) / 100.0)
	if filled > width {
		filled = width
	}
	empty := width - filled
	return strings.Repeat("█", filled) + strings.Repeat("░", empty)
}

const maxBoxInnerWidth = 120

func getTermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

func getBoxWidth(termW int) (inner, outer int) {
	outer = termW
	inner = outer - 2
	if inner < 40 {
		inner = 40
		outer = inner + 2
	}
	if inner > maxBoxInnerWidth {
		inner = maxBoxInnerWidth
		outer = inner + 2
	}
	return
}

func buildServerLabel(plexCount, jellyfinCount int) string {
	var parts []string
	if plexCount > 0 {
		parts = append(parts, fmt.Sprintf("Plex: %d", plexCount))
	}
	if jellyfinCount > 0 {
		parts = append(parts, fmt.Sprintf("Jellyfin: %d", jellyfinCount))
	}
	return strings.Join(parts, ", ")
}

func boxStreamTop(bw *bufio.Writer, termW int) {
	fmt.Fprint(bw, "┌")
	fmt.Fprint(bw, strings.Repeat("─", termW-2))
	fmt.Fprint(bw, "┐\n")
}

func boxStreamSep(bw *bufio.Writer, termW int) {
	fmt.Fprint(bw, "├")
	fmt.Fprint(bw, strings.Repeat("─", termW-2))
	fmt.Fprint(bw, "┤\n")
}

func boxStreamBottom(bw *bufio.Writer, termW int) {
	fmt.Fprint(bw, "└")
	fmt.Fprint(bw, strings.Repeat("─", termW-2))
	fmt.Fprint(bw, "┘\n")
}

func displayTerminalOutput(streams []StreamInfo, plexCount, jellyfinCount int, noColor bool, p *colors.Palette) error {
	clr := colors.ClrFunc(noColor)

	rawW := getTermWidth()
	boxW, termW := getBoxWidth(rawW)

	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	// Title banner
	title := "Media Streams Monitor"
	prefix := "┌── " + title + " ──"
	rc := colors.VisibleLen(prefix)
	padLen := termW - rc - 1
	if padLen < 0 {
		padLen = 0
	}
	fmt.Fprint(bw, prefix)
	fmt.Fprint(bw, strings.Repeat("─", padLen))
	fmt.Fprint(bw, "┐\n")

	// Header
	serverLabel := buildServerLabel(plexCount, jellyfinCount)
	header := "Active Sessions"
	if len(streams) == 0 {
		header = "Active Sessions: 0"
	} else {
		header = fmt.Sprintf("Active Sessions: %s%d%s", clr(p.Bold), len(streams), clr(p.Reset))
		if serverLabel != "" {
			header += fmt.Sprintf(" (%s)", serverLabel)
		}
	}
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(" "+header+" ", boxW))
	fmt.Fprint(bw, "│\n")

	// Empty state
	if len(streams) == 0 {
		boxStreamBottom(bw, termW)
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, clr(p.Success))
		fmt.Fprint(bw, colors.PadRight(" No active streams ", boxW))
		fmt.Fprint(bw, clr(p.Reset))
		fmt.Fprint(bw, "│\n")
		boxStreamBottom(bw, termW)
		bw.Flush()
		os.Stdout.Write(buf.Bytes())
		return nil
	}

	// Close outer box before grid
	boxStreamBottom(bw, termW)

	// Stream grid
	renderStreamGrid(bw, streams, termW, noColor, p)

	// Summary standalone box
	boxStreamTop(bw, termW)
	displayStreamSummaryToBox(bw, streams, boxW, noColor, p)
	boxStreamBottom(bw, termW)

	bw.Flush()
	os.Stdout.Write(buf.Bytes())
	return nil
}

func displayTerminalOutputWithHistory(currentStreams []StreamInfo, history *SessionHistory, plexCount, jellyfinCount int, noColor, plain bool, p *colors.Palette) error {
	clr := colors.ClrFunc(noColor)

	rawW := getTermWidth()
	boxW, termW := getBoxWidth(rawW)

	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	if !plain {
		fmt.Fprint(bw, colors.ClearScreen+colors.HomeCursor)
	}

	// Title
	title := "Media Streams Monitor"
	prefix := "┌── " + title + " ──"
	rc := colors.VisibleLen(prefix)
	padLen := termW - rc - 1
	if padLen < 0 {
		padLen = 0
	}
	fmt.Fprint(bw, prefix)
	fmt.Fprint(bw, strings.Repeat("─", padLen))
	fmt.Fprint(bw, "┐\n")

	active, ended := getActiveAndEndedSessions(history)
	serverLabel := buildServerLabel(plexCount, jellyfinCount)

	// Active count
	header := fmt.Sprintf("Active Sessions: %s%d%s", clr(p.Bold), len(active), clr(p.Reset))
	if serverLabel != "" {
		header += fmt.Sprintf(" (%s)", serverLabel)
	}
	if len(ended) > 0 {
		header += fmt.Sprintf("    Ended: %s%d%s", clr(p.Subdued), len(ended), clr(p.Reset))
	}
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(" "+header+" ", boxW))
	fmt.Fprint(bw, "│\n")

	if len(active) == 0 && len(ended) == 0 {
		boxStreamSep(bw, termW)
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, clr(p.Success))
		fmt.Fprint(bw, colors.PadRight(" No active streams ", boxW))
		fmt.Fprint(bw, clr(p.Reset))
		fmt.Fprint(bw, "│\n")
		boxStreamBottom(bw, termW)
		if !plain {
			fmt.Fprint(bw, colors.EraseDown)
		}
		bw.Flush()
		os.Stdout.Write(buf.Bytes())
		return nil
	}

	// Active streams as standalone grid
	if len(active) > 0 {
		sort.Slice(active, func(i, j int) bool {
			return active[i].StartTime.Before(active[j].StartTime)
		})
		boxStreamBottom(bw, termW)
		var activeStreams []StreamInfo
		for _, record := range active {
			activeStreams = append(activeStreams, record.Stream)
		}
		renderStreamGrid(bw, activeStreams, termW, noColor, p)
	}

	// Ended sessions standalone box
	if len(ended) > 0 {
		boxStreamTop(bw, termW)
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, clr(p.Subdued))
		fmt.Fprint(bw, colors.PadRight(" Recently Ended Sessions ", boxW))
		fmt.Fprint(bw, clr(p.Reset))
		fmt.Fprint(bw, "│\n")

		for _, record := range ended {
			displayEndedStreamToBox(bw, record, boxW, noColor, p)
		}
		boxStreamBottom(bw, termW)
	}

	// Summary standalone box
	if len(active) > 0 {
		boxStreamTop(bw, termW)
		var activeStreams []StreamInfo
		for _, record := range active {
			activeStreams = append(activeStreams, record.Stream)
		}
		displayStreamSummaryToBox(bw, activeStreams, boxW, noColor, p)
		boxStreamBottom(bw, termW)
	}

	if !plain {
		fmt.Fprint(bw, colors.EraseDown)
	}
	bw.Flush()
	os.Stdout.Write(buf.Bytes())
	return nil
}

func displayStreamToBox(bw *bufio.Writer, stream StreamInfo, boxW int, noColor bool, p *colors.Palette) {
	clr := colors.ClrFunc(noColor)

	serverColor := p.ServerJellyfin
	if stream.Server == "plex" {
		serverColor = p.ServerPlex
	}

	// Server + User line
	line := fmt.Sprintf(" %s%s%s %sUser%s: %s%s%s",
		clr(serverColor), strings.ToUpper(stream.Server), clr(p.Reset),
		clr(p.Bold), clr(p.Reset),
		clr(p.ServerPlex), stream.User, clr(p.Reset))
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(line, boxW))
	fmt.Fprint(bw, "│\n")

	// Show/Title line
	if stream.Type == "episode" && stream.Show != "" {
		line := fmt.Sprintf(" %sShow%s: %s", clr(p.Bold), clr(p.Reset), stream.Show)
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
		if stream.Season != "" || stream.Episode != "" {
			epStr := ""
			if stream.Season != "" {
				epStr += fmt.Sprintf("S%02s", stream.Season)
			}
			if stream.Episode != "" {
				epStr += fmt.Sprintf("E%02s", stream.Episode)
			}
			line := fmt.Sprintf("  %s - %s", epStr, stream.Title)
			fmt.Fprint(bw, "│")
			fmt.Fprint(bw, colors.PadRight(line, boxW))
			fmt.Fprint(bw, "│\n")
		}
	} else {
		line := fmt.Sprintf(" %sTitle%s: %s", clr(p.Bold), clr(p.Reset), stream.Title)
		if stream.Year != "" {
			line += fmt.Sprintf(" %s(%s)%s", clr(p.Accent), stream.Year, clr(p.Reset))
		}
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	}

	// Client line
	if stream.Client != "" {
		var clientLine string
		if stream.Device != "" {
			clientLine = fmt.Sprintf(" %sClient%s: %s (%s)", clr(p.Bold), clr(p.Reset), stream.Client, stream.Device)
		} else {
			clientLine = fmt.Sprintf(" %sClient%s: %s", clr(p.Bold), clr(p.Reset), stream.Client)
		}
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(clientLine, boxW))
		fmt.Fprint(bw, "│\n")
	}

	// Status line
	statusColor := p.Success
	if stream.Transcoding {
		statusColor = p.Error
	}
	statusText := stream.Status
	if stream.IsPaused {
		statusText += " (Paused)"
	}
	line = fmt.Sprintf(" %sStatus%s: %s%s%s", clr(p.Bold), clr(p.Reset), clr(statusColor), statusText, clr(p.Reset))
	if stream.Transcoding {
		line += " !"
	}
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(line, boxW))
	fmt.Fprint(bw, "│\n")

	// Bandwidth line
	if stream.Bandwidth > 0 {
		line := fmt.Sprintf(" %sBandwidth%s: %s%.2f Mbps%s", clr(p.Bold), clr(p.Reset), clr(p.Bandwidth), stream.Bandwidth, clr(p.Reset))
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	}

	// Quality line
	if stream.Resolution != "" || stream.VideoCodec != "" {
		line := fmt.Sprintf(" %sQuality%s: ", clr(p.Bold), clr(p.Reset))
		if stream.Resolution != "" {
			line += fmt.Sprintf("%s ", stream.Resolution)
		}
		if stream.VideoCodec != "" {
			line += fmt.Sprintf("(%s", stream.VideoCodec)
			if stream.AudioCodec != "" {
				line += fmt.Sprintf("/%s", stream.AudioCodec)
			}
			line += ")"
		}
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	}

	// Progress bar
	if stream.Progress > 0 {
		barW := boxW - 18
		if barW > 30 {
			barW = 30
		}
		if barW < 10 {
			barW = 10
		}
		bar := renderProgressBar(stream.Progress, barW)
		line := fmt.Sprintf(" %sPlayback%s: %s %s%5.1f%%%s",
			clr(p.Bold), clr(p.Reset),
			bar,
			clr(p.Accent), stream.Progress, clr(p.Reset))
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	}
}

// streamContentLines returns the padded interior content of a stream box
// (without border characters), one element per line.
func streamContentLines(stream StreamInfo, boxW int, noColor bool, p *colors.Palette) []string {
	clr := colors.ClrFunc(noColor)

	trunc := func(s string, maxW int) string {
		return runewidth.Truncate(s, maxW, "…")
	}

	var lines []string

	serverColor := p.ServerJellyfin
	if stream.Server == "plex" {
		serverColor = p.ServerPlex
	}

	userW := boxW - streamUserWidth
	user := trunc(stream.User, userW)
	line := fmt.Sprintf(" %s%s%s %sUser%s: %s%s%s",
		clr(serverColor), strings.ToUpper(stream.Server), clr(p.Reset),
		clr(p.Bold), clr(p.Reset),
		clr(p.ServerPlex), user, clr(p.Reset))
	lines = append(lines, colors.PadRight(line, boxW))

	if stream.Type == "episode" && stream.Show != "" {
		showW := boxW - streamShowWidth
		show := trunc(stream.Show, showW)
		line := fmt.Sprintf(" %sShow%s: %s", clr(p.Bold), clr(p.Reset), show)
		lines = append(lines, colors.PadRight(line, boxW))
		if stream.Season != "" || stream.Episode != "" {
			epStr := ""
			if stream.Season != "" {
				epStr += fmt.Sprintf("S%02s", stream.Season)
			}
			if stream.Episode != "" {
				epStr += fmt.Sprintf("E%02s", stream.Episode)
			}
			line := fmt.Sprintf(" %sEpisode%s: %s", clr(p.Bold), clr(p.Reset), epStr)
			lines = append(lines, colors.PadRight(line, boxW))
		}
	} else {
		titleW := boxW - streamTitleWidth
		title := trunc(stream.Title, titleW)
		line := fmt.Sprintf(" %sTitle%s: %s", clr(p.Bold), clr(p.Reset), title)
		if stream.Year != "" {
			line += fmt.Sprintf(" %s(%s)%s", clr(p.Accent), stream.Year, clr(p.Reset))
		}
		lines = append(lines, colors.PadRight(line, boxW))
	}

	if stream.Client != "" {
		var clientLine string
		if stream.Device != "" && stream.Device != stream.Client {
			availW := boxW - streamClientWidth
			devW := runewidth.StringWidth(stream.Device)
			cliW := availW - devW
			if cliW < 4 {
				cliW = availW * 2 / 5
				if cliW < 4 {
					cliW = 4
				}
				devW = availW - cliW
			}
			client := trunc(stream.Client, cliW)
			dev := trunc(stream.Device, devW)
			clientLine = fmt.Sprintf(" %sClient%s: %s (%s)", clr(p.Bold), clr(p.Reset), client, dev)
		} else {
			client := trunc(stream.Client, boxW-11)
			clientLine = fmt.Sprintf(" %sClient%s: %s", clr(p.Bold), clr(p.Reset), client)
		}
		lines = append(lines, colors.PadRight(clientLine, boxW))
	}

	statusColor := p.Success
	if stream.Transcoding {
		statusColor = p.Error
	}
	statusText := stream.Status
	if stream.IsPaused {
		statusText += " (Paused)"
	}
	line = fmt.Sprintf(" %sStatus%s: %s%s%s", clr(p.Bold), clr(p.Reset), clr(statusColor), statusText, clr(p.Reset))
	if stream.Transcoding {
		line += " !"
	}
	lines = append(lines, colors.PadRight(line, boxW))

	if stream.Bandwidth > 0 {
		line := fmt.Sprintf(" %sBandwidth%s: %s%.2f Mbps%s", clr(p.Bold), clr(p.Reset), clr(p.Bandwidth), stream.Bandwidth, clr(p.Reset))
		lines = append(lines, colors.PadRight(line, boxW))
	}

	if stream.Resolution != "" || stream.VideoCodec != "" {
		line := fmt.Sprintf(" %sQuality%s: ", clr(p.Bold), clr(p.Reset))
		if stream.Resolution != "" {
			line += fmt.Sprintf("%s ", stream.Resolution)
		}
		if stream.VideoCodec != "" {
			line += fmt.Sprintf("(%s", stream.VideoCodec)
			if stream.AudioCodec != "" {
				line += fmt.Sprintf("/%s", stream.AudioCodec)
			}
			line += ")"
		}
		lines = append(lines, colors.PadRight(line, boxW))
	}

	if stream.Progress > 0 {
		barW := boxW - 18
		if barW > 30 {
			barW = 30
		}
		if barW < 10 {
			barW = 10
		}
		bar := renderProgressBar(stream.Progress, barW)
		line := fmt.Sprintf(" %sPlayback%s: %s %s%5.1f%%%s",
			clr(p.Bold), clr(p.Reset),
			bar,
			clr(p.Accent), stream.Progress, clr(p.Reset))
		lines = append(lines, colors.PadRight(line, boxW))
	}

	return lines
}

// renderStreamGrid renders a set of stream boxes in a side-by-side grid that
// wraps to new rows when the terminal width is exhausted.
func renderStreamGrid(bw *bufio.Writer, streams []StreamInfo, termW int, noColor bool, p *colors.Palette) {
	if len(streams) == 0 {
		return
	}

	const prefBoxInnerW = 48
	boxFullW := prefBoxInnerW + 2
	numCols := termW / boxFullW
	if numCols < 1 {
		numCols = 1
	}
	if numCols > len(streams) {
		numCols = len(streams)
	}

	innerW := (termW - 1 - (numCols - 1)) / numCols
	if innerW > prefBoxInnerW {
		innerW = prefBoxInnerW
	}
	if innerW < 20 {
		innerW = 20
	}
	innerW-- // account for border chars

	type box struct {
		lines []string
	}
	boxes := make([]box, len(streams))
	maxLines := 0
	for i, s := range streams {
		l := streamContentLines(s, innerW, noColor, p)
		boxes[i] = box{lines: l}
		if len(l) > maxLines {
			maxLines = len(l)
		}
	}

	for i := 0; i < len(boxes); i += numCols {
		end := i + numCols
		if end > len(boxes) {
			end = len(boxes)
		}
		row := boxes[i:end]

		for range row {
			fmt.Fprint(bw, "┌"+strings.Repeat("─", innerW)+"┐")
		}
		fmt.Fprintln(bw)

		for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
			for _, b := range row {
				fmt.Fprint(bw, "│")
				if lineIdx < len(b.lines) {
					fmt.Fprint(bw, b.lines[lineIdx])
				} else {
					fmt.Fprint(bw, strings.Repeat(" ", innerW))
				}
				fmt.Fprint(bw, "│")
			}
			fmt.Fprintln(bw)
		}

		for range row {
			fmt.Fprint(bw, "└"+strings.Repeat("─", innerW)+"┘")
		}
		fmt.Fprintln(bw)
	}
}

func displayEndedStreamToBox(bw *bufio.Writer, record SessionRecord, boxW int, noColor bool, p *colors.Palette) {
	clr := colors.ClrFunc(noColor)
	stream := record.Stream

	endedStr := fmt.Sprintf("%s[ENDED %s]%s", clr(p.Subdued), formatTimeSince(*record.EndTime), clr(p.Reset))
	line := fmt.Sprintf(" %s%s%s %sUser%s: %s %s",
		clr(p.Subdued), strings.ToUpper(stream.Server), clr(p.Reset),
		clr(p.Bold), clr(p.Reset), stream.User, endedStr)
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(line, boxW))
	fmt.Fprint(bw, "│\n")

	if stream.Type == "episode" && stream.Show != "" {
		line := fmt.Sprintf(" %sShow%s: %s", clr(p.Subdued), clr(p.Reset), stream.Show)
		if stream.Season != "" && stream.Episode != "" {
			line += fmt.Sprintf(" S%sE%s", stream.Season, stream.Episode)
		}
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	} else {
		line := fmt.Sprintf(" %sTitle%s: %s", clr(p.Subdued), clr(p.Reset), stream.Title)
		if stream.Year != "" {
			line += fmt.Sprintf(" (%s)", stream.Year)
		}
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	}

	line = fmt.Sprintf(" %sClient%s: %s", clr(p.Subdued), clr(p.Reset), stream.Client)
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(line, boxW))
	fmt.Fprint(bw, "│\n")

	if record.EndTime != nil {
		duration := record.EndTime.Sub(record.StartTime)
		line = fmt.Sprintf(" %sDuration%s: %s", clr(p.Subdued), clr(p.Reset), formatDuration(duration))
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, colors.PadRight(line, boxW))
		fmt.Fprint(bw, "│\n")
	}
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds > 0 {
			return fmt.Sprintf("%dm %ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, minutes)
}

func displayStreamSummaryToBox(bw *bufio.Writer, streams []StreamInfo, boxW int, noColor bool, p *colors.Palette) {
	clr := colors.ClrFunc(noColor)

	transcodeCount := 0
	totalBandwidth := 0.0
	for _, stream := range streams {
		if stream.Transcoding {
			transcodeCount++
		}
		totalBandwidth += stream.Bandwidth
	}

	line := fmt.Sprintf(" %sTotal Streams%s: %d    %sTranscoding%s: %d    %sBandwidth%s: %.2f Mbps",
		clr(p.Bold), clr(p.Reset), len(streams),
		clr(p.Bold), clr(p.Reset), transcodeCount,
		clr(p.Bold), clr(p.Reset), totalBandwidth)
	fmt.Fprint(bw, "│")
	fmt.Fprint(bw, colors.PadRight(line, boxW))
	fmt.Fprint(bw, "│\n")
}
