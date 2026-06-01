package buildinfo

import (
	"fmt"
	"io"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}

func Print(w io.Writer) {
	fmt.Fprintln(w, String())
}
