package tests

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

const url = "ws://ottam-http-server.ottam.svc:8080/ws"

var (
	msgBytes []byte
)

func TestSub(t *testing.T) {
	fmt.Println("Connecting to:", url)

	noOfClients := 20000
	testDuration := 30 * time.Second
	msgPublishedEvery := 200 * time.Millisecond

	maxPossibleMessages := int(testDuration/msgPublishedEvery) * noOfClients

	fmt.Println("noOfClients", noOfClients)
	fmt.Println("testDuration", testDuration.String())
	fmt.Println("msgPublishedEvery", msgPublishedEvery.String())
	fmt.Println("maxPossibleMessages", maxPossibleMessages)

	maxSafeBufferRequired := maxPossibleMessages * 2

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
	latencyCh := make(chan float64, maxSafeBufferRequired)
	const fileName = "latencies.txt"
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	latencies := make([]float64, 0, maxSafeBufferRequired)

	go func() {
		for latency := range latencyCh {
			// fmt.Fprintf(file, "%.6f\n", latency)
			latencies = append(latencies, latency)
		}
	}()

	wg := &sync.WaitGroup{}

	ctx, cancel := context.WithCancel(context.TODO())

	for i := 0; i < noOfClients; i++ {
		go wsConn(ctx, wg, latencyCh)
	}

	ticker := time.NewTicker(testDuration)
	select {
	case <-interrupt:
		fmt.Printf("Stopping ")
		cancel()
	case <-ticker.C:
		// close(done)
		fmt.Printf("Stopping after %s\n", testDuration.String())
		cancel()
	}

	wg.Wait()
	// printLatencyStatsFromFile(fileName)
	printLatencyStatsFromSlice(latencies)
}

func wsConn(ctx context.Context, wg *sync.WaitGroup, writerChan chan<- float64) {
	wg.Add(1)
	defer wg.Done()
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer conn.Close()
	err = conn.WriteMessage(websocket.TextMessage, msgBytes)
	if err != nil {
		log.Println("write error:", err)
		return
	}
	readTimeout := 30 * time.Second

	conn.SetReadDeadline(time.Now().Add(readTimeout))
	conn.SetPongHandler(func(string) error {
		// extend deadline every time we get a Pong
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			conn.Close()
			return

		default:
			_, msg, err := conn.ReadMessage()
			if err != nil {
				// log.Println("read error:", err)
				return
			}

			m := NewEmpty()
			if err := json.Unmarshal(msg, &m); err != nil {
				log.Println("unmarshal error:", err)
				continue
			}

			latencyNs := time.Since(m.GetPublishedTime()).Nanoseconds()
			latencyMs := float64(latencyNs) / 1e6
			select {
			case writerChan <- latencyMs:
			default:
			}
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
	printLatencyStatsFromSlice(latencies)
}

func printLatencyStatsFromSlice(latencies []float64) {
	fmt.Print(returnStats(latencies))
}

func returnStats(latencies []float64) string {
	if len(latencies) == 0 {
		return "No latency samples found in file."
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

	summary := "--------- Latency Summary (ms) ---------\n" +
		fmt.Sprintf("Samples: %d\n", n) +
		fmt.Sprintf("Samples: %s\n", formatSamples(n)) +
		fmt.Sprintf("Min: %.3f ms\n", min) +
		fmt.Sprintf("Max: %.3f ms\n", max) +
		fmt.Sprintf("Avg: %.3f ms\n", avg) +
		fmt.Sprintf("P70: %.3f ms\n", p(70)) +
		fmt.Sprintf("P75: %.3f ms\n", p(75)) +
		fmt.Sprintf("P80: %.3f ms\n", p(80)) +
		fmt.Sprintf("P85: %.3f ms\n", p(85)) +
		fmt.Sprintf("P90: %.3f ms\n", p(90)) +
		fmt.Sprintf("P95: %.3f ms\n", p(95)) +
		fmt.Sprintf("P99: %.3f ms\n", p(99)) +
		fmt.Sprintf("P99.9: %.3f ms\n", p(99.9)) +
		fmt.Sprintf("P99.99: %.3f ms\n", p(99.99)) +
		"----------------------------------------\n"

	return summary
}

func formatSamples(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.2f M samples", float64(n)/1_000_000)
	} else if n >= 1_000 {
		return fmt.Sprintf("%.2f k samples", float64(n)/1_000)
	}
	return fmt.Sprintf("%d samples", n)
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
