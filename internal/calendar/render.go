package calendar

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
)

const (
	minColWidth = 20
	maxColumns  = 3
	maxPerShow  = 2
)

type resizeEvent struct {
	width int
}

type dataEvent struct {
	items       []CalendarItem
	queueIssues []QueueIssue
	err         error
}

type dayGroup struct {
	date  time.Time
	items []CalendarItem
}

func resizeAgent(ctx context.Context, ch chan<- resizeEvent) {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err == nil && w > 0 {
		select {
		case ch <- resizeEvent{width: w}:
		case <-ctx.Done():
			return
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-sigCh:
			w, _, err := term.GetSize(int(os.Stdout.Fd()))
			if err == nil && w > 0 {
				select {
				case ch <- resizeEvent{width: w}:
				case <-ctx.Done():
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func dataAgent(ctx context.Context, cfg ToolConfig, ch chan<- dataEvent) {
	for {
		items, queueIssues, err := aggregateCalendar(ctx, cfg)
		select {
		case ch <- dataEvent{items: items, queueIssues: queueIssues, err: err}:
		case <-ctx.Done():
			return
		}
		if !cfg.Watch {
			return
		}
		if err != nil {
			select {
			case <-ctx.Done():
				return
			case <-time.After(30 * time.Second):
			}
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(cfg.WatchSeconds) * time.Second):
		}
	}
}

func runWithSubagents(ctx context.Context, cfg ToolConfig, p *colors.Palette) error {
	resizeCh := make(chan resizeEvent, 1)
	dataCh := make(chan dataEvent, 1)

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go resizeAgent(ctx, resizeCh)
	go dataAgent(ctx, cfg, dataCh)

	var (
		termWidth int = 80
		items     []CalendarItem
		issues    []QueueIssue
	)

	for items == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case re := <-resizeCh:
			termWidth = re.width
		case de := <-dataCh:
			if de.err != nil {
				if !cfg.Watch {
					return de.err
				}
				fmt.Fprintf(os.Stderr, "WARNING: %v (retrying)\n", de.err)
				continue
			}
			items = de.items
			issues = de.queueIssues
		}
	}

	fmt.Print(colors.ClearScreen + colors.HomeCursor)
	renderCalendar(cfg, items, issues, termWidth, p)

	if !cfg.Watch {
		return nil
	}

	var lastHash string

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case re := <-resizeCh:
			if re.width > 0 && re.width != termWidth {
				termWidth = re.width
				renderCalendar(cfg, items, issues, termWidth, p)
			}
		case de := <-dataCh:
			if de.err != nil {
				fmt.Fprintf(os.Stderr, "WARNING: %v\n", de.err)
				continue
			}
			items = de.items
			issues = de.queueIssues
			newHash := computeCalendarHash(items, issues)
			if newHash == lastHash {
				continue
			}
			lastHash = newHash
			renderCalendar(cfg, items, issues, termWidth, p)
		}
	}
}

func computeCalendarHash(items []CalendarItem, queueIssues []QueueIssue) string {
	data, _ := json.Marshal(struct {
		Items       []CalendarItem
		QueueIssues []QueueIssue
	}{items, queueIssues})
	h := sha256.Sum256(data)
	return string(h[:])
}

func renderCalendar(cfg ToolConfig, items []CalendarItem, queueIssues []QueueIssue, termWidth int, p *colors.Palette) {
	clr := colors.ClrFunc(cfg.NoColor)

	now := time.Now()
	items = applyFilters(items, cfg)

	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	fmt.Fprint(bw, colors.ClearScreen+colors.HomeCursor)

	if !cfg.Quiet && len(queueIssues) > 0 {
		totalIssues := 0
		for _, issue := range queueIssues {
			totalIssues += issue.Count
		}
		fmt.Fprintf(bw, "%s!  WARNING: %d items require manual intervention%s\n",
			clr(p.QueueWarning), totalIssues, clr(p.Reset))
		for _, issue := range queueIssues {
			fmt.Fprintf(bw, "%s→ %s: %s%s\n",
				clr(p.QueueLink), issue.ServiceName, issue.URL, clr(p.Reset))
		}
		fmt.Fprintln(bw)
	}

	if len(items) == 0 {
		fmt.Fprintf(bw, "%sNo items match the current filters.%s\n",
			clr(p.NoReleases), clr(p.Reset))
		fmt.Fprint(bw, colors.EraseDown)
		bw.Flush()
		os.Stdout.Write(buf.Bytes())
		return
	}

	start, _ := calculateDateRange(cfg.Days, cfg.DaysPast)
	totalDays := cfg.Days + cfg.DaysPast

	dayMap := make(map[string]*dayGroup, totalDays)
	dayOrder := make([]*dayGroup, 0, totalDays)
	for d := 0; d < totalDays; d++ {
		day := start.AddDate(0, 0, d)
		dg := &dayGroup{date: day}
		dayMap[day.Format("2006-01-02")] = dg
		dayOrder = append(dayOrder, dg)
	}
	for _, item := range items {
		if dg, ok := dayMap[item.AirTime.Format("2006-01-02")]; ok {
			dg.items = append(dg.items, item)
		}
	}

	numCols := maxColumns
	if termWidth < 100 {
		numCols = 2
	}
	if termWidth < 60 {
		numCols = 1
	}
	if numCols > totalDays {
		numCols = totalDays
	}
	if numCols < 1 {
		numCols = 1
	}

	availWidth := termWidth - (numCols + 1)
	colWidth := availWidth / numCols
	if colWidth < minColWidth {
		colWidth = minColWidth
	}

	title := "TV & Anime Schedule"
	if cfg.NoBanner {
		title = ""
	}

	for sectionStart := 0; sectionStart < totalDays; sectionStart += numCols {
		sectionEnd := sectionStart + numCols
		if sectionEnd > totalDays {
			sectionEnd = totalDays
		}
		section := dayOrder[sectionStart:sectionEnd]
		sectionCols := sectionEnd - sectionStart

		w := colWidth
		if sectionCols < numCols {
			aw := termWidth - (sectionCols + 1)
			w = aw / sectionCols
			if w < minColWidth {
				w = minColWidth
			}
		}

		renderBox(bw, cfg, clr, now, section, w, termWidth, title, cfg.NoBanner, p)
		title = ""
	}

	renderSummary(bw, items, now, clr, p)

	fmt.Fprint(bw, colors.EraseDown)
	bw.Flush()
	os.Stdout.Write(buf.Bytes())
}

