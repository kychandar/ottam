package metrics

import "github.com/prometheus/client_golang/prometheus"

var LatencyHist = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "ottam_message_latency_ms",
		Help:    "Latency per layer in milli seconds",
		Buckets: prometheus.ExponentialBuckets(10, 2, 100), // 1µs → ~1s
	},
	[]string{"layer"},
)

func init() {
	prometheus.MustRegister(LatencyHist)
}
