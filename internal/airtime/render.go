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

	reader := bufio.NewReader(os.Stdin)
	for {
		var buf bytes.Buffer
		bw := bufio.NewWriter(&buf)

		title := fmt.Sprintf("candidates for %q", query)
		boxW := termWidth - 2
		if boxW > 80 {
			boxW = 80
		}

		fmt.Fprint(bw, "┌── ")
		fmt.Fprint(bw, clr(p.Accent))
		fmt.Fprint(bw, title)
		fmt.Fprint(bw, clr(p.Reset))
		rem := boxW - len(title) - 4
		if rem < 0 {
			rem = 0
		}
		fmt.Fprint(bw, strings.Repeat(" ", rem))
		fmt.Fprint(bw, "┐\n")

		for i, sm := range candidates {
			icon := iconForType(sm.Candidate.Type)
			label := fmt.Sprintf("  %d.  %s %s (%d)", i+1, icon, sm.Candidate.Title, sm.Candidate.Year)
			meta := fmt.Sprintf("[%s]", sm.Candidate.Source)
			if sm.Candidate.Monitored && sm.Candidate.HasFile {
				meta += " [Has]"
			} else if sm.Candidate.Monitored {
				meta += " [Mon]"
			}
			fmt.Fprint(bw, "│")
			fmt.Fprint(bw, label)
			remaining := boxW - len(label) - len(meta) - 1
			if remaining < 1 {
				remaining = 1
			}
			fmt.Fprint(bw, strings.Repeat(" ", remaining))
			fmt.Fprint(bw, clr(p.Subdued))
			fmt.Fprint(bw, meta)
			fmt.Fprint(bw, clr(p.Reset))
			fmt.Fprint(bw, "│\n")
		}

		fmt.Fprint(bw, "├")
		fmt.Fprint(bw, strings.Repeat("─", boxW))
		fmt.Fprint(bw, "┤\n")

		help := "Select [1..N] (s=search again, q=quit): "
		fmt.Fprint(bw, "│ ")
		fmt.Fprint(bw, help)
		pad := boxW - len(help) - 1
		if pad > 0 {
			fmt.Fprint(bw, strings.Repeat(" ", pad))
		}
		fmt.Fprint(bw, "│\n")

		fmt.Fprint(bw, "└")
		fmt.Fprint(bw, strings.Repeat("─", boxW))
		fmt.Fprint(bw, "┘\n")

		bw.Flush()
		fmt.Fprint(os.Stdout, "\033[s") // save cursor
		os.Stdout.Write(buf.Bytes())

		line, err := reader.ReadString('\n')
		if err != nil {
			return &candidates[0]
		}
		line = strings.TrimSpace(strings.ToLower(line))

		fmt.Fprint(os.Stdout, "\033[u\033[J") // restore + clear

		if line == "q" {
			fmt.Fprintln(os.Stderr, "Quit.")
			return nil
		}
		if line == "s" {
			fmt.Fprintf(os.Stderr, "Start a new search with: media-airtime <new query>\n")
			return nil
		}

		var idx int
		if _, err := fmt.Sscanf(line, "%d", &idx); err == nil && idx >= 1 && idx <= len(candidates) {
			return &candidates[idx-1]
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

	boxW := termWidth - 2
	if boxW > 100 {
		boxW = 100
	}

	icon := iconForType(info.Type)
	title := fmt.Sprintf(" %s %s (%d) ─ %s ", icon, info.Title, info.Year, info.Source)
	topRem := boxW - len(title)
	if topRem < 1 {
		topRem = 1
	}

	var bodyBuf bytes.Buffer
	bw := bufio.NewWriter(&bodyBuf)

	fmt.Fprint(bw, "┌──")
	fmt.Fprint(bw, title)
	fmt.Fprint(bw, strings.Repeat("─", topRem))
	fmt.Fprint(bw, "┐\n")

	now := time.Now()

	addLine := func(key, val string) {
		line := fmt.Sprintf("│  %s%s%s %s%s", clr(p.Bold), key, clr(p.Reset), val, clr(p.Reset))
		rem := boxW - len("│ ") - len(key) - len(" ") - colors.VisibleLen(val)
		if rem < 1 {
			rem = 1
		}
		line += strings.Repeat(" ", rem-1)
		line += "│\n"
		fmt.Fprint(bw, line)
	}

	addSep := func() {
		fmt.Fprint(bw, "│")
		fmt.Fprint(bw, strings.Repeat(" ", boxW-1))
		fmt.Fprint(bw, "│\n")
	}

	if info.Type == "series" {
		statusColor := statusColor(info.Status, p)
		addLine("Status:", fmt.Sprintf("%s%s%s", clr(statusColor), info.Status, clr(p.Reset)))

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
			lastStr := fmt.Sprintf("%s", info.LastLabel)
			if len(lastStr) > boxW-20 {
				lastStr = truncate(lastStr, boxW-23)
			}
			addLine("  Last air:", fmt.Sprintf("%s", lastStr))
			addLine("           ", rel)
		}

		if info.NextAir != nil {
			rel := formatRelativeDate(now, *info.NextAir)
			nextStr := fmt.Sprintf("%s", info.NextLabel)
			if len(nextStr) > boxW-20 {
				nextStr = truncate(nextStr, boxW-23)
			}
			addLine("  Next air:", fmt.Sprintf("%s", nextStr))
			addLine("           ", rel)
		} else if info.Status == "ended" {
			addLine("  Next air:", clr(p.Subdued)+"— (series has ended)"+clr(p.Reset))
		} else {
			addLine("  Next air:", clr(p.Subdued)+"— (TBA)"+clr(p.Reset))
		}
	} else {
		statusColor := statusColor(info.Status, p)
		addLine("Status:", fmt.Sprintf("%s%s%s", clr(statusColor), info.Status, clr(p.Reset)))

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
			rel := formatRelativeDate(now, *info.LastAir)
			addLine("  Release:", rel)
		} else if info.NextAir != nil {
			rel := formatRelativeDate(now, *info.NextAir)
			addLine("  Release:", rel)
		} else {
			addLine("  Release:", clr(p.Subdued)+"— (TBA)"+clr(p.Reset))
		}
	}

	fmt.Fprint(bw, "└")
	fmt.Fprint(bw, strings.Repeat("─", boxW))
	fmt.Fprint(bw, "┘\n")

	bw.Flush()
	os.Stdout.Write(bodyBuf.Bytes())
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
		d := int(diff.Hours() / 24)
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
	if diff < 7*24*time.Hour {
		d := int(diff.Hours() / 24)
		if d == 1 {
			return "yesterday"
		}
		return fmt.Sprintf("%d days ago", d)
	}

	return t.Local().Format("2006-01-02 15:04")
}

func terminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
