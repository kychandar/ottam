package wsbridge

import (
	"context"
	"encoding/json"
	"fmt"
)

type ClientMessage struct {
	Version uint32

	Id                    string
	IsControlPlaneMessage bool

	Payload any
}

type ControlPlaneOp uint32

const (
	StartSubscription ControlPlaneOp = iota
	StopSubscription
)

type ControlPlaneMessage struct {
	Version uint32

	ControlPlaneOp ControlPlaneOp
	Payload        any
}

type SubscriptionPayload struct {
	ChannelName string
}

func (w *wsBridge) HandleControlOp(op ControlPlaneOp, data []byte) error {
	switch op {
	case StartSubscription, StopSubscription:
		return w.handleSubscriptionOp(op, data)

	default:
		return fmt.Errorf("unknown control op: %d", op)
	}
}

func (w *wsBridge) handleSubscriptionOp(op ControlPlaneOp, data []byte) error {
	fmt.Println(string(data))
	subscriptonPayload := &SubscriptionPayload{}
	err := json.Unmarshal(data, subscriptonPayload)
	if err != nil {
		return err
	}
	fmt.Println(subscriptonPayload)

	switch op {
	case StartSubscription:
		w.centSubscriber.Subscribe(context.TODO(), subscriptonPayload.ChannelName, w.wsConnID, w.AsyncWriteMessageToClient)
	case StopSubscription:
		w.centSubscriber.Unsubscribe(context.TODO(), subscriptonPayload.ChannelName, w.wsConnID)
	}
	// todo someting

	return nil
}
