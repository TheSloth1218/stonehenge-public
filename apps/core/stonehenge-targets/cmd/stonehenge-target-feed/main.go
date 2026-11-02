package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"stonehenge-targets/internal/client"
	"stonehenge-targets/internal/config"
	"stonehenge-targets/internal/generator"
	"stonehenge-targets/internal/runner"
	"stonehenge-targets/internal/server"
)

func main() {
	cfg, err := config.Load()
	logOutput := io.Writer(os.Stdout)
	var logFile *os.File
	if err == nil && cfg.LogPath != "" {
		logFile, err = os.OpenFile(cfg.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("log file open failed", "path", cfg.LogPath, "error", err)
			os.Exit(1)
		}
		defer logFile.Close()
		logOutput = io.MultiWriter(os.Stdout, logFile)
	}
	logger := slog.New(slog.NewJSONHandler(logOutput, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err != nil {
		logger.Error("config load failed", "error", err)
		os.Exit(1)
	}
	logger.Info("target feed starting", "mode", cfg.TargetMode, "endpoint", config.RedactEndpoint(cfg.TargetEndpoint), "source_id", cfg.TargetSourceID)

	tracker := server.NewTracker(cfg.TargetMode, cfg.TargetEndpoint, cfg.TargetSourceID, time.Now().UTC())
	statusServer := server.New(cfg.FeedListenAddress, tracker, logger)
	serverErrs := statusServer.Start()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if cfg.RunDuration > 0 {
		runCtx, runCancel := context.WithTimeout(ctx, cfg.RunDuration)
		defer runCancel()
		ctx = runCtx
	}

	gen := generator.New(cfg.TargetSourceID, cfg.TargetSeed)
	targetClient := client.NewWithBackoff(cfg.TargetEndpoint, cfg.TargetAuthToken, cfg.TargetRequestTimeout, cfg.TargetMaxRetries, cfg.TargetRetryDelay, cfg.TargetRetryMaxDelay, cfg.TargetRetryJitter, logger)
	run := runner.New(cfg, gen, targetClient, tracker, logger)
	runErrs := make(chan error, 1)
	go func() {
		runErrs <- run.Run(ctx)
	}()

	exitCode := 0
	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-runErrs:
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("runner failed", "error", err)
			exitCode = 1
		}
		cancel()
	case err := <-serverErrs:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("status server failed", "error", err)
			exitCode = 1
		}
		cancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.GracefulShutdownTimeout)
	defer shutdownCancel()
	if err := statusServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("status server shutdown failed", "error", err)
		exitCode = 1
	}
	select {
	case err := <-runErrs:
		if err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("runner shutdown failed", "error", err)
			exitCode = 1
		}
	case <-time.After(5 * time.Second):
		logger.Warn("runner shutdown timed out")
		exitCode = 1
	default:
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
