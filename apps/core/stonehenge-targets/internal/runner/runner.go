package runner

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"stonehenge-targets/internal/client"
	"stonehenge-targets/internal/config"
	"stonehenge-targets/internal/generator"
	"stonehenge-targets/internal/model"
	"stonehenge-targets/internal/server"
)

type Runner struct {
	cfg     config.Config
	gen     *generator.Generator
	client  *client.Client
	tracker *server.Tracker
	logger  *slog.Logger
}

func New(cfg config.Config, gen *generator.Generator, client *client.Client, tracker *server.Tracker, logger *slog.Logger) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{cfg: cfg, gen: gen, client: client, tracker: tracker, logger: logger}
}

func (r *Runner) Run(ctx context.Context) error {
	switch r.cfg.TargetMode {
	case "once":
		return r.submitOne(ctx)
	case "burst":
		for i := 0; i < r.cfg.TargetBurstCount; i++ {
			if err := r.submitOne(ctx); err != nil {
				return err
			}
		}
		return nil
	default:
		return r.runContinuous(ctx)
	}
}

func (r *Runner) runContinuous(ctx context.Context) error {
	timer := time.NewTimer(0)
	defer timer.Stop()
	var pendingID string
	var pendingPayload []byte
	var pendingGeneratedAt time.Time
	attemptCycle := 0
	stabilized := false
	for {
		select {
		case <-ctx.Done():
			if pendingID != "" {
				r.tracker.Pending(pendingID)
				r.logger.Info("shutdown with pending target retained in memory", "external_id", pendingID, "generated_at", pendingGeneratedAt)
			}
			r.writeStatus()
			return nil
		case <-timer.C:
			if r.cfg.ReadinessPoll > 0 {
				ready, delay := r.waitForReadiness(ctx, &stabilized)
				if !ready {
					timer.Reset(delay)
					continue
				}
			}
			if pendingPayload == nil {
				track := r.gen.Next(time.Now().UTC())
				r.tracker.Generated(track.ExternalID)
				r.tracker.Pending(track.ExternalID)
				payload, err := json.Marshal(track)
				if err != nil {
					r.tracker.Rejected(err.Error())
					r.logger.Warn("target rejected before submit", "external_id", track.ExternalID, "error", err)
					timer.Reset(r.cfg.TargetInterval)
					continue
				}
				pendingID = track.ExternalID
				pendingPayload = payload
				pendingGeneratedAt = track.ObservedAt
				attemptCycle = 0
			}
			if time.Since(pendingGeneratedAt) > r.targetTTL() {
				r.tracker.ExpiredUnsent(pendingID)
				r.logger.Warn("target expired before successful acceptance", "external_id", pendingID, "age", time.Since(pendingGeneratedAt).String())
				pendingID = ""
				pendingPayload = nil
				pendingGeneratedAt = time.Time{}
				attemptCycle = 0
				timer.Reset(r.nextGenerationDelay())
				continue
			}
			if r.cfg.DryRun {
				r.tracker.Accepted(time.Now().UTC())
				r.logger.Info("dry-run target generated but not submitted", "external_id", pendingID)
				pendingID = ""
				pendingPayload = nil
				pendingGeneratedAt = time.Time{}
				attemptCycle = 0
				timer.Reset(r.nextGenerationDelay())
				continue
			}
			result := r.client.SubmitPayloadOnce(ctx, pendingPayload)
			result.Attempts = 1
			if result.Accepted {
				r.tracker.Accepted(time.Now().UTC())
				r.logger.Info("target accepted", "external_id", pendingID, "attempts", result.Attempts, "status", result.StatusCode)
				pendingID = ""
				pendingPayload = nil
				pendingGeneratedAt = time.Time{}
				attemptCycle = 0
				timer.Reset(r.nextGenerationDelay())
				continue
			}
			if result.Duplicate {
				r.tracker.Duplicate(result.Error)
				r.logger.Warn("target duplicate acknowledged; not retrying external_id", "external_id", pendingID, "status", result.StatusCode, "error", result.Error)
				pendingID = ""
				pendingPayload = nil
				pendingGeneratedAt = time.Time{}
				attemptCycle = 0
				timer.Reset(r.nextGenerationDelay())
				continue
			}
			if result.Stale {
				r.tracker.ExpiredUnsent(pendingID)
				r.logger.Warn("middleware rejected target as stale", "external_id", pendingID, "status", result.StatusCode, "error", result.Error)
				pendingID = ""
				pendingPayload = nil
				pendingGeneratedAt = time.Time{}
				attemptCycle = 0
				timer.Reset(r.nextGenerationDelay())
				continue
			}
			if result.Permanent {
				r.tracker.Rejected(result.Error)
				r.logger.Warn("target rejected permanently", "external_id", pendingID, "attempts", result.Attempts, "status", result.StatusCode, "error", result.Error)
				pendingID = ""
				pendingPayload = nil
				pendingGeneratedAt = time.Time{}
				attemptCycle = 0
				timer.Reset(r.nextGenerationDelay())
				continue
			}
			attemptCycle++
			delay := r.retryDelay(result, attemptCycle)
			r.tracker.Retried(2)
			if result.StatusCode == 503 {
				r.tracker.HTTP503(result.Error)
			} else if result.QueueFull {
				r.tracker.HTTP429(result.Error)
			} else {
				r.tracker.Failed(result.Error)
			}
			r.logger.Warn("target delivery pending; middleware unavailable or retryable response", "external_id", pendingID, "next_retry", delay.String(), "attempts", result.Attempts, "status", result.StatusCode, "error", result.Error)
			timer.Reset(delay)
		}
	}
}

