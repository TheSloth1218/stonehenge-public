package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stonehenge-targets/internal/model"
)

func sampleTrack() model.FragmentTrack {
	return model.FragmentTrack{
		ExternalID:          "USEA-TRACKING-NET-000001",
		SourceID:            "USEA-TRACKING-NET",
		Bearing:             247.3,
		Elevation:           41.8,
		MapX:                34.2,
		MapY:                52.7,
		Priority:            model.PriorityHigh,
		TerrainType:         model.TerrainLand,
		StrategicZone:       "SAN_SALVACION",
		TimeToImpactSeconds: 75,
		ObservedAt:          time.Date(2026, 6, 26, 14, 30, 0, 0, time.UTC),
	}
}

func TestRequestJSONSerializationAndBearerToken(t *testing.T) {
	var gotAuth string
	var got model.FragmentTrack
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content type = %q", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	c := New(server.URL, "secret-token", time.Second, 0, 0, nil)
	result := c.Submit(context.Background(), sampleTrack())
	if !result.Accepted {
		t.Fatalf("expected accepted result: %+v", result)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	if got.ExternalID != "USEA-TRACKING-NET-000001" || got.SourceID != "USEA-TRACKING-NET" {
		t.Fatalf("unexpected request body: %+v", got)
	}
}

func TestNoBearerTokenWhenUnset(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	c := New(server.URL, "", time.Second, 0, 0, nil)
	result := c.Submit(context.Background(), sampleTrack())
	if !result.Accepted {
		t.Fatalf("expected accepted result: %+v", result)
	}
	if gotAuth != "" {
		t.Fatalf("unexpected auth header %q", gotAuth)
	}
}

func TestRetriesOnNetworkErrors429And5xx(t *testing.T) {
	c := New("http://127.0.0.1:1", "", 10*time.Millisecond, 2, 0, nil)
	result := c.Submit(context.Background(), sampleTrack())
	if result.Attempts != 3 || result.Accepted || result.Permanent {
		t.Fatalf("expected network retries, got %+v", result)
	}

	for _, code := range []int{http.StatusTooManyRequests, http.StatusInternalServerError} {
		attempts := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(code)
				return
			}
			w.WriteHeader(http.StatusAccepted)
		}))
		c = New(server.URL, "", time.Second, 3, 0, nil)
		result = c.Submit(context.Background(), sampleTrack())
		server.Close()
		if !result.Accepted || result.Attempts != 3 {
			t.Fatalf("expected retry then success for %d, got %+v attempts=%d", code, result, attempts)
		}
	}
}

func TestBackoffIncreasesAndResetsPerSubmit(t *testing.T) {
	c := NewWithBackoff("http://127.0.0.1:1", "", time.Millisecond, 3, time.Second, 15*time.Second, false, nil)
	if got := c.delayForAttempt(1); got != time.Second {
		t.Fatalf("attempt 1 delay = %s, want 1s", got)
	}
	if got := c.delayForAttempt(2); got != 2*time.Second {
		t.Fatalf("attempt 2 delay = %s, want 2s", got)
	}
	if got := c.delayForAttempt(10); got != 15*time.Second {
		t.Fatalf("attempt 10 delay = %s, want 15s cap", got)
	}
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	c = NewWithBackoff(server.URL, "", time.Second, 3, time.Second, 15*time.Second, false, nil)
	first := c.Submit(context.Background(), sampleTrack())
	second := c.Submit(context.Background(), sampleTrack())
	if !first.Accepted || !second.Accepted || first.Attempts != 1 || second.Attempts != 1 || attempts != 2 {
		t.Fatalf("backoff did not reset after success: first=%+v second=%+v attempts=%d", first, second, attempts)
	}
}

func TestNoRetryOnPermanent4xx(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	c := New(server.URL, "", time.Second, 5, 0, nil)
	result := c.Submit(context.Background(), sampleTrack())
	if !result.Permanent || result.Attempts != 1 || attempts != 1 {
		t.Fatalf("expected one permanent attempt, got %+v attempts=%d", result, attempts)
	}
}

func TestPayloadAndExternalIDRetainedAcrossRetries(t *testing.T) {
	var bodies []model.FragmentTrack
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		var got model.FragmentTrack
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		bodies = append(bodies, got)
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	c := New(server.URL, "", time.Second, 3, 0, nil)
	result := c.Submit(context.Background(), sampleTrack())
	if !result.Accepted || len(bodies) != 3 {
		t.Fatalf("expected accepted after retries, got %+v bodies=%d", result, len(bodies))
	}
	for _, body := range bodies {
		if body.ExternalID != "USEA-TRACKING-NET-000001" || body != bodies[0] {
			t.Fatalf("payload changed across retries: %+v vs %+v", body, bodies[0])
		}
	}
}

func TestReadinessHandlesLargeTargetFeedResponse(t *testing.T) {
	largeMetadata := make([]map[string]interface{}, 0, 700)
	for i := 0; i < 700; i++ {
		largeMetadata = append(largeMetadata, map[string]interface{}{
			"state": "completed",
			"target": map[string]interface{}{
				"target_id": i + 1,
				"metadata": map[string]interface{}{
					"external_id": "USEA-TRACKING-NET-LARGE-RESPONSE-PADDING",
					"padding":     "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789",
				},
			},
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/dispatcher":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"enabled":                    true,
				"ready":                      true,
				"ownership_reconciled":       true,
				"target_ingest_allowed":      true,
				"queued_target_count":        0,
				"active_assignments":         []interface{}{},
				"uncertain_assignment_count": 1,
			})
		case "/api/v1/target-feed/targets":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"targets": largeMetadata})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	c := New(server.URL+"/api/v1/ingest/fragments", "", time.Second, 0, 0, nil)
	state, err := c.Readiness(context.Background(), 25)
	if err != nil {
		t.Fatalf("Readiness returned error: %v", err)
	}
	if !state.Ready || state.UncertainCount != 1 || state.OutstandingCount != 1 {
		t.Fatalf("unexpected readiness: %+v", state)
	}
}
