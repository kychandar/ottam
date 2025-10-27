package wspooler

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/services"
	centralisedsubscriber "github.com/kychandar/ottam/services/centralisedSubscriber"
	wsbridge "github.com/kychandar/ottam/services/wsBridge"
)

// Define an upgrader to upgrade HTTP connections to WebSocket
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all connections (for demo)
	},
}

func (pooler *Pooler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP to WebSocket
	log.Println("Client initiated!")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	log.Println("Client connected!")

	wsConnID := uuid.New().String()
	bridge, err := wsbridge.New(wsConnID, conn, pooler.centSubscriber)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	ctx := r.Context()
	go func() {
		bridge.ProcessMessagesFromServer(ctx)
	}()

	bridge.ProcessMessagesFromClient(ctx)
}

type Pooler struct {
	conn           services.PubSubProvider
	centSubscriber services.CentralisedSubscriber
}

func New(ctx context.Context, conn services.PubSubProvider) *Pooler {
	return &Pooler{
		conn:           conn,
		centSubscriber: centralisedsubscriber.New(ctx, conn),
	}
}

func (pooler *Pooler) Start() {
	http.HandleFunc("/ws", pooler.handleWebSocket)

	fmt.Println("Server started at http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(err)
	}
}
