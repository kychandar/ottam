package main

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
)

func Test2(t *testing.T) {
	// Logger (you can use watermill.NopLogger if you don't want output)
	logger := watermill.NopLogger{}

	// Create a new in-memory Pub/Sub using Go channels
	pubSub := gochannel.NewGoChannel(gochannel.Config{
		OutputChannelBuffer: 2000,
	}, logger)

	topic := "news"

	// Create two subscribers to the same topic (fan-out)
	ctx, cancel := context.WithCancel(context.TODO())
	var cache sync.Map
	n := 1000
	for i := 0; i < n; i++ {
		go func(i int) {
			subscriber, _ := pubSub.Subscribe(ctx, topic)
			for msg := range subscriber {
				tsInt, err := strconv.ParseInt(string(msg.Payload), 10, 64)
				if err != nil {
					panic(err)
				}
				cache.Store(i, time.Since(time.Unix(0, tsInt)).Milliseconds())
				msg.Ack() // acknowledge message
			}
		}(i)
	}

	// Publisher sends messages
	go publishMessage(ctx, pubSub, topic)
	time.Sleep(2 * time.Second) // give goroutines time to process
	cancel()

	pubSub.Close()

	var latencies []int64
	cache.Range(func(key, val any) bool {
		latencies = append(latencies, val.(int64))
		return true
	})

	slices.Sort(latencies)

	n = len(latencies)

	min := float64(latencies[0])
	max := float64(latencies[n-1])

	var sum float64
	for _, v := range latencies {
		sum += float64(v)
	}
	avg := sum / float64(n)

	// Percentile function
	p := func(pct float64) float64 {
		pos := pct / 100.0 * float64(n-1)
		i := int(pos)
		if i >= n-1 {
			return float64(latencies[n-1])
		}
		f := pos - float64(i)
		return float64(latencies[i]) + float64(latencies[i+1]-latencies[i])*f
	}

	fmt.Println("--------- Latency Summary (ms) ---------")
	fmt.Printf("Samples: %d\n", n)
	fmt.Printf("Min: %.3f ms\n", min)
	fmt.Printf("Max: %.3f ms\n", max)
	fmt.Printf("Avg: %.3f ms\n", avg)
	fmt.Printf("P70: %.3f ms\n", p(70))
	fmt.Printf("P75: %.3f ms\n", p(75))
	fmt.Printf("P80: %.3f ms\n", p(80))
	fmt.Printf("P85: %.3f ms\n", p(85))
	fmt.Printf("P90: %.3f ms\n", p(90))
	fmt.Printf("P95: %.3f ms\n", p(95))
	fmt.Println("latencies[len(latencies)-1]", latencies[len(latencies)-1])
	// fmt.Printf("P99: %.3f ms\n", p(99))
	// fmt.Printf("P99.9: %.3f ms\n", p(99.9))
	// fmt.Printf("P99.99: %.3f ms\n", p(99.99))
	// fmt.Printf("P99.999999999999999999999999: %.3f ms\n", p(99.999999999999999999999999))
	fmt.Println("----------------------------------------")
}

// Helper function to publish a message
func publishMessage(ctx context.Context, pub message.Publisher, topic string) {
	ticer := time.NewTicker(1 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticer.C:
			msg := message.NewMessage(watermill.NewUUID(), []byte(fmt.Sprintf("%v", time.Now().UnixNano())))
			if err := pub.Publish(topic, msg); err != nil {
				panic(err)
			}
		}
	}

}
