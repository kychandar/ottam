package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHealthChecker(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(logger, "1.0.0")

	assert.NotNil(t, hc)
	assert.True(t, hc.IsLive())
	assert.False(t, hc.IsReady()) // Should be false initially
}

func TestHealthChecker_SetReady(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(logger, "1.0.0")

	assert.False(t, hc.IsReady())

	hc.SetReady(true)
	assert.True(t, hc.IsReady())

	hc.SetReady(false)
	assert.False(t, hc.IsReady())
}

func TestHealthChecker_SetLive(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(logger, "1.0.0")

	assert.True(t, hc.IsLive())

	hc.SetLive(false)
	assert.False(t, hc.IsLive())

	hc.SetLive(true)
	assert.True(t, hc.IsLive())
}

func TestReadinessHandler_NotReady(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(logger, "1.0.0")

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler := hc.ReadinessHandler()
	handler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response HealthStatus
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "not_ready", response.Status)
}

func TestReadinessHandler_Ready(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(logger, "1.0.0")
	hc.SetReady(true)

	// Wait a tiny bit to ensure uptime > 0
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()

	handler := hc.ReadinessHandler()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response HealthStatus
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "ready", response.Status)
	assert.Equal(t, "1.0.0", response.Version)
	assert.GreaterOrEqual(t, response.Uptime, int64(0))
}

func TestLivenessHandler_NotAlive(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(logger, "1.0.0")
	hc.SetLive(false)

	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()

	handler := hc.LivenessHandler()
	handler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response HealthStatus
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "not_alive", response.Status)
}

func TestLivenessHandler_Alive(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(logger, "1.0.0")

	// Wait a tiny bit to ensure uptime > 0
	time.Sleep(10 * time.Millisecond)

	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()

	handler := hc.LivenessHandler()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response HealthStatus
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	assert.Equal(t, "alive", response.Status)
	assert.Equal(t, "1.0.0", response.Version)
	assert.GreaterOrEqual(t, response.Uptime, int64(0))
}

func TestHealthChecker_UptimeIncreases(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(logger, "1.0.0")
	hc.SetReady(true)

	// First request
	req1 := httptest.NewRequest("GET", "/health/ready", nil)
	w1 := httptest.NewRecorder()
	handler := hc.ReadinessHandler()
	handler(w1, req1)

	var response1 HealthStatus
	err := json.NewDecoder(w1.Body).Decode(&response1)
	require.NoError(t, err)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Second request
	req2 := httptest.NewRequest("GET", "/health/ready", nil)
	w2 := httptest.NewRecorder()
	handler(w2, req2)

	var response2 HealthStatus
	err = json.NewDecoder(w2.Body).Decode(&response2)
	require.NoError(t, err)

	// Uptime should increase
	assert.GreaterOrEqual(t, response2.Uptime, response1.Uptime)
}

func TestHealthChecker_ConcurrentAccess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	hc := NewHealthChecker(logger, "1.0.0")

	done := make(chan bool)
	numGoroutines := 100

	// Concurrent reads and writes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			if id%2 == 0 {
				hc.SetReady(true)
				hc.SetLive(true)
			} else {
				_ = hc.IsReady()
				_ = hc.IsLive()
			}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Should not panic and should have valid state
	assert.NotPanics(t, func() {
		_ = hc.IsReady()
		_ = hc.IsLive()
	})
}

func TestHealthStatus_JSONMarshaling(t *testing.T) {
	status := HealthStatus{
		Status:    "ready",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    3600,
	}

	data, err := json.Marshal(status)
	require.NoError(t, err)
	assert.Contains(t, string(data), "ready")
	assert.Contains(t, string(data), "1.0.0")
	assert.Contains(t, string(data), "3600")
}

