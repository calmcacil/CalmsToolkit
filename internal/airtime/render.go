package airtime

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/calmcacil/CalmsToolkit/internal/colors"
	"golang.org/x/term"
)

func interactiveSelect(ctx context.Context, candidates []scoredMatch, query string, cfg ToolConfig) *scoredMatch {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "Multiple matches (use -json or pipe through TTY for interactive pick):\n")
		for i, sm := range candidates {
			icon := iconForType(sm.Candidate.Type)
			fmt.Fprintf(os.Stderr, "  %d. %s %s (%d) [%s]\n", i+1, icon, sm.Candidate.Title, sm.Candidate.Year, sm.Candidate.Source)
		}
		return &candidates[0]
	}

	p := colors.GetPalette(cfg.Theme)
	clr := colors.ClrFunc(cfg.NoColor)
	termWidth := terminalWidth()
	if termWidth < 40 {
		termWidth = 40
	}

	innerW := termWidth - 2
	if innerW > 80 {
		innerW = 80
	}

	reader := bufio.NewReader(os.Stdin)
	lineCh := make(chan string, 1)
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(strings.ToLower(line))
			select {
			case lineCh <- line:
			case <-ctx.Done():
				return
			}
		}
	}()

	for {
		var buf bytes.Buffer
		bw := bufio.NewWriter(&buf)

		fmt.Fprint(bw, "┌"+strings.Repeat("─", innerW)+"┐\n")

		title := fmt.Sprintf("  candidates for %q", query)
		title = colors.PadRight(title, innerW)
		fmt.Fprint(bw, "│"+title+"│\n")

		fmt.Fprint(bw, "├"+strings.Repeat("─", innerW)+"┤\n")

		for i, sm := range candidates {
			icon := iconForType(sm.Candidate.Type)
			label := fmt.Sprintf("%d. %s %s (%d)", i+1, icon, sm.Candidate.Title, sm.Candidate.Year)
			meta := "[" + sm.Candidate.Source + "]"
			if sm.Candidate.Monitored && sm.Candidate.HasFile {
				meta += " [Has]"
			} else if sm.Candidate.Monitored {
				meta += " [Mon]"
			}

			available := innerW - 3 - colors.VisibleLen(label) - colors.VisibleLen(meta)
			if available < 1 {
				maxLabel := innerW - 3 - 8
				if maxLabel < 4 {
					maxLabel = 4
				}
				label = truncateVis(label, maxLabel)
				meta = truncateVis(meta, 8)
				available = innerW - 3 - colors.VisibleLen(label) - colors.VisibleLen(meta)
				if available < 1 {
					available = 1
				}
			}
			content := fmt.Sprintf("  %s%s%s",
				label,
				strings.Repeat(" ", available),
				clr(p.Subdued)+meta+clr(p.Reset))
			content = colors.PadRight(content, innerW)
			fmt.Fprint(bw, "│"+content+"│\n")
		}

		fmt.Fprint(bw, "├"+strings.Repeat("─", innerW)+"┤\n")

		help := "Select [1..N] (s=search again, q=quit): "
		content := " " + help
		content = colors.PadRight(content, innerW)
		fmt.Fprint(bw, "│"+content+"│\n")

		fmt.Fprint(bw, "└"+strings.Repeat("─", innerW)+"┘\n")

		bw.Flush()
		fmt.Fprint(os.Stdout, "\033[s")
		os.Stdout.Write(buf.Bytes())

		select {
		case line, ok := <-lineCh:
			if !ok {
				return nil
			}
			fmt.Fprint(os.Stdout, "\033[u\033[J")

			if line == "q" {
				fmt.Fprintln(os.Stderr, "Quit.")
				return nil
			}
			if line == "s" {
				fmt.Fprintf(os.Stderr, "Start a new search with: calmstoolkit airtime <new query>\n")
				return nil
			}

			var idx int
			if _, err := fmt.Sscanf(line, "%d", &idx); err == nil && idx >= 1 && idx <= len(candidates) {
				return &candidates[idx-1]
			}
		case <-ctx.Done():
			fmt.Fprint(os.Stdout, "\033[u\033[J")
			return nil
		}
	}
}

