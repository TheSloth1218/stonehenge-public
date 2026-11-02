package config

import (
	"testing"
	"time"
)

func TestRedactEndpoint(t *testing.T) {
	got := RedactEndpoint("http://user:pass@example.test/path?token=secret&ok=1")
	if got != "http://user@example.test/path?ok=1&token=REDACTED" {
		t.Fatalf("redacted endpoint = %q", got)
	}
}

func TestLoadUsesConfigurableMiddlewareEndpointAndRetryDefaults(t *testing.T) {
	t.Setenv("TARGET_ENDPOINT", "http://127.0.0.1:8080/api/v1/ingest/fragments")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.TargetEndpoint != "http://127.0.0.1:8080/api/v1/ingest/fragments" {
		t.Fatalf("endpoint = %q", cfg.TargetEndpoint)
	}
	if cfg.TargetRetryDelay != time.Second || cfg.TargetRetryMaxDelay != 15*time.Second || !cfg.TargetRetryJitter {
		t.Fatalf("retry defaults = delay %s max %s jitter %v", cfg.TargetRetryDelay, cfg.TargetRetryMaxDelay, cfg.TargetRetryJitter)
	}
}

func TestNoComposeOnlyHostnameDefault(t *testing.T) {
	t.Setenv("TARGET_ENDPOINT", "")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.TargetEndpoint == "http://middleware:8080/api/v1/ingest/fragments" {
		t.Fatal("default endpoint must not depend on Compose service DNS")
	}
}