func boxExtra(totalWidth, colWidth, numCols int) int {
	return totalWidth - (numCols*colWidth + numCols + 1)
}

func renderBox(bw *bufio.Writer, cfg ToolConfig, clr func(string) string, now time.Time, days []*dayGroup, colWidth, totalWidth int, title string, _ bool, p *colors.Palette) {
	extra := boxExtra(totalWidth, colWidth, len(days))
	numCols := len(days)

	if title != "" {
		prefix := "┌── " + title + " ──"
		rc := colors.VisibleLen(prefix)
		padLen := totalWidth - rc - 1
		if padLen < 0 {
			padLen = 0
		}
		fmt.Fprint(bw, prefix)
		fmt.Fprint(bw, strings.Repeat("─", padLen))
		fmt.Fprint(bw, "┐\n")
	} else {
		boxTop(bw, colWidth, numCols, extra)
	}

	headerRow(bw, clr, days, colWidth, extra, p)

	boxSep(bw, colWidth, numCols, extra)

	dayLines := make([][]string, numCols)
	maxLines := 0
	for i, dg := range days {
		width := colWidth
		if i == numCols-1 {
			width += extra
		}
		lines := buildDayLines(dg, clr, now, width, p)
		dayLines[i] = lines
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}

	for row := 0; row < maxLines; row++ {
		fmt.Fprint(bw, "│")
		for i := 0; i < numCols; i++ {
			width := colWidth
			if i == numCols-1 {
				width += extra
			}
			if row < len(dayLines[i]) {
				fmt.Fprint(bw, dayLines[i][row])
			} else {
				fmt.Fprint(bw, strings.Repeat(" ", width))
			}
			fmt.Fprint(bw, "│")
		}
		fmt.Fprintln(bw)
	}

	boxBottom(bw, colWidth, numCols, extra)
}

func boxTop(bw *bufio.Writer, colWidth, numCols, extra int) {
	fmt.Fprint(bw, "├")
	for i := 0; i < numCols; i++ {
		w := colWidth
		if i == numCols-1 {
			w += extra
		}
		fmt.Fprint(bw, strings.Repeat("─", w))
		if i < numCols-1 {
			fmt.Fprint(bw, "┼")
		}
	}
	fmt.Fprint(bw, "┤\n")
}

func boxSep(bw *bufio.Writer, colWidth, numCols, extra int) {
	boxTop(bw, colWidth, numCols, extra)
}

func boxBottom(bw *bufio.Writer, colWidth, numCols, extra int) {
	fmt.Fprint(bw, "└")
	for i := 0; i < numCols; i++ {
		w := colWidth
		if i == numCols-1 {
			w += extra
		}
		fmt.Fprint(bw, strings.Repeat("─", w))
		if i < numCols-1 {
			fmt.Fprint(bw, "┴")
		}
	}
	fmt.Fprint(bw, "┘\n")
}

func headerRow(bw *bufio.Writer, clr func(string) string, days []*dayGroup, colWidth, extra int, p *colors.Palette) {
	fmt.Fprint(bw, "│")
	for i, dg := range days {
		w := colWidth
		if i == len(days)-1 {
			w += extra
		}
		dateStr := dg.date.Format("Mon 01/02")
		countStr := fmt.Sprintf("(%d Show%s)", len(dg.items), pluralS(len(dg.items)))
		full := fmt.Sprintf(" %s%s%s %s",
			clr(p.DayHeader), dateStr, clr(p.Reset), countStr)
		fmt.Fprint(bw, colors.PadRight(full, w))
		fmt.Fprint(bw, "│")
	}
	fmt.Fprintln(bw)
}

