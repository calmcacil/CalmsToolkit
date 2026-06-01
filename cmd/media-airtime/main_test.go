package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestMainHelpExitsOnNoArgs(t *testing.T) {
	if os.Getenv("GO_TEST_SUBPROCESS") == "1" {
		os.Args = []string{"media-airtime"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMainHelpExitsOnNoArgs")
	cmd.Env = append(os.Environ(), "GO_TEST_SUBPROCESS=1")
	err := cmd.Run()
	if e, ok := err.(*exec.ExitError); ok && !e.Success() {
		return
	}
	t.Fatal("expected exit 1 for no args")
}
