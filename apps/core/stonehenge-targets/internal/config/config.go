package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TargetEndpoint           string
	TargetInterval           time.Duration
	TargetJitter             time.Duration
	TargetRequestTimeout     time.Duration
	TargetSourceID           string
	TargetAuthToken          string
	TargetSeed               *int64
	TargetMaxRetries         int
	TargetRetryDelay         time.Duration
	TargetRetryMaxDelay      time.Duration
	TargetRetryJitter        bool
	TargetMode               string
	TargetBurstCount         int
	FeedListenAddress        string
	ReadinessPoll            time.Duration
	StartupStabilization     time.Duration
	MaxOutstandingTargets    int
	MaxPendingQueueThreshold int
	TargetTTL                time.Duration
	QueueFullBackoff         time.Duration
	TransientErrorBackoff    time.Duration
	RunDuration              time.Duration
	GracefulShutdownTimeout  time.Duration
	LogPath                  string
	StatusOutputPath         string
	DryRun                   bool
}

func Load() (Config, error) {
	cfg := Config{
		TargetEndpoint:           "http://192.168.50.20:8080/api/v1/ingest/fragments",
		TargetInterval:           10 * time.Second,
		TargetJitter:             4 * time.Second,
		TargetRequestTimeout:     5 * time.Second,
		TargetSourceID:           "USEA-TRACKING-NET",
		TargetMaxRetries:         5,
		TargetRetryDelay:         1 * time.Second,
		TargetRetryMaxDelay:      15 * time.Second,
		TargetRetryJitter:        true,
		TargetMode:               "continuous",
		TargetBurstCount:         10,
		FeedListenAddress:        "0.0.0.0:8090",
		ReadinessPoll:            30 * time.Second,
		StartupStabilization:     30 * time.Second,
		MaxOutstandingTargets:    30,
		MaxPendingQueueThreshold: 25,
		TargetTTL:                10 * time.Minute,
		QueueFullBackoff:         30 * time.Second,
		TransientErrorBackoff:    10 * time.Second,
		GracefulShutdownTimeout:  5 * time.Second,
	}
	cfg.TargetEndpoint = getenv("TARGET_ENDPOINT", cfg.TargetEndpoint)
	cfg.TargetSourceID = getenv("TARGET_SOURCE_ID", cfg.TargetSourceID)
	cfg.TargetAuthToken = os.Getenv("TARGET_AUTH_TOKEN")
	cfg.TargetMode = getenv("TARGET_MODE", cfg.TargetMode)
	cfg.FeedListenAddress = getenv("FEED_LISTEN_ADDRESS", cfg.FeedListenAddress)

	var err error
	cfg.TargetInterval, err = durationFromSeconds("TARGET_INTERVAL_SECONDS", cfg.TargetInterval)
	if err != nil {
		return cfg, err
	}
	cfg.TargetJitter, err = durationFromSeconds("TARGET_JITTER_SECONDS", cfg.TargetJitter)
	if err != nil {
		return cfg, err
	}
	cfg.TargetRequestTimeout, err = durationFromSeconds("TARGET_REQUEST_TIMEOUT_SECONDS", cfg.TargetRequestTimeout)
	if err != nil {
		return cfg, err
	}
	cfg.TargetRetryDelay, err = durationFromSeconds("TARGET_RETRY_DELAY_SECONDS", cfg.TargetRetryDelay)
	if err != nil {
		return cfg, err
	}
	cfg.TargetRetryMaxDelay, err = durationFromSeconds("TARGET_RETRY_MAX_DELAY_SECONDS", cfg.TargetRetryMaxDelay)
	if err != nil {
		return cfg, err
	}
	cfg.TargetRetryJitter, err = boolEnv("TARGET_RETRY_JITTER", cfg.TargetRetryJitter)
	if err != nil {
		return cfg, err
	}
	cfg.TargetMaxRetries, err = intEnv("TARGET_MAX_RETRIES", cfg.TargetMaxRetries)
	if err != nil {
		return cfg, err
	}
	cfg.TargetBurstCount, err = intEnv("TARGET_BURST_COUNT", cfg.TargetBurstCount)
	if err != nil {
		return cfg, err
	}
	cfg.ReadinessPoll, err = durationFromSeconds("TARGET_READINESS_POLL_SECONDS", cfg.ReadinessPoll)
	if err != nil {
		return cfg, err
	}
	cfg.StartupStabilization, err = durationFromSeconds("TARGET_STARTUP_STABILIZATION_SECONDS", cfg.StartupStabilization)
	if err != nil {
		return cfg, err
	}
	cfg.MaxOutstandingTargets, err = intEnv("TARGET_MAX_OUTSTANDING_TARGETS", cfg.MaxOutstandingTargets)
	if err != nil {
		return cfg, err
	}
	cfg.MaxPendingQueueThreshold, err = intEnv("TARGET_MAX_PENDING_QUEUE_THRESHOLD", cfg.MaxPendingQueueThreshold)
	if err != nil {
		return cfg, err
	}
	cfg.TargetTTL, err = durationFromSeconds("TARGET_TTL_SECONDS", cfg.TargetTTL)
	if err != nil {
		return cfg, err
	}
	cfg.QueueFullBackoff, err = durationFromSeconds("TARGET_QUEUE_FULL_BACKOFF_SECONDS", cfg.QueueFullBackoff)
	if err != nil {
		return cfg, err
	}
	cfg.TransientErrorBackoff, err = durationFromSeconds("TARGET_TRANSIENT_ERROR_BACKOFF_SECONDS", cfg.TransientErrorBackoff)
	if err != nil {
		return cfg, err
	}
	cfg.RunDuration, err = durationFromSeconds("TARGET_RUN_DURATION_SECONDS", cfg.RunDuration)
	if err != nil {
		return cfg, err
	}
	cfg.GracefulShutdownTimeout, err = durationFromSeconds("TARGET_GRACEFUL_SHUTDOWN_TIMEOUT_SECONDS", cfg.GracefulShutdownTimeout)
	if err != nil {
		return cfg, err
	}
	cfg.LogPath = os.Getenv("TARGET_LOG_PATH")
	cfg.StatusOutputPath = os.Getenv("TARGET_STATUS_OUTPUT_PATH")
	cfg.DryRun, err = boolEnv("TARGET_DRY_RUN", cfg.DryRun)
	if err != nil {
		return cfg, err
	}
	if raw := strings.TrimSpace(os.Getenv("TARGET_SEED")); raw != "" {
		seed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return cfg, fmt.Errorf("TARGET_SEED must be an integer: %w", err)
		}
		cfg.TargetSeed = &seed
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if _, err := url.ParseRequestURI(c.TargetEndpoint); err != nil {
		return fmt.Errorf("TARGET_ENDPOINT must be a complete URL: %w", err)
	}
	if c.TargetInterval <= 0 {
		return fmt.Errorf("TARGET_INTERVAL_SECONDS must be positive")
	}
	if c.TargetJitter < 0 {
		return fmt.Errorf("TARGET_JITTER_SECONDS must be non-negative")
	}
	if c.TargetRequestTimeout <= 0 {
		return fmt.Errorf("TARGET_REQUEST_TIMEOUT_SECONDS must be positive")
	}
	if strings.TrimSpace(c.TargetSourceID) == "" {
		return fmt.Errorf("TARGET_SOURCE_ID is required")
	}
	if c.TargetMaxRetries < 0 {
		return fmt.Errorf("TARGET_MAX_RETRIES must be non-negative")
	}
	if c.TargetRetryDelay < 0 {
		return fmt.Errorf("TARGET_RETRY_DELAY_SECONDS must be non-negative")
	}
	if c.TargetRetryMaxDelay <= 0 {
		return fmt.Errorf("TARGET_RETRY_MAX_DELAY_SECONDS must be positive")
	}
	if c.TargetBurstCount <= 0 {
		return fmt.Errorf("TARGET_BURST_COUNT must be positive")
	}
	if c.ReadinessPoll <= 0 {
		return fmt.Errorf("TARGET_READINESS_POLL_SECONDS must be positive")
	}
	if c.StartupStabilization < 0 {
		return fmt.Errorf("TARGET_STARTUP_STABILIZATION_SECONDS must be non-negative")
	}
	if c.MaxOutstandingTargets <= 0 {
		return fmt.Errorf("TARGET_MAX_OUTSTANDING_TARGETS must be positive")
	}
	if c.MaxPendingQueueThreshold < 0 {
		return fmt.Errorf("TARGET_MAX_PENDING_QUEUE_THRESHOLD must be non-negative")
	}
	if c.TargetTTL <= 0 {
		return fmt.Errorf("TARGET_TTL_SECONDS must be positive")
	}
	if c.QueueFullBackoff <= 0 {
		return fmt.Errorf("TARGET_QUEUE_FULL_BACKOFF_SECONDS must be positive")
	}
	if c.TransientErrorBackoff <= 0 {
		return fmt.Errorf("TARGET_TRANSIENT_ERROR_BACKOFF_SECONDS must be positive")
	}
	if c.RunDuration < 0 {
		return fmt.Errorf("TARGET_RUN_DURATION_SECONDS must be non-negative")
	}
	if c.GracefulShutdownTimeout <= 0 {
		return fmt.Errorf("TARGET_GRACEFUL_SHUTDOWN_TIMEOUT_SECONDS must be positive")
	}
	switch c.TargetMode {
	case "continuous", "burst", "once":
	default:
		return fmt.Errorf("TARGET_MODE must be continuous, burst, or once")
	}
	return nil
}

func (c Config) RetryDelayForAttempt(attempt int) time.Duration {
	delay := c.TargetRetryDelay
	if delay <= 0 {
		delay = time.Second
	}
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= c.TargetRetryMaxDelay {
			return c.TargetRetryMaxDelay
		}
	}
	return delay
}

func RedactEndpoint(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if u.User != nil {
		if username := u.User.Username(); username != "" {
			u.User = url.User(username)
		} else {
			u.User = nil
		}
	}
	q := u.Query()
	for key := range q {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "key") {
			q.Set(key, "REDACTED")
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return value, nil
}

func boolEnv(key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback, fmt.Errorf("%s must be true or false: %w", key, err)
	}
	return value, nil
}

func durationFromSeconds(key string, fallback time.Duration) (time.Duration, error) {
	value, err := intEnv(key, int(fallback/time.Second))
	if err != nil {
		return fallback, err
	}
	return time.Duration(value) * time.Second, nil
}