func buildDayLines(dg *dayGroup, clr func(string) string, now time.Time, colWidth int, p *colors.Palette) []string {
	if len(dg.items) == 0 {
		line := fmt.Sprintf(" %sNo releases%s", clr(p.NoReleases), clr(p.Reset))
		return []string{colors.PadRight(line, colWidth)}
	}

	sorted := slices.Clone(dg.items)
	sort.Slice(sorted, func(i, j int) bool {
		if !sorted[i].AirTime.Equal(sorted[j].AirTime) {
			return sorted[i].AirTime.Before(sorted[j].AirTime)
		}
		if sorted[i].Type != sorted[j].Type {
			return sorted[i].Type == "episode"
		}
		if sorted[i].Type == "episode" && sorted[j].Type == "episode" {
			if sorted[i].Season != sorted[j].Season {
				return sorted[i].Season < sorted[j].Season
			}
			return sorted[i].Episode < sorted[j].Episode
		}
		return false
	})

	var lines []string
	i := 0
	for i < len(sorted) {
		item := sorted[i]

		if item.Type == "movie" {
			lines = append(lines, buildMovieLines(item, clr, now, colWidth, p)...)
			i++
			continue
		}

		showStart := i
		showEnd := i + 1
		for showEnd < len(sorted) &&
			sorted[showEnd].Type == "episode" &&
			sorted[showEnd].ShowTitle == item.ShowTitle {
			showEnd++
		}
		count := showEnd - showStart

		if count > maxPerShow {
			for j := showStart; j < showStart+maxPerShow; j++ {
				lines = append(lines, buildEpisodeLines(sorted[j], clr, now, colWidth, p)...)
			}
			extra := fmt.Sprintf("  %s+ %d more%s", clr(p.Overflow), count-maxPerShow, clr(p.Reset))
			lines = append(lines, colors.PadRight(extra, colWidth))
		} else {
			for j := showStart; j < showEnd; j++ {
				lines = append(lines, buildEpisodeLines(sorted[j], clr, now, colWidth, p)...)
			}
		}

		i = showEnd
	}

	return lines
}

func buildEpisodeLines(ep CalendarItem, clr func(string) string, now time.Time, colWidth int, p *colors.Palette) []string {
	c := getStatusColor(ep, now, p)
	timeStr := ep.AirTime.Format("15:04")
	titleMax := colWidth - 7
	title := truncateWithEllipsis(ep.ShowTitle, titleMax)

	line1 := fmt.Sprintf(" %s %s%s%s", timeStr, clr(c), title, clr(p.Reset))
	line1 = colors.PadRight(line1, colWidth)

	epInfo := fmt.Sprintf("[S%02dE%02d]", ep.Season, ep.Episode)
	line2 := fmt.Sprintf("       %s%s%s", clr(c), epInfo, clr(p.Reset))
	line2 = colors.PadRight(line2, colWidth)

	line3 := strings.Repeat(" ", colWidth)

	return []string{line1, line2, line3}
}

func buildMovieLines(m CalendarItem, clr func(string) string, now time.Time, colWidth int, p *colors.Palette) []string {
	c := getStatusColor(m, now, p)
	timeStr := m.AirTime.Format("15:04")
	titleMax := colWidth - 7
	title := truncateWithEllipsis(m.Title, titleMax)

	line1 := fmt.Sprintf(" %s %s%s%s", timeStr, clr(c), title, clr(p.Reset))
	line1 = colors.PadRight(line1, colWidth)

	yearStr := fmt.Sprintf("(%d)", m.Year)
	line2 := fmt.Sprintf("       %s%s%s", clr(c), yearStr, clr(p.Reset))
	line2 = colors.PadRight(line2, colWidth)

	line3 := strings.Repeat(" ", colWidth)

	return []string{line1, line2, line3}
}

func renderSummary(bw *bufio.Writer, items []CalendarItem, now time.Time, clr func(string) string, p *colors.Palette) {
	episodes := 0
	movies := 0
	available := 0
	missing := 0

	for _, item := range items {
		if item.Type == "episode" {
			episodes++
		} else {
			movies++
		}
		if item.HasFile {
			available++
		} else if item.AirTime.Before(now) {
			missing++
		}
	}
	future := len(items) - available - missing

	fmt.Fprintf(bw, "\n%s%s%d items%s (%d episodes, %d movies) — ",
		clr(p.Bold), clr(p.Accent), len(items), clr(p.Reset), episodes, movies)
	fmt.Fprintf(bw, "%s%d available%s, %s%d missing%s, %d upcoming\n",
		clr(p.Success), available, clr(p.Reset),
		clr(p.Error), missing, clr(p.Reset),
		future)
}

func truncateWithEllipsis(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-1]) + "…"
}

func pluralS(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
