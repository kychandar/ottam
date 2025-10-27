package wsbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gammazero/deque"
	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/services"
)

type wsBridge struct {
	wsConnID       string
	mu             sync.Mutex
	conn           *websocket.Conn
	publishBuffer  deque.Deque[[]byte]
	st             chan struct{}
	centSubscriber services.CentralisedSubscriber
}

func New(wsConnID string, conn *websocket.Conn, centSubscriber services.CentralisedSubscriber) (services.WSPubSubBridge, error) {
	return &wsBridge{
		wsConnID:       wsConnID,
		conn:           conn,
		publishBuffer:  deque.Deque[[]byte]{},
		st:             make(chan struct{}, 1),
		centSubscriber: centSubscriber,
	}, nil
}

// ProcessMessagesFromServer implements services.WSPubSubBridge.
func (w *wsBridge) ProcessMessagesFromServer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.st:
			for w.publishBuffer.Len() > 0 {
				cur := w.publishBuffer.Front()
				err := w.conn.WriteMessage(websocket.BinaryMessage, cur)
				if err == nil {
					w.mu.Lock()
					w.publishBuffer.PopFront()
					w.mu.Unlock()
				} else {
					fmt.Println("error in writing ws msg", err)
				}
			}
		}
	}
}

// ProcessMessagesFromClient implements services.WSPubSubBridge.
func (w *wsBridge) ProcessMessagesFromClient(ctx context.Context) {
	for {
		// mt, message, err := conn.ReadMessage()
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			fmt.Println("Read error:", err)
			continue
		}

		clientMessage := &ClientMessage{}
		err = json.Unmarshal(message, clientMessage)
		if err != nil {
			// TODO handle error
			fmt.Println("error in decoding client message", err)
			continue
		}

		payload, err := json.Marshal(clientMessage.Payload)
		if err != nil {
			// TODO handle error
			fmt.Println("error in decoding payload", err)
			continue
		}

		if clientMessage.IsControlPlaneMessage {
			contrlPlaneMsg := &ControlPlaneMessage{}
			err = json.Unmarshal(payload, contrlPlaneMsg)
			if err != nil {
				// TODO handle error
				continue
			}

			payload, err := json.Marshal(contrlPlaneMsg.Payload)
			if err != nil {
				// TODO handle error
				fmt.Println("error in decoding control plane payload", err)
				continue
			}

			err = w.HandleControlOp(contrlPlaneMsg.ControlPlaneOp, payload)
			if err != nil {
				// TODO handle error
				continue
			}
		} else {

		}

	}
}

// AsyncWriteMessageToClient implements services.WSPubSubBridge.
func (w *wsBridge) AsyncWriteMessageToClient(ctx context.Context, subject string, data []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.publishBuffer.PushBack(data)
	select {
	case w.st <- struct{}{}:
	default:
	}
}
