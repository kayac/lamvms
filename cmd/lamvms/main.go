package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"

	"github.com/kayac/lamvms"
)

func main() {
	os.Exit(_main())
}

func _main() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	exitCode, err := lamvms.CLI(ctx, lamvms.ParseCLI)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			slog.Warn("interrupted")
		} else {
			slog.Error("FAILED", "error", err)
		}
	}
	return exitCode
}
