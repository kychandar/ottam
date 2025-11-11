package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" // optional: enables /debug/pprof endpoints
	"runtime"

	"github.com/kychandar/ottam/cmd"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cmd.Execute()
}

func init() {
	// Enable block profiling for performance analysis
	runtime.SetBlockProfileRate(1)
	
	// Register metrics handler globally
	// This will be available on the health check port (default 8081)
	// along with /health/ready and /health/live endpoints
	http.Handle("/metrics", promhttp.Handler())
	
	fmt.Println("Ottam Real-time Backend Server")
	fmt.Println("===============================")
	fmt.Println("Metrics will be available at: /metrics")
	fmt.Println("Profiling available at: /debug/pprof/")
}
