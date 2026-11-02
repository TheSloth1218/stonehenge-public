package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/guns"
	"stonehenge-middleware/internal/history"
	"stonehenge-middleware/internal/model"
	"stonehenge-middleware/internal/plc"
	"stonehenge-middleware/internal/safety"
	"stonehenge-middleware/internal/sim"
	"stonehenge-middleware/internal/store"
)

func TestHTTPServerResourceLimits(t *testing.T) {
	server := newHTTPServer("127.0.0.1:0", http.NotFoundHandler())
	if server.ReadTimeout != 15*time.Second || server.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("read timeouts = %s/%s", server.ReadTimeout, server.ReadHeaderTimeout)
	}
	if server.WriteTimeout != 21*time.Minute || server.IdleTimeout != time.Minute {
		t.Fatalf("write/idle timeouts = %s/%s", server.WriteTimeout, server.IdleTimeout)
	}
	if server.MaxHeaderBytes != 1<<20 {
		t.Fatalf("max header bytes = %d", server.MaxHeaderBytes)
	}
}

func TestApplyStartupModeOverridesAutoEnabledGun(t *testing.T) {
	cfg := config.Defaults()
	cfg.Simulation.GunCount = 1
	cfg.PLC.StartupMode = "manual"
	cfg.Guns[0].PLC = cfg.PLC
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	engine := sim.NewEngine(
		cfg,
		store.NewMemoryStore(cfg.History.MaxEvents),
		plc.NewSimulatedAdapter(),
		safety.NewSimulatedController(cfg.Safety),
		nil,
		history.NewNoOpSink(),
		logger,
	)
	registry, err := guns.Build(gunDefinitions(cfg.Guns), nil, nil, nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyStartupMode(cfg, engine, registry, logger); err != nil {
		t.Fatal(err)
	}

	if mode := engine.Status().Guns[0].Mode; mode != model.ModeManual {
		t.Fatalf("public gun mode = %s, want %s", mode, model.ModeManual)
	}
	instance := registry.List()[0]
	if status := instance.Arbitration.Status(); status.Mode != "manual" {
		t.Fatalf("arbitration mode = %s, want manual", status.Mode)
	}
	if status := instance.Auto.Status(context.Background()); status.Enabled {
		t.Fatal("auto controller remained enabled during manual startup")
	}
}
