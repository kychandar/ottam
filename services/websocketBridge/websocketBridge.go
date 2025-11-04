package websocketbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/ds"
	"github.com/kychandar/ottam/services"
)

func NewWsBridgeFactory() func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn, writerChannel <-chan []byte) services.WebSocketBridge {
	return func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn, writerChannel <-chan []byte) services.WebSocketBridge {
		return &websocketBridge{
			wsConnID:       wsConnID,
			conn:           conn,
			writerChannel:  writerChannel,
			centSubscriber: centSubscriber,
		}
	}
}

type websocketBridge struct {
	wsConnID       string
	conn           *websocket.Conn
	writerChannel  <-chan []byte
	centSubscriber services.CentralisedSubscriber
}

// ProcessMessagesFromServer implements services.WebSocketBridge.
func (w *websocketBridge) ProcessMessagesFromServer(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-w.writerChannel:
			err := w.conn.WriteMessage(websocket.BinaryMessage, msg)
			if err != nil {
				fmt.Println("error in writing ws msg:", err)
				return
			}
		}
	}
}

// ProcessMessagesFromClient implements services.WebSocketBridge.
func (w *websocketBridge) ProcessMessagesFromClient(ctx context.Context) {
	// TODO: chk how to handle error for below

	for {
		_, message, err := w.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected close error: %v", err)
			} else {
				log.Printf("read error: %v", err)
			}
			break
		}

		clientMessage := &ds.ClientMessage{}
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
			contrlPlaneMsg := &ds.ControlPlaneMessage{}
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

func (w *websocketBridge) HandleControlOp(op ds.ControlPlaneOp, data []byte) error {
	switch op {
	case ds.StartSubscription, ds.StopSubscription:
		return w.handleSubscriptionOp(op, data)

	default:
		return fmt.Errorf("unknown control op: %d", op)
	}
}

func (w *websocketBridge) handleSubscriptionOp(op ds.ControlPlaneOp, data []byte) error {
	subscriptonPayload := &ds.SubscriptionPayload{}
	err := json.Unmarshal(data, subscriptonPayload)
	if err != nil {
		return err
	}

	switch op {
	case ds.StartSubscription:
		return w.centSubscriber.Subscribe(context.TODO(), w.wsConnID, subscriptonPayload.ChannelName)
	case ds.StopSubscription:
		return w.centSubscriber.UnSubscribe(context.TODO(), w.wsConnID, subscriptonPayload.ChannelName)
	}
	// todo someting

	return nil
}
