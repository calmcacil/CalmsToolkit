package anisearch

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// KeyEvent represents a single key press from the terminal.
type KeyEvent int

const (
	KeyUnknown KeyEvent = iota
	KeyUp
	KeyDown
	KeyEnter
	KeyEsc
	KeyQuit  // q or Q
	KeyNext  // n or N (next page)
	KeyPrev  // p or P (previous page)
	KeyBack  // b or B (back to search results)
	KeyCtrlC // Ctrl+C
	KeyDigit // 0-9 (use DigitValue())
)

// readKey reads a single key press from stdin in raw mode.
// The caller MUST have placed stdin into raw mode via term.MakeRaw before
// calling this function.
func readKey() KeyEvent {
	buf := make([]byte, 3)
	n, err := os.Stdin.Read(buf)
	if err != nil || n == 0 {
		return KeyUnknown
	}

	switch buf[0] {
	case 0x1b: // ESC or arrow key
		if n >= 3 && buf[1] == '[' {
			switch buf[2] {
			case 'A':
				return KeyUp
			case 'B':
				return KeyDown
			}
		}
		return KeyEsc

	case 0x0d, 0x0a: // CR or LF
		return KeyEnter

	case 0x03: // Ctrl+C
		return KeyCtrlC

	case 'q', 'Q':
		return KeyQuit
	case 'n', 'N':
		return KeyNext
	case 'p', 'P':
		return KeyPrev
	case 'b', 'B':
		return KeyBack

	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return KeyDigit

	default:
		return KeyUnknown
	}
}

// runInRawTerminal executes f while stdin is in raw terminal mode.
// It restores the terminal state before returning.
func runInRawTerminal(f func(raw bool)) {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		// Not a terminal; just run f directly.
		f(false)
		return
	}

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Cannot set raw mode; run f directly anyway.
		f(false)
		return
	}
	defer term.Restore(fd, oldState) //nolint:errcheck

	f(true)
}

// PromptForQuery reads a search query from stdin.
func PromptForQuery() (string, error) {
	fmt.Fprint(os.Stderr, "Search: ")
	reader := bufio.NewReader(os.Stdin)
	q, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(q, "\r\n"), nil
}
