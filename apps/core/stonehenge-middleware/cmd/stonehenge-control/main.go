package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"stonehenge-middleware/internal/api"
	"stonehenge-middleware/internal/arbitration"
	"stonehenge-middleware/internal/autocontrol"
	"stonehenge-middleware/internal/config"
	"stonehenge-middleware/internal/dispatcher"
	"stonehenge-middleware/internal/fragments"
	"stonehenge-middleware/internal/guns"
	"stonehenge-middleware/internal/history"
	"stonehenge-middleware/internal/maintenance"
	"stonehenge-middleware/internal/model"
	"stonehenge-middleware/internal/plc"
	"stonehenge-middleware/internal/safety"
	"stonehenge-middleware/internal/sim"
	"stonehenge-middleware/internal/store"
	"stonehenge-middleware/internal/targetfeed"
	"stonehenge-middleware/internal/telemetry"
)

type autoSafetyProvider struct {
	service *safety.Service
}

func (provider autoSafetyProvider) CanFire(gunID int) autocontrol.SafetyDecision {
	decision := provider.service.CanFire(gunID)
	return autocontrol.SafetyDecision{Allowed: decision.Allowed, Reason: decision.Reason}
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("middleware stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(os.Getenv("STONEHENGE_CONFIG"))
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	historySink, err := history.NewSink(cfg.History, logger)
	if err != nil {
		return fmt.Errorf("create history sink: %w", err)
	}
	defer historySink.Close()

	fragmentSource, err := fragments.NewSource(cfg)
	if err != nil {
		return fmt.Errorf("create fragment source: %w", err)
	}

	state := store.NewMemoryStore(cfg.Simulation.MaxEvents)
	engine := sim.NewEngine(
		cfg,
		state,
		plc.NewSimulatedAdapter(),
		safety.NewSimulatedController(cfg.Safety),
		fragmentSource,
		historySink,
		logger,
	)
	if cfg.Simulation.AutoStart {
		engine.Start()
		defer engine.Stop()
	}

	commandAuditor, _ := historySink.(history.CommandAuditor)
	engagementAuditor, _ := historySink.(history.EngagementAuditor)
	registry, err := guns.Build(
		gunDefinitions(cfg.Guns),
		commandAuditor,
		engagementAuditor,
		historySink,
		logger,
	)
	if err != nil {
		return fmt.Errorf("create gun registry: %w", err)
	}
	if err := applyStartupMode(cfg, engine, registry, logger); err != nil {
		return fmt.Errorf("apply PLC startup mode: %w", err)
	}
	if err := seedSequenceFloors(registry, historySink); err != nil {
		return err
	}

	safetyAuditor, _ := historySink.(safety.EventAuditor)
	safetyService := safety.NewService(cfg.SafetyPLC, safetyAuditor, logger)
	safetyService.Start()
	defer safetyService.Close()
	safetyLeaseCtx, stopSafetyLeases := context.WithCancel(ctx)
	var safetyLeaseMirrors sync.WaitGroup
	for _, instance := range registry.List() {
		safetyLeaseMirrors.Add(1)
		go func() {
			defer safetyLeaseMirrors.Done()
			mirrorSafetyLeases(safetyLeaseCtx, cfg.SafetyPLC, safetyService, instance, logger)
		}()
	}
	defer func() {
		stopSafetyLeases()
		safetyLeaseMirrors.Wait()
	}()
	safetyProvider := autoSafetyProvider{service: safetyService}
	for _, instance := range registry.List() {
		instance.Auto.SetSafetyDecisionProvider(safetyProvider)
	}
	registry.StartAll()
	defer registry.Close()

	feed := targetfeed.New(targetfeed.Config{
		Enabled:       cfg.TargetFeed.Enabled,
		MaxPending:    cfg.TargetFeed.MaxPending,
		TargetTTL:     cfg.TargetFeed.TargetTTL,
		HealthyWindow: cfg.TargetFeed.HealthyWindow,
		PollInterval:  cfg.TargetFeed.DispatchPollInterval,
	}, nil, logger)
	feed.Start()
	defer feed.Close()

	dispatcherStore, _ := historySink.(dispatcher.Store)
	if cfg.Dispatcher.Enabled && dispatcherStore == nil {
		return errors.New("dispatcher history store is unavailable")
	}
	dispatcherService := dispatcher.New(
		cfg.Dispatcher,
		feed,
		registry,
		safetyService,
		dispatcherStore,
		logger,
	)
	dispatcherService.Start()
	defer dispatcherService.Close()

	gun1, hasGun1 := registry.Get(plc.Gun1ID)
	if !hasGun1 {
		return errors.New("gun 1 is required for compatibility and telemetry")
	}
	autoTelemetry := telemetry.NewModbusServer(
		gun1.Definition.PLC,
		gun1.Auto,
		gun1.Commands,
		logger,
	)
	autoTelemetry.SetTargetFeed(feed)
	autoTelemetry.SetSafety(safetyService)
	if err := autoTelemetry.Start(); err != nil {
		return fmt.Errorf("start auto telemetry: %w", err)
	}
	defer autoTelemetry.Close()

	handler := api.NewRouter(api.Dependencies{
		Config:        cfg,
		Store:         state,
		Engine:        engine,
		History:       historySink,
		Guns:          registry,
		PLCCommands:   gun1.Commands,
		PLCLifecycle:  gun1.Lifecycle,
		AutoGun1:      gun1.Auto,
		Arbitration:   gun1.Arbitration,
		TargetFeed:    feed,
		Dispatcher:    dispatcherService,
		SafetyPLC:     safetyService,
		AutoTelemetry: autoTelemetry,
		Maintenance:   maintenance.NewFromEnv(),
		ControlToken:  os.Getenv("STONEHENGE_CONTROL_TOKEN"),
		IngestToken:   os.Getenv("STONEHENGE_INGEST_TOKEN"),
		Logger:        logger,
	})
	server := newHTTPServer(cfg.Server.Addr, handler)
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()
	logger.Info("middleware started", "addr", cfg.Server.Addr, "history", historySink.Name())

	var serveErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case serveErr = <-serverErrors:
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	if serveErr != nil {
		return fmt.Errorf("serve HTTP: %w", serveErr)
	}
	return nil
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		// Preserve the documented 20-minute synchronous maintenance ceiling.
		WriteTimeout:   21 * time.Minute,
		IdleTimeout:    time.Minute,
		MaxHeaderBytes: 1 << 20,
	}
}

