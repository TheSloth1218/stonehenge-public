package maintenance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const Confirmation = "STONEHENGE_MAINTENANCE_RESET"

var allowedActions = []string{"status", "startup", "hard-reset", "complete-reset", "soft-reset"}
var allowedSoftTargets = []string{"all-guns", "gun1", "gun2", "gun3", "gun4", "gun5", "gun6", "gun7", "gun8", "middleware", "fuxa", "safety"}

type Service struct {
	enabled     bool
	token       string
	script      string
	timeout     time.Duration
	hostRepo    string
	runnerImage string
}

type Request struct {
	Action        string `json:"action"`
	Execute       bool   `json:"execute"`
	Confirm       string `json:"confirm"`
	Target        string `json:"target"`
	MiddlewareURL string `json:"middleware_url"`
	Credentials   string `json:"credentials"`
}

type Result struct {
	Enabled  bool            `json:"enabled"`
	Action   string          `json:"action,omitempty"`
	Executed bool            `json:"executed,omitempty"`
	Status   string          `json:"status"`
	Output   json.RawMessage `json:"output,omitempty"`
	Stdout   string          `json:"stdout,omitempty"`
	Stderr   string          `json:"stderr,omitempty"`
	Error    string          `json:"error,omitempty"`
}

func NewFromEnv() *Service {
	timeout := 10 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("STONEHENGE_MAINTENANCE_TIMEOUT")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			timeout = parsed
		}
	}
	return &Service{
		enabled:     envBool("STONEHENGE_MAINTENANCE_CONTROLS"),
		token:       os.Getenv("STONEHENGE_MAINTENANCE_TOKEN"),
		script:      envDefault("STONEHENGE_MAINTENANCE_SCRIPT", "./tools/maintenance/stonehenge-maintenance"),
		timeout:     timeout,
		hostRepo:    envDefault("STONEHENGE_HOST_REPO", "/opt/stonehenge-middleware"),
		runnerImage: envDefault("STONEHENGE_MAINTENANCE_RUNNER_IMAGE", "stonehenge-middleware-middleware"),
	}
}

func (s *Service) Enabled() bool {
	return s != nil && s.enabled
}

func (s *Service) Authorized(token string) bool {
	return s != nil && s.enabled && s.token != "" && token == s.token
}

func (s *Service) Status() Result {
	if s == nil || !s.enabled {
		return Result{Enabled: false, Status: "disabled"}
	}
	return Result{Enabled: true, Status: "enabled"}
}

func (s *Service) Run(parent context.Context, req Request) Result {
	if s == nil || !s.enabled {
		return Result{Enabled: false, Status: "disabled", Error: "maintenance controls are disabled"}
	}
	if !slices.Contains(allowedActions, req.Action) {
		return Result{Enabled: true, Status: "rejected", Error: "maintenance action is not allowed"}
	}
	if req.Action == "soft-reset" {
		target := req.Target
		if target == "" {
			target = "all-guns"
		}
		if !slices.Contains(allowedSoftTargets, target) {
			return Result{Enabled: true, Action: req.Action, Status: "rejected", Error: "soft reset target is not allowed"}
		}
		req.Target = target
	}
	if req.Execute && requiresConfirmation(req.Action) && req.Confirm != Confirmation {
		return Result{Enabled: true, Action: req.Action, Status: "rejected", Error: "live reset confirmation is required"}
	}
	if req.Execute && requiresDetachedRunner(req.Action) {
		return s.startDetachedRunner(parent, req)
	}
	ctx, cancel := context.WithTimeout(parent, s.timeout)
	defer cancel()
	args := []string{req.Action}
	if req.Execute {
		args = append(args, "--execute")
	}
	if req.Confirm != "" {
		args = append(args, "--confirm", req.Confirm)
	}
	if req.Target != "" {
		args = append(args, "--target", req.Target)
	}
	if req.MiddlewareURL != "" {
		args = append(args, "--middleware-url", req.MiddlewareURL)
	}
	if req.Credentials != "" {
		clean, err := cleanCredentialsPath(req.Credentials)
		if err != nil {
			return Result{Enabled: true, Action: req.Action, Status: "rejected", Error: err.Error()}
		}
		args = append(args, "--credentials", clean)
	}
	cmd := exec.CommandContext(ctx, s.script, args...)
	cmd.Dir = workingDir(s.script)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := Result{
		Enabled:  true,
		Action:   req.Action,
		Executed: req.Execute,
		Status:   "completed",
		Stdout:   truncate(stdout.String(), 20000),
		Stderr:   truncate(stderr.String(), 12000),
	}
	if len(stdout.Bytes()) > 0 && json.Valid(stdout.Bytes()) {
		result.Output = append(json.RawMessage(nil), stdout.Bytes()...)
		result.Stdout = ""
	}
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		if ctx.Err() != nil {
			result.Error = ctx.Err().Error()
		}
	}
	return result
}