func renderCard(info AirtimeInfo, cfg ToolConfig) {
	p := colors.GetPalette(cfg.Theme)
	clr := colors.ClrFunc(cfg.NoColor)
	termWidth := terminalWidth()
	if termWidth < 40 {
		termWidth = 40
	}

	innerW := termWidth - 2
	if innerW > 100 {
		innerW = 100
	}

	var buf bytes.Buffer
	bw := bufio.NewWriter(&buf)

	boxTop := fmt.Sprintf("┌%s┐\n", strings.Repeat("─", innerW))
	fmt.Fprint(bw, boxTop)

	icon := iconForType(info.Type)
	titleLine := fmt.Sprintf("  %s %s (%d) ─ %s ", icon, info.Title, info.Year, info.Source)
	titleLine = colors.PadRight(titleLine, innerW)
	fmt.Fprint(bw, "│"+titleLine+"│\n")

	boxSep := fmt.Sprintf("├%s┤\n", strings.Repeat("─", innerW))
	fmt.Fprint(bw, boxSep)

	now := time.Now()

	addLine := func(key, val string) {
		content := fmt.Sprintf("  %s%s%s %s%s", clr(p.Bold), key, clr(p.Reset), val, clr(p.Reset))
		content = colors.PadRight(content, innerW)
		fmt.Fprint(bw, "│"+content+"│\n")
	}

	addSep := func() {
		content := strings.Repeat(" ", innerW)
		fmt.Fprint(bw, "│"+content+"│\n")
	}

	if info.Type == "series" {
		c := statusColor(info.Status, p)
		addLine("Status:", fmt.Sprintf("%s%s%s", clr(c), info.Status, clr(p.Reset)))

		libStatus := "not monitored"
		if info.EpisodesTotal > 0 {
			diskLabel := "missing"
			if info.EpisodesOnDisk == info.EpisodesTotal {
				diskLabel = "on disk"
			}
			libStatus = fmt.Sprintf("monitored, %d/%d %s", info.EpisodesOnDisk, info.EpisodesTotal, diskLabel)
		} else if info.Monitored {
			libStatus = "monitored"
		}
		addLine("Library:", libStatus)

		if info.TVDBID > 0 {
			addLine("TVDB:", fmt.Sprintf("%d", info.TVDBID))
		}
		addSep()

		seasonLabel := "Current season:"
		if info.Status == "ended" {
			seasonLabel = "Final season:"
		}
		addLine(seasonLabel, fmt.Sprintf("%d (%d episodes)", info.Season, info.EpisodesTotal))

		if info.LastAir != nil {
			rel := formatRelativeDate(now, *info.LastAir)
			addLine("  Last air:", info.LastLabel)
			addLine("           ", rel)
		}

		if info.NextAir != nil {
			rel := formatRelativeDate(now, *info.NextAir)
			addLine("  Next air:", info.NextLabel)
			addLine("           ", rel)
		} else if info.Status == "ended" {
			addLine("  Next air:", clr(p.Subdued)+"—"+clr(p.Reset))
		} else {
			addLine("  Next air:", clr(p.Subdued)+"—"+clr(p.Reset))
		}

		if cfg.FullSeason && len(info.SeasonEpisodes) > 0 {
			fmt.Fprint(bw, boxSep)
			renderFullSeason(bw, info, now, p, innerW)
		}
	} else {
		c := statusColor(info.Status, p)
		addLine("Status:", fmt.Sprintf("%s%s%s", clr(c), info.Status, clr(p.Reset)))

		libStatus := "not monitored"
		if info.Monitored {
			libStatus = "monitored"
		}
		if info.EpisodesTotal > 0 {
			libStatus = fmt.Sprintf("monitored, %d/%d on disk", info.EpisodesOnDisk, info.EpisodesTotal)
		}
		addLine("Library:", libStatus)

		if info.TMDBID > 0 {
			addLine("TMDB:", fmt.Sprintf("%d", info.TMDBID))
		}
		addSep()

		if info.LastAir != nil {
			addLine("  Release:", formatRelativeDate(now, *info.LastAir))
		} else if info.NextAir != nil {
			addLine("  Release:", formatRelativeDate(now, *info.NextAir))
		} else {
			addLine("  Release:", clr(p.Subdued)+"—"+clr(p.Reset))
		}
	}

	fmt.Fprint(bw, "└"+strings.Repeat("─", innerW)+"┘\n")
	bw.Flush()
	os.Stdout.Write(buf.Bytes())
}

