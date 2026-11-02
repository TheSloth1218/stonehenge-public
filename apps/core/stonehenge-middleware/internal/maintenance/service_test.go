package maintenance

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestServiceAuthorization(t *testing.T) {
	service := &Service{enabled: true, token: "secret"}
	if !service.Authorized("secret") {
		t.Fatal("expected matching token to authorize")
	}
	if service.Authorized("wrong") {
		t.Fatal("expected wrong token to fail")
	}
	disabled := &Service{enabled: false, token: "secret"}
	if disabled.Authorized("secret") {
		t.Fatal("expected disabled service to fail authorization")
	}
}

func TestRunRejectsLiveResetWithoutConfirmation(t *testing.T) {
	service := &Service{enabled: true, token: "secret", script: testScript(t), timeout: testTimeout()}
	result := service.Run(context.Background(), Request{Action: "hard-reset", Execute: true})
	if result.Status != "rejected" {
		t.Fatalf("status = %q, want rejected", result.Status)
	}
}

func TestRunRejectsUnknownSoftTarget(t *testing.T) {
	service := &Service{enabled: true, token: "secret", script: testScript(t), timeout: testTimeout()}
	result := service.Run(context.Background(), Request{Action: "soft-reset", Target: "database"})
	if result.Status != "rejected" {
		t.Fatalf("status = %q, want rejected", result.Status)
	}
}

func TestRunExecutesDryRunAction(t *testing.T) {
	service := &Service{enabled: true, token: "secret", script: testScript(t), timeout: testTimeout()}
	result := service.Run(context.Background(), Request{Action: "soft-reset", Target: "gun3"})
	if result.Status != "completed" {
		t.Fatalf("status = %q error=%q stderr=%q", result.Status, result.Error, result.Stderr)
	}
	if len(result.Output) == 0 {
		t.Fatalf("expected JSON output, got stdout %q", result.Stdout)
	}
}

func testTimeout() time.Duration {
	return 5 * time.Second
}

func testScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "maintenance-test-script")
	body := "#!/bin/sh\nprintf '{\"ok\":true,\"args\":\"%s\"}\\n' \"$*\"\n"
	if runtime.GOOS == "windows" {
		t.Skip("shell script test is unix-only")
	}
	if err := os.WriteFile(script, []byte(body), 0755); err != nil {
		t.Fatal(err)
	}
	return script
}
