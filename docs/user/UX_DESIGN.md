# CalmsToolkit Terminal UX Design

## Principles

- **Box-drawn containers** — all tools use Unicode box-drawing characters for structure
- **Terminal-aware sizing** — content scales to terminal width via `golang.org/x/term`
- **Proportional columns** — column widths are fractions of available space, not hardcoded
- **ANSI-safe rendering** — all padding and truncation uses `visibleLen()` to account for ANSI escape codes
- **Consistent color vocabulary** — same status colors across all tools
- **Buffered output** — all rendering writes to `bytes.Buffer` + `bufio.Writer`, flushed to `os.Stdout`

## Box-Drawing Characters

| Char | Use |
|------|-----|
| `─` | Horizontal rule |
| `│` | Vertical rule (column separator, box sides) |
| `┌` | Top-left corner |
| `┐` | Top-right corner |
| `└` | Bottom-left corner |
| `┘` | Bottom-right corner |
| `├` | Left tee (separator meets left edge) |
| `┤` | Right tee (separator meets right edge) |
| `┬` | Top tee (column junction in header) |
| `┴` | Bottom tee (column junction in footer) |
| `┼` | Cross (column junction in separator) |
| `…` | Ellipsis (truncated text) |
| `█` / `░` | Filled/empty progress bar chars |

## Box Construction

### Table-style layout (arr-feed, calendar)

```
┌── Title ──────────────────────────────────────┐
│ Header 1          │ Header 2          │ Head… │
├───────────────────┼───────────────────┼───────┤
│ Data cell         │ Data cell         │ Data… │
│ Data cell         │ Data cell         │ Data… │
└───────────────────┴───────────────────┴───────┘
```

Functions:
- `boxTop(bw, colWidths)` — draws `┌──┬──┐`
- `boxSep(bw, colWidths)` — draws `├──┼──┤`
- `boxBottom(bw, colWidths)` — draws `└──┴──┘`
- Each column's width is computed proportionally; integer remainder is added to the last column to ensure the total exactly matches terminal width.

### Card-style layout (media-streams)

```
┌───────────────────────────────────────┐
│ [PLEX] User: name                     │
│ Show: Title                           │
│ Status: Direct Play                   │
│ Playback: ████████░░░░ 45%            │
├───────────────────────────────────────┤
│ Total Streams: 3    Transcoding: 1    │
└───────────────────────────────────────┘
```

Functions:
- `boxStreamTop(bw, termW)` — draws `┌──┐`
- `boxStreamSep(bw, termW)` — draws `├──┤`
- `boxStreamBottom(bw, termW)` — draws `└──┘`
- Box inner width cap: 120 columns (`maxBoxInnerWidth`)
- Content lines: `"│" + padRight(" " + content + " ", boxW) + "│"`

## Terminal Width Detection

All tools use `golang.org/x/term`:

```go
w, _, err := term.GetSize(int(os.Stdout.Fd()))
if err != nil || w <= 0 {
    w = defaultWidth // 120 for feed, 80 for streams, 80 for calendar
}
```

Width is then clamped per-tool:
- Calendar: minimum column width 20
- Feed: minimum column width 5
- Streams: inner width clamped to [40, 120]

## Column Width Calculation

```go
availWidth := termWidth - totalCols - 1  // minus borders
sumProp := sum of all column proportions
colWidths[i] = availWidth * prop[i] / sumProp
// Minimum enforced per column
// Integer remainder absorbed by distribution across columns
```

### arr-feed column proportions

| Column | Proportion | Notes |
|--------|-----------|-------|
| When | 12 | Relative time |
| Action | 8 | Colored by type |
| Series/Movie | 28 | Truncated with `…` |
| Episode | 8 | `S02E09` format |
| Episode Title | 20 | Truncated with `…` |
| Quality | 12 | |
| Formats | 15 | Comma-joined |
| Subtitles | 10 | Optional, only when `ShowSubtitles` is on |

When the subtitles column is hidden, the space is redistributed proportionally.

## Color Convention

### Action/Event colors (arr-feed)

| Event | Color |
|-------|-------|
| Imported / Bulk Import | `colors.Green` |
| Grabbed | `colors.Cyan` |
| Failed | `colors.Red` |
| Deleted | `colors.Yellow` |
| Ignored | `colors.Gray` |
| Renamed | `colors.Blue` |

### Status colors (streams)

| State | Color |
|-------|-------|
| Direct Play | `colors.Green` |
| Transcoding | `colors.Red` |
| Paused | Appended `(Paused)` to status |
| Ended session | `colors.Gray` for entire block |

### Server label colors (streams)

| Server | Color |
|--------|-------|
| Plex | `colors.Yellow` |
| Jellyfin | `colors.Magenta` |

### Summary/header colors

| Element | Color |
|---------|-------|
| Total events/streams | `colors.Bold` + `colors.Cyan` |
| Available count | `colors.Green` |
| Missing count | `colors.Red` |
| Bandwidth value | `colors.Magenta` |
| Progress percentage | `colors.Cyan` |
| No items / no streams | `colors.Green` |

## ANSI-Safe Helpers

These are shared across rendering code (exact functions duplicated per package for simplicity):

```go
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// visibleLen returns the visible character count, stripping ANSI codes.
func visibleLen(s string) int {
    return utf8.RuneCountInString(ansiRe.ReplaceAllString(s, ""))
}

// padRight pads a string to the given visible width with spaces on the right.
func padRight(s string, width int) string {
    v := visibleLen(s)
    if v >= width {
        return s
    }
    return s + strings.Repeat(" ", width-v)
}

// truncateWithEllipsis truncates a string at maxLen runes, appending … if truncated.
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
```

## Watch Mode Behavior

All tools with watch mode follow the same pattern:

1. Hide cursor on start: `fmt.Print(colors.HideCursor)`
2. On each refresh: `fmt.Print(colors.ClearScreen + colors.HomeCursor)`
3. Re-render full output
4. Show cursor on exit: `defer fmt.Print(colors.ShowCursor)`
5. SIGWINCH handling for terminal resize (calendar only currently)

## Summary Footer Convention

When displaying aggregates, the summary is styled as:

```
Total items: N (N episodes, N movies) — N available, N missing, N upcoming
```

Or for streams:

```
Total Streams: N    Transcoding: N    Bandwidth: N.NN Mbps
```

Each numeric value is wrapped in `clr(colors.Bold)` for emphasis.

## Key Implementation Locations

| Component | File | Key Functions |
|-----------|------|---------------|
| Calendar rendering | `internal/calendar/render.go` | `renderCalendar`, `renderBox`, `boxTop/Sep/Bottom`, `headerRow`, `buildDayLines` |
| Feed rendering | `internal/feed/feed.go` | `renderTable`, `boxTop/Sep/Bottom`, helpers |
| Streams rendering | `internal/streams/streams.go` | `displayTerminalOutput`, `displayStreamToBox`, `renderProgressBar`, `getBoxWidth` |
| Colors | `internal/colors/` | ANSI escape constants |
| Config | `internal/config/config.go` | Shared config types |