func renderFullSeason(bw *bufio.Writer, info AirtimeInfo, now time.Time, p *colors.Palette, innerW int) {
	header := fmt.Sprintf("  Season %d episodes", info.Season)
	header = colors.PadRight(header, innerW)
	fmt.Fprint(bw, "│"+header+"│\n")

	for _, ep := range info.SeasonEpisodes {
		icon := " "
		if ep.HasFile {
			icon = "D"
		}
		rel := formatRelativeDate(now, ep.AirDateUtc)
		line := fmt.Sprintf("  E%02d [%s]  %s \u2014 %s", ep.EpisodeNumber, icon, ep.Title, rel)
		if colors.VisibleLen(line) > innerW {
			line = truncateVis(line, innerW)
		}
		line = colors.PadRight(line, innerW)
		fmt.Fprint(bw, "│"+line+"│\n")
	}
}

func iconForType(t string) string {
	switch t {
	case "series":
		return "[TV]"
	case "movie":
		return "[Film]"
	default:
		return "[?]"
	}
}

func statusColor(status string, p *colors.Palette) string {
	switch status {
	case "ongoing":
		return p.Success
	case "released":
		return p.Success
	case "ended":
		return p.Subdued
	case "announced":
		return p.Info
	default:
		return p.Warning
	}
}

func formatRelativeDate(now, t time.Time) string {
	diff := t.Sub(now)

	if diff >= -30*time.Second && diff <= 30*time.Second {
		return "now"
	}

	if diff > 0 {
		if diff < time.Minute {
			return "in a few seconds"
		}
		if diff < time.Hour {
			m := int(diff.Minutes())
			if m == 1 {
				return "in 1 minute"
			}
			return fmt.Sprintf("in %d minutes", m)
		}
		if diff < 24*time.Hour {
			h := int(diff.Hours())
			if h == 1 {
				return "in 1 hour"
			}
			return fmt.Sprintf("in %d hours", h)
		}
		d := calendarDays(now, t)
		if d <= 0 {
			d = 1
		}
		if d == 1 {
			return "tomorrow"
		}
		return fmt.Sprintf("in %d days", d)
	}

	diff = -diff
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
	if diff < 30*24*time.Hour {
		d := calendarDays(t, now)
		if d <= 0 {
			d = 1
		}
		if d == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", d)
	}

	return t.Format("2006-01-02")
}

func calendarDays(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	left := time.Date(ay, am, ad, 0, 0, 0, 0, a.Location())
	right := time.Date(by, bm, bd, 0, 0, 0, 0, b.Location())
	return int(right.Sub(left).Hours() / 24)
}

func terminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

func truncateVis(s string, maxLen int) string {
	visLen := colors.VisibleLen(s)
	if visLen <= maxLen {
		return s
	}
	if maxLen <= 3 {
		runes := []rune(s)
		if len(runes) > maxLen {
			return string(runes[:maxLen])
		}
		return s
	}
	ellipsis := "..."
	elLen := len(ellipsis)
	for colors.VisibleLen(s[:len(s)-elLen]) > maxLen-elLen {
		s = s[:len(s)-1]
	}
	return s + ellipsis
}
