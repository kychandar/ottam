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
	// Start a simple HTTP server in a goroutine
	runtime.SetBlockProfileRate(1)
	go func() {
		port := 8081
		fmt.Printf("Starting server on :%d\n", port)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			fmt.Printf("HTTP server error: %v\n", err)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())

}
