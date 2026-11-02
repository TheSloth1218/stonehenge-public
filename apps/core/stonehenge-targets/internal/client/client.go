package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"stonehenge-targets/internal/model"
)

type Client struct {
	endpoint      string
	authToken     string
	httpClient    *http.Client
	maxRetries    int
	retryDelay    time.Duration
	maxRetryDelay time.Duration
	jitter        bool
	logger        *slog.Logger
}

const maxReadinessBodyBytes = 4 * 1024 * 1024

func New(endpoint, authToken string, timeout time.Duration, maxRetries int, retryDelay time.Duration, logger *slog.Logger) *Client {
	return NewWithBackoff(endpoint, authToken, timeout, maxRetries, retryDelay, 15*time.Second, true, logger)
}

func NewWithBackoff(endpoint, authToken string, timeout time.Duration, maxRetries int, retryDelay, maxRetryDelay time.Duration, jitter bool, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	if maxRetryDelay <= 0 {
		maxRetryDelay = retryDelay
	}
	return &Client{
		endpoint:      endpoint,
		authToken:     authToken,
		httpClient:    &http.Client{Timeout: timeout},
		maxRetries:    maxRetries,
		retryDelay:    retryDelay,
		maxRetryDelay: maxRetryDelay,
		jitter:        jitter,
		logger:        logger,
	}
}

func NewWithHTTPClient(endpoint, authToken string, httpClient *http.Client, maxRetries int, retryDelay time.Duration, logger *slog.Logger) *Client {
	c := New(endpoint, authToken, 5*time.Second, maxRetries, retryDelay, logger)
	c.httpClient = httpClient
	return c
}

func (c *Client) Submit(ctx context.Context, track model.FragmentTrack) model.SubmitResult {
	payload, err := json.Marshal(track)
	if err != nil {
		return model.SubmitResult{Permanent: true, Error: err.Error()}
	}
	return c.SubmitPayload(ctx, track.ExternalID, payload)
}

func (c *Client) SubmitPayload(ctx context.Context, externalID string, payload []byte) model.SubmitResult {
	attempts := c.maxRetries + 1
	var last model.SubmitResult
	for attempt := 1; attempt <= attempts; attempt++ {
		result := c.trySubmit(ctx, payload)
		result.Attempts = attempt
		if result.Accepted || result.Permanent {
			return result
		}
		last = result
		if attempt == attempts {
			break
		}
		delay := c.delayForAttempt(attempt)
		c.logger.Warn("target submit retry scheduled", "external_id", externalID, "attempt", attempt, "delay", delay.String(), "error", result.Error, "status", result.StatusCode)
		if !sleepContext(ctx, delay) {
			return model.SubmitResult{Attempts: attempt, Error: ctx.Err().Error()}
		}
	}
	return last
}

func (c *Client) SubmitPayloadOnce(ctx context.Context, payload []byte) model.SubmitResult {
	return c.trySubmit(ctx, payload)
}

func (c *Client) delayForAttempt(attempt int) time.Duration {
	delay := c.retryDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= c.maxRetryDelay {
			delay = c.maxRetryDelay
			break
		}
	}
	if c.jitter && delay > 0 {
		delta := time.Duration(rand.Int63n(int64(delay/2) + 1))
		delay = delay/2 + delta
	}
	return delay
}

func (c *Client) trySubmit(ctx context.Context, payload []byte) model.SubmitResult {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return model.SubmitResult{Permanent: true, Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return model.SubmitResult{Error: err.Error()}
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		result := model.SubmitResult{Accepted: true, StatusCode: resp.StatusCode}
		var decoded map[string]interface{}
		if err := json.Unmarshal(body, &decoded); err == nil {
			if duplicate, _ := decoded["duplicate"].(bool); duplicate {
				result.Duplicate = true
				result.Accepted = false
				result.Permanent = true
			}
		}
		return result
	}
	result := model.SubmitResult{
		StatusCode: resp.StatusCode,
		Error:      fmt.Sprintf("middleware returned HTTP %d: %s", resp.StatusCode, string(body)),
	}
	if retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); retryAfter > 0 {
		result.RetryAfter = retryAfter
	}
	if resp.StatusCode == http.StatusConflict && strings.Contains(string(body), "external_id") {
		result.Duplicate = true
		result.Permanent = true
		return result
	}
	if resp.StatusCode == http.StatusGone {
		result.Stale = true
		result.Permanent = true
		return result
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		result.QueueFull = true
		return result
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return result
	}
	if resp.StatusCode >= 400 && resp.StatusCode <= 499 {
		result.Permanent = true
	}
	return result
}

