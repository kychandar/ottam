package websocketbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/ds"
	"github.com/kychandar/ottam/services"
	"github.com/kychandar/ottam/services/pool"
)

func NewWsBridgeFactory() func(
	centSubscriber services.CentralisedSubscriber,
	wsConnID string, conn *websocket.Conn) services.WebSocketBridge {
	return func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn) services.WebSocketBridge {
		return &websocketBridge{
			wsConnID:       wsConnID,
			conn:           conn,
			centSubscriber: centSubscriber,
		}
	}
}

type websocketBridge struct {
	wsConnID       string
	conn           *websocket.Conn
	centSubscriber services.CentralisedSubscriber
}

// ProcessMessagesFromServer is no longer needed - messages go directly to worker pool from fanout workers
func (w *websocketBridge) ProcessMessagesFromServer(ctx context.Context) {
	// No-op: This method is kept to satisfy the interface but does nothing
	// Messages are now sent directly from fanout workers to the worker pool
	<-ctx.Done()
}

// ProcessMessagesFromClient implements services.WebSocketBridge.
func (w *websocketBridge) ProcessMessagesFromClient(ctx context.Context) {
	// TODO: chk how to handle error for below
	objPool := pool.GetGlobalPool()

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
		fmt.Println("read message from client")
		// Get ClientMessage from pool instead of allocating new
		clientMessage := objPool.ClientMessage.Get()
		err = json.Unmarshal(message, clientMessage)
		if err != nil {
			// TODO handle error
			objPool.ResetClientMessage(clientMessage)
			fmt.Println("error in decoding client message", err)
			continue
		}

		payload, err := json.Marshal(clientMessage.Payload)
		if err != nil {
			// TODO handle error
			objPool.ResetClientMessage(clientMessage)
			fmt.Println("error in decoding payload", err)
			continue
		}

		if clientMessage.IsControlPlaneMessage {
			// Get ControlPlaneMessage from pool instead of allocating new
			contrlPlaneMsg := objPool.ControlPlaneMsg.Get()
			err = json.Unmarshal(payload, contrlPlaneMsg)
			if err != nil {
				// TODO handle error
				objPool.ResetClientMessage(clientMessage)
				objPool.ResetControlPlaneMessage(contrlPlaneMsg)
				continue
			}

			payload, err := json.Marshal(contrlPlaneMsg.Payload)
			if err != nil {
				// TODO handle error
				objPool.ResetClientMessage(clientMessage)
				objPool.ResetControlPlaneMessage(contrlPlaneMsg)
				fmt.Println("error in decoding control plane payload", err)
				continue
			}

			err = w.HandleControlOp(contrlPlaneMsg.ControlPlaneOp, payload)
			if err != nil {
				// TODO handle error
				objPool.ResetClientMessage(clientMessage)
				objPool.ResetControlPlaneMessage(contrlPlaneMsg)
				continue
			}

			objPool.ResetClientMessage(clientMessage)
			objPool.ResetControlPlaneMessage(contrlPlaneMsg)
		} else {
			objPool.ResetClientMessage(clientMessage)
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
	objPool := pool.GetGlobalPool()
	subscriptonPayload := objPool.SubscriptionPayload.Get()
	err := json.Unmarshal(data, subscriptonPayload)
	if err != nil {
		objPool.ResetSubscriptionPayload(subscriptonPayload)
		return err
	}

	var result error
	switch op {
	case ds.StartSubscription:
		result = w.centSubscriber.Subscribe(context.TODO(), w.wsConnID, subscriptonPayload.ChannelName)
	case ds.StopSubscription:
		result = w.centSubscriber.UnSubscribe(context.TODO(), w.wsConnID, subscriptonPayload.ChannelName)
	}

	objPool.ResetSubscriptionPayload(subscriptonPayload)
	return result
}
