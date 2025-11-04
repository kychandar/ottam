package http

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/services"
	slogctx "github.com/veqryn/slog-context"
)

type server struct {
	wsWriteChanManager services.WsWriteChanManager
	centSubscriber     services.CentralisedSubscriber
	wsBridgeFactory    func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn, writerChannel <-chan common.IntermittenMsg) services.WebSocketBridge
	logger             *slog.Logger
}

func New(
	centSubscriber services.CentralisedSubscriber,
	wsBridgeFactory func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn, writerChannel <-chan common.IntermittenMsg) services.WebSocketBridge,
	wsWriteChanManager services.WsWriteChanManager,
	logger *slog.Logger) *server {
	return &server{
		centSubscriber:     centSubscriber,
		wsBridgeFactory:    wsBridgeFactory,
		wsWriteChanManager: wsWriteChanManager,
		logger:             logger,
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

func (pooler *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	writerChan := make(chan common.IntermittenMsg, 15000)
	wsConnID := uuid.New().String()

	pooler.wsWriteChanManager.SetWriterChannelForClientID(wsConnID, writerChan)
	ctx, cancel := context.WithCancel(r.Context())
	ctx = slogctx.NewCtx(ctx, pooler.logger)
	defer func() {
		cancel()
		pooler.centSubscriber.UnsubscribeAll(context.TODO(), wsConnID)
		pooler.wsWriteChanManager.DeleteClientID(wsConnID)
		conn.Close()
	}()

	bridge := pooler.wsBridgeFactory(pooler.centSubscriber, wsConnID, conn, writerChan)
	go bridge.ProcessMessagesFromServer(ctx)

	bridge.ProcessMessagesFromClient(ctx)
}
