package http

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/proxysql/orchestrator/go/process"
)

func TestHealthLiveReturns200(t *testing.T) {
	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()

	HealthLive(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json content type, got %s", ct)
	}
}

func TestHealthReadyReturns200WhenHealthy(t *testing.T) {
	atomic.StoreInt64(&process.LastContinousCheckHealthy, 1)
	defer atomic.StoreInt64(&process.LastContinousCheckHealthy, 0)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	HealthReady(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHealthReadyReturns503WhenUnhealthy(t *testing.T) {
	atomic.StoreInt64(&process.LastContinousCheckHealthy, 0)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	HealthReady(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

func TestHealthLeaderReturns503WhenNotLeader(t *testing.T) {
	// With raft disabled and health check not healthy, this should return 503
	atomic.StoreInt64(&process.LastContinousCheckHealthy, 0)

	req := httptest.NewRequest("GET", "/health/leader", nil)
	w := httptest.NewRecorder()

	HealthLeader(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d", w.Code)
	}
}

func TestHealthLeaderReturns200WhenHealthyNoRaft(t *testing.T) {
	// With raft disabled, healthy node is treated as leader
	atomic.StoreInt64(&process.LastContinousCheckHealthy, 1)
	defer atomic.StoreInt64(&process.LastContinousCheckHealthy, 0)

	req := httptest.NewRequest("GET", "/health/leader", nil)
	w := httptest.NewRecorder()

	HealthLeader(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
