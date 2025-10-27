package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valkey-io/valkey-go"
)

type LatencyStats struct {
	count     atomic.Int64
	totalNs   atomic.Int64
	minNs     atomic.Int64
	maxNs     atomic.Int64
	latencies []int64
	mu        sync.Mutex
}

func NewLatencyStats() *LatencyStats {
	stats := &LatencyStats{
		latencies: make([]int64, 0, 10000),
	}
	stats.minNs.Store(math.MaxInt64)
	return stats
}

func (ls *LatencyStats) Record(latencyNs int64) {
	ls.count.Add(1)
	ls.totalNs.Add(latencyNs)

	// Update min
	for {
		oldMin := ls.minNs.Load()
		if latencyNs >= oldMin {
			break
		}
		if ls.minNs.CompareAndSwap(oldMin, latencyNs) {
			break
		}
	}

	// Update max
	for {
		oldMax := ls.maxNs.Load()
		if latencyNs <= oldMax {
			break
		}
		if ls.maxNs.CompareAndSwap(oldMax, latencyNs) {
			break
		}
	}

	// Store for percentile calculation
	ls.mu.Lock()
	ls.latencies = append(ls.latencies, latencyNs)
	ls.mu.Unlock()
}

func (ls *LatencyStats) GetStats() (avg, min, max, p50, p95, p99 float64) {
	count := ls.count.Load()
	if count == 0 {
		return 0, 0, 0, 0, 0, 0
	}

	total := ls.totalNs.Load()
	avg = float64(total) / float64(count) / 1e6 // Convert to ms
	min = float64(ls.minNs.Load()) / 1e6
	max = float64(ls.maxNs.Load()) / 1e6

	// Calculate percentiles
	ls.mu.Lock()
	sorted := make([]int64, len(ls.latencies))
	copy(sorted, ls.latencies)
	ls.mu.Unlock()

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	if len(sorted) > 0 {
		p50 = float64(sorted[len(sorted)*50/100]) / 1e6
		p95 = float64(sorted[len(sorted)*95/100]) / 1e6
		p99 = float64(sorted[len(sorted)*99/100]) / 1e6
	}

	return
}

func testPing(client valkey.Client, iterations int) {
	ctx := context.Background()
	stats := NewLatencyStats()

	fmt.Printf("Running PING test (%d iterations)...\n", iterations)

	for i := 0; i < iterations; i++ {
		start := time.Now()

		err := client.Do(ctx, client.B().Ping().Build()).Error()

		elapsed := time.Since(start).Nanoseconds()

		if err != nil {
			fmt.Printf("Error on iteration %d: %v\n", i, err)
			continue
		}

		stats.Record(elapsed)
	}

	avg, min, max, p50, p95, p99 := stats.GetStats()

	fmt.Printf("\nPING Latency Results:\n")
	fmt.Printf("  Count:   %d\n", stats.count.Load())
	fmt.Printf("  Average: %.3f ms\n", avg)
	fmt.Printf("  Min:     %.3f ms\n", min)
	fmt.Printf("  Max:     %.3f ms\n", max)
	fmt.Printf("  P50:     %.3f ms\n", p50)
	fmt.Printf("  P95:     %.3f ms\n", p95)
	fmt.Printf("  P99:     %.3f ms\n", p99)
	fmt.Println()
}

func testSetGet(client valkey.Client, iterations int) {
	ctx := context.Background()
	setStats := NewLatencyStats()
	getStats := NewLatencyStats()

	fmt.Printf("Running SET/GET test (%d iterations)...\n", iterations)

	for i := 0; i < iterations; i++ {
		key := fmt.Sprintf("latency_test:%d", i)
		value := fmt.Sprintf("value_%d", i)

		// Test SET
		start := time.Now()
		err := client.Do(ctx, client.B().Set().Key(key).Value(value).Build()).Error()
		elapsed := time.Since(start).Nanoseconds()

		if err != nil {
			fmt.Printf("SET error on iteration %d: %v\n", i, err)
			continue
		}
		setStats.Record(elapsed)

		// Test GET
		start = time.Now()
		result := client.Do(ctx, client.B().Get().Key(key).Build())
		elapsed = time.Since(start).Nanoseconds()

		if result.Error() != nil {
			fmt.Printf("GET error on iteration %d: %v\n", i, result.Error())
			continue
		}
		getStats.Record(elapsed)
	}

	// Print SET stats
	avg, min, max, p50, p95, p99 := setStats.GetStats()
	fmt.Printf("\nSET Latency Results:\n")
	fmt.Printf("  Count:   %d\n", setStats.count.Load())
	fmt.Printf("  Average: %.3f ms\n", avg)
	fmt.Printf("  Min:     %.3f ms\n", min)
	fmt.Printf("  Max:     %.3f ms\n", max)
	fmt.Printf("  P50:     %.3f ms\n", p50)
	fmt.Printf("  P95:     %.3f ms\n", p95)
	fmt.Printf("  P99:     %.3f ms\n", p99)
	fmt.Println()

	// Print GET stats
	avg, min, max, p50, p95, p99 = getStats.GetStats()
	fmt.Printf("GET Latency Results:\n")
	fmt.Printf("  Count:   %d\n", getStats.count.Load())
	fmt.Printf("  Average: %.3f ms\n", avg)
	fmt.Printf("  Min:     %.3f ms\n", min)
	fmt.Printf("  Max:     %.3f ms\n", max)
	fmt.Printf("  P50:     %.3f ms\n", p50)
	fmt.Printf("  P95:     %.3f ms\n", p95)
	fmt.Printf("  P99:     %.3f ms\n", p99)
	fmt.Println()
}