func mirrorSafetyLeases(ctx context.Context, cfg config.SafetyPLCConfig, service *safety.Service, instance *guns.Instance, logger *slog.Logger) {
	interval := cfg.PollInterval
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer func() {
		clearCtx, cancel := context.WithTimeout(context.Background(), interval)
		defer cancel()
		_ = instance.Commands.WriteSafetyLeases(clearCtx, 0)
	}()

	var lease uint16
	writeFailed := false
	for {
		lease++
		if lease == 0 {
			lease = 1
		}
		value := uint16(0)
		if safety.EvaluateFireDecision(service.Snapshot(), instance.Definition.SafetyGunID).Allowed {
			value = lease
		}
		writeCtx, cancel := context.WithTimeout(ctx, interval)
		err := instance.Commands.WriteSafetyLeases(writeCtx, value)
		cancel()
		if err != nil && !writeFailed {
			logger.Warn("gun safety lease write failed", "gun_id", instance.Definition.ID, "error", err)
			writeFailed = true
		} else if err == nil && writeFailed {
			logger.Info("gun safety lease write recovered", "gun_id", instance.Definition.ID)
			writeFailed = false
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func applyStartupMode(cfg config.Config, engine *sim.Engine, registry *guns.Registry, logger *slog.Logger) error {
	var engineMode model.GunMode
	var arbitrationMode arbitration.Mode
	autoEnabled := false
	switch cfg.PLC.StartupMode {
	case "manual":
		engineMode = model.ModeManual
		arbitrationMode = arbitration.ModeManual
	case "auto":
		engineMode = model.ModeAuto
		arbitrationMode = arbitration.ModeAuto
		autoEnabled = true
	case "maintenance":
		engineMode = model.ModeMaintenance
		arbitrationMode = arbitration.ModeMaintenance
	default:
		return fmt.Errorf("unsupported mode %q", cfg.PLC.StartupMode)
	}

	for id := 1; id <= cfg.Simulation.GunCount; id++ {
		if _, err := engine.SetMode(id, engineMode); err != nil {
			return fmt.Errorf("set simulated gun %d mode: %w", id, err)
		}
	}

	deadline := time.Now().Add(cfg.PLC.StartupModeRetryTimeout)
	for _, instance := range registry.List() {
		instance.Auto.SetEnabled(false)
		for {
			if _, err := instance.Arbitration.RequestMode(context.Background(), arbitrationMode); err == nil {
				break
			} else if cfg.PLC.StartupModeRetryTimeout == 0 || !time.Now().Before(deadline) {
				return fmt.Errorf("set gun %d mode: %w", instance.Definition.ID, err)
			}
			time.Sleep(min(cfg.PLC.AutoReadinessPollInterval, time.Until(deadline)))
		}
		instance.Auto.SetEnabled(autoEnabled)
	}
	logger.Info("PLC startup mode applied", "mode", cfg.PLC.StartupMode)
	return nil
}

func gunDefinitions(configured []config.GunConfig) []guns.Definition {
	definitions := make([]guns.Definition, 0, len(configured))
	for _, gun := range configured {
		definitions = append(definitions, guns.Definition{
			ID:                  gun.ID,
			Name:                gun.Name,
			PLCEnabled:          gun.PLCEnabled,
			SafetyGunID:         gun.SafetyGunID,
			AutoEnabled:         gun.AutoEnabled,
			DispatcherEligible:  gun.DispatcherEligible,
			AutoExecutorEnabled: gun.AutoExecutorEnabled,
			MinAzimuth:          gun.MinAzimuth,
			MaxAzimuth:          gun.MaxAzimuth,
			MinElevation:        gun.MinElevation,
			MaxElevation:        gun.MaxElevation,
			PLC:                 gun.PLC,
		})
	}
	return definitions
}

func seedSequenceFloors(registry *guns.Registry, sink history.Sink) error {
	reader, ok := sink.(history.CommandSequenceReader)
	if !ok {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, instance := range registry.List() {
		sequence, found, err := reader.LatestGunCommandSequence(ctx, instance.Definition.ID)
		if err != nil {
			return fmt.Errorf("read gun %d command sequence floor: %w", instance.Definition.ID, err)
		}
		if found {
			instance.Commands.SeedSequenceFloor(sequence)
		}
	}
	return nil
}
