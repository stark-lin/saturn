// Command server starts the Saturn HTTP service.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stark-lin/go-proj/internal/app"
)

func main() {
	configPath := flag.String("config", "config.json", "path to Saturn JSON config file")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	deps, err := app.LoadDependencies(ctx, *configPath)
	if err != nil {
		slog.Error("failed to load dependencies", "error", err)
		os.Exit(1)
	}

	saturn, err := app.New(ctx, deps)
	if err != nil {
		deps.Logger.Error("failed to initialize app", "error", err)
		os.Exit(1)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- saturn.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := saturn.Shutdown(shutdownCtx); err != nil {
			deps.Logger.Error("failed to shut down cleanly", "error", err)
			os.Exit(1)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			deps.Logger.Error("server stopped with error", "error", err)
			os.Exit(1)
		}
	}
}
