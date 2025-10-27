package main

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ------------------ Server ------------------
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(1 * time.Millisecond) // send every 10ms
	defer ticker.Stop()

	for range ticker.C {
		timestamp := time.Now().UnixNano()
		err := conn.WriteMessage(websocket.TextMessage, []byte(strconv.FormatInt(timestamp, 10)))
		if err != nil {
			return
		}
	}
}

// ------------------ Client ------------------
func wsClient(id int, url string, duration time.Duration, wg *sync.WaitGroup, latenciesCh chan<- float64) {
	defer wg.Done()

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Println("Client", id, "dial error:", err)
		return
	}
	defer conn.Close()

	end := time.Now().Add(duration)

	for time.Now().Before(end) {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}
		sentNano, err := strconv.ParseInt(string(msg), 10, 64)
		if err != nil {
			continue
		}
		latency := float64(time.Now().UnixNano()-sentNano) / 1e6 // ms
		latenciesCh <- latency
	}
}

// ------------------ Main ------------------

func TestWSLatency(t *testing.T) {
	nClients := 100
	testDuration := 10 * time.Second

	// Start server
	http.HandleFunc("/ws", wsHandler)
	go func() {
		log.Println("Starting WS server on :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	time.Sleep(100 * time.Millisecond) // Give server time to start

	// Channel to collect latencies from all clients
	latenciesCh := make(chan float64, nClients*10000) // buffered channel

	wg := sync.WaitGroup{}
	for i := 1; i <= nClients; i++ {
		wg.Add(1)
		go wsClient(i, "ws://localhost:8080/ws", testDuration, &wg, latenciesCh)
	}

	// Wait for clients to finish
	go func() {
		wg.Wait()
		close(latenciesCh)
	}()

	// Collect all latencies
	allLatencies := []float64{}
	for l := range latenciesCh {
		allLatencies = append(allLatencies, l)
	}

	// Calculate summary
	if len(allLatencies) == 0 {
		log.Println("No latencies recorded")
		return
	}

	sort.Float64s(allLatencies)
	min := allLatencies[0]
	max := allLatencies[len(allLatencies)-1]
	sum := 0.0
	for _, l := range allLatencies {
		sum += l
	}
	avg := sum / float64(len(allLatencies))
	getPercentile := func(p float64) float64 {
		k := int(float64(len(allLatencies)-1) * p / 100.0)
		return allLatencies[k]
	}

	fmt.Println("-------- Global Latency Summary --------")
	fmt.Printf("Clients: %d | Duration: %v\n", nClients, testDuration)
	fmt.Printf("Total messages received: %d\n", len(allLatencies))
	fmt.Printf("Min latency: %.3f ms\n", min)
	fmt.Printf("Max latency: %.3f ms\n", max)
	fmt.Printf("Avg latency: %.3f ms\n", avg)
	fmt.Printf("P90 latency: %.3f ms\n", getPercentile(90))
	fmt.Printf("P95 latency: %.3f ms\n", getPercentile(95))
	fmt.Printf("P99 latency: %.3f ms\n", getPercentile(99))
}
