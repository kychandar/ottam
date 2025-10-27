package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

func TestRedisLatency(t *testing.T) {
	addr, exist := os.LookupEnv("VALKEY_ADDR")
	if !exist {
		t.Error("VALKEY_ADDR required")
		t.FailNow()
	}

	ctx := context.Background()
	rdb := redis.NewClient(&redis.Options{
		Addr: addr, // Redis address
	})

	channel := "latency_test"
	duration := 10 * time.Second     // test duration
	tickInterval := time.Millisecond // 1 ms between messages

	latencies := []float64{}

	// Subscriber
	done := make(chan bool)
	go func() {
		pubsub := rdb.Subscribe(ctx, channel)
		defer pubsub.Close()
		ch := pubsub.Channel()

		end := time.Now().Add(duration)
		for time.Now().Before(end) {
			msg, ok := <-ch
			if !ok {
				break
			}
			sentTime, err := time.Parse(time.RFC3339Nano, msg.Payload)
			if err != nil {
				log.Println("Error parsing timestamp:", err)
				continue
			}
			latency := time.Since(sentTime).Seconds() * 1000 // ms
			latencies = append(latencies, latency)
		}
		done <- true
	}()

	// Give subscriber a moment to start
	time.Sleep(100 * time.Millisecond)

	// Publisher using ticker
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	publishEnd := time.Now().Add(duration)
	go func() {
		for now := range ticker.C {
			if now.After(publishEnd) {
				break
			}
			timestamp := time.Now().Format(time.RFC3339Nano)
			if err := rdb.Publish(ctx, channel, timestamp).Err(); err != nil {
				log.Println("Publish error:", err)
			}
		}
	}()

	// Wait for subscriber to finish
	<-done

	// Sort latencies for percentile calculation
	sort.Float64s(latencies)

	getPercentile := func(p float64) float64 {
		if len(latencies) == 0 {
			return 0
		}
		k := int(float64(len(latencies)-1) * p / 100.0)
		return latencies[k]
	}

	// Calculate stats
	if len(latencies) == 0 {
		log.Println("No messages received")
		return
	}
	min := latencies[0]
	max := latencies[len(latencies)-1]
	sum := 0.0
	for _, l := range latencies {
		sum += l
	}
	avg := sum / float64(len(latencies))

	fmt.Printf("Test duration: %v\n", duration)
	fmt.Printf("Messages received: %d\n", len(latencies))
	fmt.Printf("Min latency: %.3f ms\n", min)
	fmt.Printf("Max latency: %.3f ms\n", max)
	fmt.Printf("Avg latency: %.3f ms\n", avg)
	fmt.Printf("P90 latency: %.3f ms\n", getPercentile(90))
	fmt.Printf("P95 latency: %.3f ms\n", getPercentile(95))
	fmt.Printf("P99 latency: %.3f ms\n", getPercentile(99))
}
