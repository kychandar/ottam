package websocketbridge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/ds"
	"github.com/kychandar/ottam/metrics"
	"github.com/kychandar/ottam/services"
	"github.com/kychandar/ottam/services/pool"
	slogctx "github.com/veqryn/slog-context"
)

func NewWsBridgeFactory() func(
	centSubscriber services.CentralisedSubscriber,
	wsConnID string, conn *websocket.Conn,
	writerChannel <-chan common.IntermittenMsg) services.WebSocketBridge {
	return func(centSubscriber services.CentralisedSubscriber, wsConnID string, conn *websocket.Conn, writerChannel <-chan common.IntermittenMsg) services.WebSocketBridge {
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
	writerChannel  <-chan common.IntermittenMsg
	centSubscriber services.CentralisedSubscriber
}

// ProcessMessagesFromServer implements services.WebSocketBridge.
func (w *websocketBridge) ProcessMessagesFromServer(ctx context.Context) {
	logger := slogctx.FromCtx(ctx).With("comoponent", "wsBridge", "cliend-id", w.wsConnID)
	for {
		select {
		case <-ctx.Done():
			return
		case intMsg := <-w.writerChannel:
			logger := logger.With("msg-id", intMsg.Id)
			logger.InfoContext(ctx, "trying to write msg", "ms", time.Since(intMsg.PublishedTime).String())
			metrics.LatencyHist.WithLabelValues("ws_bridge_ws_msg_write_start").Observe(float64(time.Since(intMsg.PublishedTime).Milliseconds()))
			err := w.conn.WritePreparedMessage(intMsg.PreparedMessage)
			// err := w.conn.WriteMessage(websocket.BinaryMessage, intMsg.Data)
			if err != nil {
				fmt.Println("error in writing ws msg:", err)
				return
			}
			metrics.LatencyHist.WithLabelValues("ws_bridge_ws_msg_write_done").Observe(float64(time.Since(intMsg.PublishedTime).Milliseconds()))
			logger.InfoContext(ctx, "msg written to ws", "ms", time.Since(intMsg.PublishedTime).String())
		}
	}
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