func testPubSub(client valkey.Client, iterations int) {
	ctx := context.Background()
	pubStats := NewLatencyStats()

	fmt.Printf("Running PUBLISH test (%d iterations)...\n", iterations)

	channel := "latency_test_channel"

	for i := 0; i < iterations; i++ {
		message := fmt.Sprintf("message_%d_%d", i, time.Now().UnixNano())

		start := time.Now()
		err := client.Do(ctx, client.B().Publish().Channel(channel).Message(message).Build()).Error()
		elapsed := time.Since(start).Nanoseconds()

		if err != nil {
			fmt.Printf("PUBLISH error on iteration %d: %v\n", i, err)
			continue
		}

		pubStats.Record(elapsed)
	}

	avg, min, max, p50, p95, p99 := pubStats.GetStats()

	fmt.Printf("\nPUBLISH Latency Results:\n")
	fmt.Printf("  Count:   %d\n", pubStats.count.Load())
	fmt.Printf("  Average: %.3f ms\n", avg)
	fmt.Printf("  Min:     %.3f ms\n", min)
	fmt.Printf("  Max:     %.3f ms\n", max)
	fmt.Printf("  P50:     %.3f ms\n", p50)
	fmt.Printf("  P95:     %.3f ms\n", p95)
	fmt.Printf("  P99:     %.3f ms\n", p99)
	fmt.Println()
}

func testConcurrentLoad(client valkey.Client, concurrency, iterations int) {
	ctx := context.Background()
	stats := NewLatencyStats()

	fmt.Printf("Running concurrent SET test (%d goroutines, %d iterations each)...\n",
		concurrency, iterations)

	var wg sync.WaitGroup

	start := time.Now()

	for c := 0; c < concurrency; c++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("concurrent:%d:%d", workerID, i)
				value := fmt.Sprintf("value_%d_%d", workerID, i)

				opStart := time.Now()
				err := client.Do(ctx, client.B().Set().Key(key).Value(value).Build()).Error()
				elapsed := time.Since(opStart).Nanoseconds()

				if err == nil {
					stats.Record(elapsed)
				}
			}
		}(c)
	}

	wg.Wait()
	totalTime := time.Since(start)

	avg, min, max, p50, p95, p99 := stats.GetStats()
	totalOps := stats.count.Load()
	opsPerSec := float64(totalOps) / totalTime.Seconds()

	fmt.Printf("\nConcurrent Load Results:\n")
	fmt.Printf("  Total Ops:   %d\n", totalOps)
	fmt.Printf("  Total Time:  %.2f s\n", totalTime.Seconds())
	fmt.Printf("  Throughput:  %.0f ops/sec\n", opsPerSec)
	fmt.Printf("  Average:     %.3f ms\n", avg)
	fmt.Printf("  Min:         %.3f ms\n", min)
	fmt.Printf("  Max:         %.3f ms\n", max)
	fmt.Printf("  P50:         %.3f ms\n", p50)
	fmt.Printf("  P95:         %.3f ms\n", p95)
	fmt.Printf("  P99:         %.3f ms\n", p99)
	fmt.Println()
}

func TestV(t *testing.T) {
	addr, exist := os.LookupEnv("VALKEY_ADDR")
	if !exist {
		t.Error("VALKEY_ADDR required")
		t.FailNow()
	}
	// Create Valkey client
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{addr},
	})
	if err != nil {
		panic(err)
	}
	defer client.Close()

	fmt.Println("=== Valkey Latency Benchmark ===")

	// Run tests
	testPing(client, 1000)
	testSetGet(client, 1000)
	testPubSub(client, 1000)
	testConcurrentLoad(client, 10, 100) // 10 goroutines, 100 ops each

	fmt.Println("Benchmark complete!")
}
