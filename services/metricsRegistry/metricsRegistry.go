package metricsregistry

import (
	"net/http"
	"time"

	"github.com/kychandar/ottam/services"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type metricsRegistry struct {
	handler     http.Handler
	instanceId  string
	latencyHist *prometheus.HistogramVec
	wsConnGuage *prometheus.GaugeVec
}

func New(instanceId string) services.MetricsRegistry {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	latencyHist := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "ottam_message_latency_ms",
			Help: "Latency per layer in milli seconds",
			Buckets: []float64{
				10, 20, 30, 40, 50, 60, 70, 80, 90, 100,
				110, 120, 130, 140, 150, 160, 170, 180, 190, 200,
				300, 500, 800, 1000, 2000, 3000, 4000, 5000, 10000, 20000, 30000,
			},
		},
		[]string{"instance_id", "layer"},
	)
	registry.MustRegister(latencyHist)

	wsConnGuage := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ws_connections_current",
			Help: "Number of currently active WebSocket connections",
		},
		[]string{"instance_id"},
	)

	registry.MustRegister(wsConnGuage)

	return &metricsRegistry{
		instanceId:  instanceId,
		handler:     promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		latencyHist: latencyHist,
		wsConnGuage: wsConnGuage,
	}
}

func (mr *metricsRegistry) GetHandler() http.Handler {
	return mr.handler
}

func (mr *metricsRegistry) ObserveCentSubscriberStartLat(msgPublishedTime time.Time) {
	mr.latencyHist.WithLabelValues(mr.instanceId, "cent_subscriber_start").Observe(float64(time.Since(msgPublishedTime).Milliseconds()))
}

func (mr *metricsRegistry) ObserveCentSubscriberFinishLat(msgPublishedTime time.Time) {
	mr.latencyHist.WithLabelValues(mr.instanceId, "cent_subscriber_done").Observe(float64(time.Since(msgPublishedTime).Milliseconds()))
}

func (mr *metricsRegistry) ObserveWsBridgeWriteStart(msgPublishedTime time.Time) {
	mr.latencyHist.WithLabelValues(mr.instanceId, "ws_bridge_ws_msg_write_start").Observe(float64(time.Since(msgPublishedTime).Milliseconds()))
}

func (mr *metricsRegistry) ObserveWsBridgeWriteEnd(msgPublishedTime time.Time) {
	mr.latencyHist.WithLabelValues(mr.instanceId, "ws_bridge_ws_msg_write_done").Observe(float64(time.Since(msgPublishedTime).Milliseconds()))
}

func (mr *metricsRegistry) IncWsConnectionCount() {
	mr.wsConnGuage.WithLabelValues(mr.instanceId).Inc()
}

func (mr *metricsRegistry) DecWsConnectionCount() {
	mr.wsConnGuage.WithLabelValues(mr.instanceId).Dec()
}