func (r *Runner) targetTTL() time.Duration {
	if r.cfg.TargetTTL > 0 {
		return r.cfg.TargetTTL
	}
	return 10 * time.Minute
}

func (r *Runner) waitForReadiness(ctx context.Context, stabilized *bool) (bool, time.Duration) {
	state, err := r.client.Readiness(ctx, r.cfg.MaxPendingQueueThreshold)
	if err != nil {
		*stabilized = false
		r.logger.Warn("middleware readiness check failed", "error", err)
		return false, r.cfg.ReadinessPoll
	}
	if r.cfg.MaxOutstandingTargets > 0 && state.OutstandingCount >= r.cfg.MaxOutstandingTargets {
		*stabilized = false
		r.logger.Info("target generation paused by outstanding limit", "outstanding", state.OutstandingCount, "limit", r.cfg.MaxOutstandingTargets)
		return false, r.cfg.ReadinessPoll
	}
	if !state.Ready {
		*stabilized = false
		r.logger.Info("target generation paused until middleware ready", "reason", state.Reason, "queue_depth", state.QueueDepth, "active", state.ActiveCount, "uncertain", state.UncertainCount)
		return false, r.cfg.ReadinessPoll
	}
	if !*stabilized && r.cfg.StartupStabilization > 0 {
		r.logger.Info("middleware readiness observed; waiting stabilization period", "delay", r.cfg.StartupStabilization.String())
		if !sleepContext(ctx, r.cfg.StartupStabilization) {
			return false, r.cfg.ReadinessPoll
		}
		*stabilized = true
		return false, 0
	}
	return true, 0
}

func (r *Runner) retryDelay(result model.SubmitResult, attempt int) time.Duration {
	if result.RetryAfter > 0 {
		return result.RetryAfter
	}
	if result.QueueFull {
		return r.cfg.QueueFullBackoff
	}
	if result.StatusCode == 0 {
		return r.cfg.TransientErrorBackoff
	}
	return r.cfg.RetryDelayForAttempt(attempt)
}

func (r *Runner) nextGenerationDelay() time.Duration {
	delay := r.cfg.TargetInterval
	if r.cfg.TargetJitter > 0 {
		jitter := time.Duration(rand.Int63n(int64(r.cfg.TargetJitter)*2+1)) - r.cfg.TargetJitter
		delay += jitter
	}
	if delay <= 0 {
		return time.Millisecond
	}
	return delay
}

func (r *Runner) writeStatus() {
	if r.cfg.StatusOutputPath == "" {
		return
	}
	status := r.tracker.Snapshot()
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		r.logger.Warn("status output encode failed", "error", err)
		return
	}
	if err := os.WriteFile(r.cfg.StatusOutputPath, append(data, '\n'), 0644); err != nil {
		r.logger.Warn("status output write failed", "path", r.cfg.StatusOutputPath, "error", err)
	}
}

func sleepContext(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (r *Runner) submitOne(ctx context.Context) error {
	track := r.gen.Next(time.Now().UTC())
	r.tracker.Generated(track.ExternalID)
	result := r.client.Submit(ctx, track)
	r.tracker.Retried(result.Attempts)
	if result.Accepted {
		r.tracker.Accepted(time.Now().UTC())
		r.logger.Info("target accepted", "external_id", track.ExternalID, "attempts", result.Attempts, "status", result.StatusCode)
		return nil
	}
	if result.Permanent {
		r.tracker.Rejected(result.Error)
		r.logger.Warn("target rejected permanently", "external_id", track.ExternalID, "attempts", result.Attempts, "status", result.StatusCode, "error", result.Error)
		return nil
	}
	r.tracker.Failed(result.Error)
	r.logger.Error("target failed after retries", "external_id", track.ExternalID, "attempts", result.Attempts, "status", result.StatusCode, "error", result.Error)
	return nil
}
