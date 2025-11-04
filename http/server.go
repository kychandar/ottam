package http

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/alphadose/haxmap"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/services"
)

type server struct {
	clietTracker    *haxmap.Map[string, chan<- []byte]
	centSubscriber  services.CentralisedSubscriber
	wsBridgeFactory func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn, writerChannel <-chan []byte) services.WebSocketBridge
}

func New(centSubscriber services.CentralisedSubscriber, wsBridgeFactory func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn, writerChannel <-chan []byte) services.WebSocketBridge) *server {
	return &server{
		clietTracker:    haxmap.New[string, chan<- []byte](),
		centSubscriber:  centSubscriber,
		wsBridgeFactory: wsBridgeFactory,
	}
}

func (server *server) Start() error {
	http.HandleFunc("/ws", server.ServeHTTP)

	fmt.Println("server started at http://:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		return err
	}
	return nil
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all connections (for demo)
	},
}

func (pooler *server) GetWriterChannelForClientID(clientID string) (chan<- []byte, bool) {
	return pooler.clietTracker.Get(clientID)
}

func (pooler *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Println("Client initiated!")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	writerChan := make(chan []byte, 150)
	wsConnID := uuid.New().String()

	pooler.clietTracker.Set(wsConnID, writerChan)
	ctx, cancel := context.WithCancel(r.Context())

	defer func() {
		cancel()
		pooler.centSubscriber.UnsubscribeAll(context.TODO(), wsConnID)
		pooler.clietTracker.Del(wsConnID)
		conn.Close()
	}()

	log.Println("Client connected!")
	bridge := pooler.wsBridgeFactory(pooler.centSubscriber, wsConnID, conn, writerChan)
	go bridge.ProcessMessagesFromServer(ctx)

	bridge.ProcessMessagesFromClient(ctx)
}
