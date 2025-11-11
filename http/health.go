package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// HealthStatus represents the health status of the application
type HealthStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version,omitempty"`
	Uptime    int64     `json:"uptime_seconds,omitempty"`
}

// HealthChecker manages health check state
type HealthChecker struct {
	ready     atomic.Bool
	live      atomic.Bool
	startTime time.Time
	version   string
	logger    *slog.Logger
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(logger *slog.Logger, version string) *HealthChecker {
	hc := &HealthChecker{
		startTime: time.Now(),
		version:   version,
		logger:    logger,
	}
	// By default, liveness is true but readiness is false until explicitly set
	hc.live.Store(true)
	hc.ready.Store(false)
	return hc
}

// SetReady marks the service as ready to accept traffic
func (hc *HealthChecker) SetReady(ready bool) {
	hc.ready.Store(ready)
	if ready {
		hc.logger.Info("Service marked as ready")
	} else {
		hc.logger.Warn("Service marked as not ready")
	}
}

// SetLive marks the service as alive
func (hc *HealthChecker) SetLive(live bool) {
	hc.live.Store(live)
	if !live {
		hc.logger.Error("Service marked as not alive")
	}
}

// IsReady returns whether the service is ready
func (hc *HealthChecker) IsReady() bool {
	return hc.ready.Load()
}

// IsLive returns whether the service is alive
func (hc *HealthChecker) IsLive() bool {
	return hc.live.Load()
}

// ReadinessHandler handles readiness probe requests
func (hc *HealthChecker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hc.IsReady() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(HealthStatus{
				Status:    "not_ready",
				Timestamp: time.Now(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(HealthStatus{
			Status:    "ready",
			Timestamp: time.Now(),
			Version:   hc.version,
			Uptime:    int64(time.Since(hc.startTime).Seconds()),
		})
	}
}

// LivenessHandler handles liveness probe requests
func (hc *HealthChecker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !hc.IsLive() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(HealthStatus{
				Status:    "not_alive",
				Timestamp: time.Now(),
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(HealthStatus{
			Status:    "alive",
			Timestamp: time.Now(),
			Version:   hc.version,
			Uptime:    int64(time.Since(hc.startTime).Seconds()),
		})
	}
}

