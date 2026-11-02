package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"stonehenge-targets/internal/model"
)

func TestHealthAndStatusEndpoints(t *testing.T) {
	tracker := NewTracker("continuous", "http://user:pass@example.test/ingest?token=secret", "SRC", time.Date(2026, 6, 26, 14, 30, 0, 0, time.UTC))
	tracker.Generated("SRC-000001")
	handler := Handler(tracker)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/status", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d", rec.Code)
	}
	var status model.Status
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.GeneratedCount != 1 || status.LastGeneratedExternalID != "SRC-000001" {
		t.Fatalf("unexpected status: %+v", status)
	}
	body := rec.Body.String()
	if strings.Contains(body, "secret") || strings.Contains(body, "pass") {
		t.Fatalf("status leaked credential: %s", body)
	}
}
