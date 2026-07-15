package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/calmcacil/CalmsToolkit/internal/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	os.Exit(cli.Execute(ctx, os.Stdin, os.Stdout, os.Stderr, os.Args[1:]))
}
