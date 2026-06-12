package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/6space7/porter/internal/api"
	"github.com/6space7/porter/internal/config"
)

func main() {
	cfg := config.Load()

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	slog.Info("starting porter", "addr", cfg.HTTPAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
