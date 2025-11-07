package metrics

import "github.com/prometheus/client_golang/prometheus"

var LatencyHist = prometheus.NewHistogramVec(
	prometheus.HistogramOpts{
		Name: "ottam_message_latency_ms",
		Help: "Latency per layer in milli seconds",
		Buckets: []float64{
			10, 20, 30, 40, 50, 60, 70, 80, 90, 100,
			110, 120, 130, 140, 150, 160, 170, 180, 190, 200,
			300, 500, 800, 1000, 2000, 3000, 4000, 5000, 10000, 20000, 30000,
		},
	},
	[]string{"layer"},
)

func init() {
	prometheus.MustRegister(LatencyHist)
}
