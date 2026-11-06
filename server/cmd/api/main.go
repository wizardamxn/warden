// The API program answers the website. It has no login yet, so it listens on
// 127.0.0.1 only and must not be made public until step 6 (accounts) is built.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"warden/internal/api"
	"warden/internal/core"
	"warden/internal/jobs"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	settings, err := core.Load(".env")
	if err != nil {
		return err
	}
	if err := settings.Require("DATABASE_URL"); err != nil {
		return err
	}
	core.SetupLog(settings.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := core.OpenDB(ctx, settings.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	server := &http.Server{
		Addr:              settings.APIAddr,
		Handler:           api.New(jobs.New(db), settings.WebOrigin),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(slog.Default().Handler(), slog.LevelError),
	}
	failed := make(chan error, 1)
	go func() { failed <- server.ListenAndServe() }()
	slog.Info("api started", "addr", settings.APIAddr, "web_origin", settings.WebOrigin)

	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}
	slog.Info("api stopping")
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdown); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	slog.Info("api stopped")
	return nil
}