func requiresConfirmation(action string) bool {
	return action == "startup" || action == "hard-reset" || action == "complete-reset"
}

func requiresDetachedRunner(action string) bool {
	return action == "startup" || action == "hard-reset" || action == "complete-reset"
}

func (s *Service) startDetachedRunner(parent context.Context, req Request) Result {
	if strings.TrimSpace(s.hostRepo) == "" {
		return Result{Enabled: true, Action: req.Action, Executed: true, Status: "failed", Error: "STONEHENGE_HOST_REPO is required for live reset runner"}
	}
	if strings.TrimSpace(s.runnerImage) == "" {
		return Result{Enabled: true, Action: req.Action, Executed: true, Status: "failed", Error: "STONEHENGE_MAINTENANCE_RUNNER_IMAGE is required for live reset runner"}
	}
	name := "stonehenge-maintenance-" + req.Action + "-" + time.Now().UTC().Format("20060102T150405")
	script := s.script
	if strings.TrimSpace(script) == "" || strings.HasPrefix(script, "/app/") {
		script = "/host/tools/maintenance/stonehenge-maintenance"
	}
	args := []string{
		"run",
		"-d",
		"--name",
		name,
		"--network",
		"host",
		"-v",
		s.hostRepo + ":/host",
		"-v",
		"/var/run/docker.sock:/var/run/docker.sock",
		"-w",
		"/host",
		"--entrypoint",
		script,
	}
	envFile := filepath.Join(s.hostRepo, ".env")
	if _, err := os.Stat(envFile); err == nil {
		args = append(args, "--env-file", envFile)
	}
	args = append(args,
		"-e", "COMPOSE_PROJECT_NAME="+envDefault("COMPOSE_PROJECT_NAME", "stonehenge-middleware"),
		"-e", "STONEHENGE_MIDDLEWARE_URL=http://127.0.0.1:8080",
		s.runnerImage,
		req.Action,
		"--execute",
		"--confirm",
		req.Confirm,
	)
	if req.Target != "" {
		args = append(args, "--target", req.Target)
	}
	if req.MiddlewareURL != "" {
		args = append(args, "--middleware-url", req.MiddlewareURL)
	}
	if req.Credentials != "" {
		clean, err := cleanCredentialsPath(req.Credentials)
		if err != nil {
			return Result{Enabled: true, Action: req.Action, Status: "rejected", Error: err.Error()}
		}
		args = append(args, "--credentials", clean)
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Result{
			Enabled:  true,
			Action:   req.Action,
			Executed: true,
			Status:   "failed",
			Stdout:   truncate(stdout.String(), 20000),
			Stderr:   truncate(stderr.String(), 12000),
			Error:    err.Error(),
		}
	}
	payload, _ := json.Marshal(map[string]string{
		"ok":           "true",
		"status":       "started",
		"runner":       name,
		"container_id": strings.TrimSpace(stdout.String()),
	})
	return Result{
		Enabled:  true,
		Action:   req.Action,
		Executed: true,
		Status:   "started",
		Output:   payload,
	}
}

func cleanCredentialsPath(path string) (string, error) {
	if strings.Contains(path, "\x00") {
		return "", errors.New("credential path contains invalid byte")
	}
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return "", errors.New("credential path must be absolute")
	}
	return clean, nil
}

func workingDir(script string) string {
	if filepath.IsAbs(script) {
		return filepath.Dir(filepath.Dir(filepath.Dir(script)))
	}
	return "."
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[len(value)-limit:]
}