func (c *Client) Readiness(ctx context.Context, maxPendingThreshold int) (model.MiddlewareReadiness, error) {
	dispatcherURL, err := c.relatedURL("/api/v1/dispatcher")
	if err != nil {
		return model.MiddlewareReadiness{}, err
	}
	targetsURL, err := c.relatedURL("/api/v1/target-feed/targets")
	if err != nil {
		return model.MiddlewareReadiness{}, err
	}
	dispatcher, err := c.getJSON(ctx, dispatcherURL)
	if err != nil {
		return model.MiddlewareReadiness{Reason: err.Error()}, err
	}
	targets, err := c.getJSON(ctx, targetsURL)
	if err != nil {
		return model.MiddlewareReadiness{Reason: err.Error()}, err
	}
	queueDepth := intFrom(dispatcher["queued_target_count"])
	activeCount := lenArray(dispatcher["active_assignments"])
	uncertainCount := intFrom(dispatcher["uncertain_assignment_count"])
	pendingCount, outstandingRecords := targetCounts(targets["targets"])
	if queueDepth == 0 {
		queueDepth = pendingCount
	}
	outstanding := outstandingRecords
	if activeCount+queueDepth+uncertainCount > outstanding {
		outstanding = activeCount + queueDepth + uncertainCount
	}
	ready := boolFrom(dispatcher["enabled"]) &&
		boolFrom(dispatcher["ready"]) &&
		boolFrom(dispatcher["ownership_reconciled"]) &&
		boolFrom(dispatcher["target_ingest_allowed"])
	reason := stringFrom(dispatcher["no_eligible_gun_reason"])
	if reason == "" && !ready {
		reason = "dispatcher_not_ready"
	}
	if maxPendingThreshold > 0 && queueDepth >= maxPendingThreshold {
		ready = false
		reason = "pending_queue_threshold"
	}
	return model.MiddlewareReadiness{
		Ready:            ready,
		Reason:           reason,
		QueueDepth:       queueDepth,
		ActiveCount:      activeCount,
		UncertainCount:   uncertainCount,
		OutstandingCount: outstanding,
	}, nil
}

func (c *Client) relatedURL(path string) (string, error) {
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return "", err
	}
	u.Path = path
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxReadinessBodyBytes))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("middleware readiness HTTP %d: %s", resp.StatusCode, string(body))
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func targetCounts(value interface{}) (int, int) {
	items, ok := value.([]interface{})
	if !ok {
		return 0, 0
	}
	pending := 0
	outstanding := 0
	for _, item := range items {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		state := stringFrom(row["state"])
		switch state {
		case "completed", "expired", "rejected":
			continue
		case "pending":
			pending++
		}
		outstanding++
	}
	return pending, outstanding
}

func parseRetryAfter(raw string, now time.Time) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds <= 0 {
			return 0
		}
		return time.Duration(seconds) * time.Second
	}
	at, err := http.ParseTime(raw)
	if err != nil {
		return 0
	}
	delay := at.Sub(now)
	if delay < 0 {
		return 0
	}
	return delay
}

func boolFrom(value interface{}) bool {
	v, _ := value.(bool)
	return v
}

func stringFrom(value interface{}) string {
	v, _ := value.(string)
	return v
}

func intFrom(value interface{}) int {
	switch v := value.(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

func lenArray(value interface{}) int {
	v, _ := value.([]interface{})
	return len(v)
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
