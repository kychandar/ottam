package ds

import "github.com/kychandar/ottam/common"

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
	ChannelName common.ChannelName
}
