package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

const url = "ws://localhost:30080/ws"

var (
	msgBytes []byte
)

func Test3(t *testing.T) {
	fmt.Println("Connecting to:", url)

	// Handle Ctrl+C
	interrupt := make(chan os.Signal, 1)

	// Example control-plane message
	message := map[string]any{
		"Version":               1,
		"Id":                    "abc123",
		"IsControlPlaneMessage": true,
		"Payload": map[string]any{
			"Version":        1,
			"ControlPlaneOp": 0,
			"Payload": map[string]any{
				"ChannelName": "t1",
			},
		},
	}

	var err error
	msgBytes, err = json.Marshal(message)
	if err != nil {
		log.Fatal("marshal:", err)
	}

	// Channel for sending latencies
	latencyCh := make(chan float64, 1000000000)
	const fileName = "latencies.txt"
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()
	// Writer goroutine
	go func() {
		for latency := range latencyCh {
			fmt.Fprintf(file, "%.6f\n", latency)
		}
	}()

	wg := &sync.WaitGroup{}
	n := 1000
	ctx, cancel := context.WithCancel(context.TODO())

	for i := 0; i < n; i++ {
		go wsConn(ctx, wg, latencyCh)
	}

	// Run for a while
	testDuration := 30 * time.Second
	select {
	case <-interrupt:
		fmt.Printf("Stopping ")
		cancel()
	case <-time.After(testDuration):
		fmt.Printf("Stopping after %v\n", testDuration)
		cancel()
	}
	wg.Wait()

	printLatencyStatsFromFile(fileName)
}

func wsConn(ctx context.Context, wg *sync.WaitGroup, writerChan chan<- float64) {
	wg.Add(1)
	defer wg.Done()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer conn.Close()
	err = conn.WriteMessage(websocket.TextMessage, msgBytes)
	if err != nil {
		log.Println("write error:", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			conn.Close()
			return

		default:
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Println("read error:", err)
				return
			}

			m := NewEmpty()
			if err := json.Unmarshal(msg, &m); err != nil {
				log.Println("unmarshal error:", err)
				continue
			}

			latencyNs := time.Since(m.GetPublishedTime()).Nanoseconds()
			fmt.Println(time.Since(m.GetPublishedTime()).Seconds())
			latencyMs := float64(latencyNs) / 1e6
			writerChan <- latencyMs
		}
	}
}
func printLatencyStatsFromFile(fileName string) {
	file, err := os.Open(fileName)
	if err != nil {
		log.Fatalf("failed to open latency file: %v", err)
	}
	defer file.Close()

	var latencies []float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		val, err := strconv.ParseFloat(line, 64) // Parse as float64
		if err != nil {
			log.Printf("failed to parse line: %v", err)
			continue
		}
		latencies = append(latencies, val)
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("scanner error: %v", err)
	}

	if len(latencies) == 0 {
		fmt.Println("No latency samples found in file.")
		return
	}

	sort.Float64s(latencies) // sort slice
	n := len(latencies)

	min := latencies[0]
	max := latencies[n-1]

	var sum float64
	for _, v := range latencies {
		sum += v
	}
	avg := sum / float64(n)

	// Percentile function
	p := func(pct float64) float64 {
		pos := pct / 100.0 * float64(n-1)
		i := int(pos)
		if i >= n-1 {
			return latencies[n-1]
		}
		f := pos - float64(i)
		return latencies[i] + (latencies[i+1]-latencies[i])*f
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
	fmt.Printf("P99: %.3f ms\n", p(99))
	fmt.Printf("P99.9: %.3f ms\n", p(99.9))
	fmt.Printf("P99.99: %.3f ms\n", p(99.99))
	fmt.Println("----------------------------------------")
}

type Msg struct {
	ChannelName   string `json:"channel_name"`
	PublishedTime int64  `json:"published_time"`
	Data          []byte `json:"data"`
}

// DeserializeFrom implements services.SerializableMessage.
func (m *Msg) DeserializeFrom(b []byte) error {
	return json.Unmarshal(b, m)
}

// GetPublishedTime implements services.SerializableMessage.
func (m *Msg) GetPublishedTime() time.Time {
	return time.Unix(0, m.PublishedTime)
}

// Serialize implements services.SerializableMessage.
func (m *Msg) Serialize() ([]byte, error) {
	return json.Marshal(m)
}

func NewEmpty() *Msg {
	return &Msg{}
}
