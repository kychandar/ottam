package services

import (
	"context"
	"time"

	"github.com/kychandar/ottam/common"
)

type PubSubProvider interface {
	CreateStream(streamName string, subjects []string) error
	Publish(subjectName string, data []byte) error
	Subscribe(consumerName string, subjectName string, callBack func(msg []byte) bool) error
	UnSubscribe(consumerName string) error
	Close() error
}

type DataStore interface {
	AddNodeSubscriptionForChannel(ctx context.Context, channelName common.ChannelName, nodeID common.NodeID) error
	RemoveNodeSubscriptionForChannel(ctx context.Context, channelName common.ChannelName, nodeID common.NodeID) error
	ListNodesSubscribedForChannel(ctx context.Context, channelName common.ChannelName) ([]common.NodeID, error)
	Close()
}

type CentralProcessor interface {
	Start(ctx context.Context) error
	Stop(context.Context) error
}

type SerializableMessage interface {
	Serialize() ([]byte, error)
	DeserializeFrom([]byte) error
	GetChannelName() common.ChannelName
	GetPublishedTime() time.Time
	GetMsgID() string
}

type WebSocketBridge interface {
	ProcessMessagesFromServer(ctx context.Context)
	ProcessMessagesFromClient(ctx context.Context)
}

type CentralisedSubscriber interface {
	Subscribe(ctx context.Context, clientID string, channelName common.ChannelName) error
	UnSubscribe(ctx context.Context, clientID string, channelName common.ChannelName) error
	UnsubscribeAll(ctx context.Context, clientID string) error

	ProcessDownstreamMessages(ctx context.Context) error
}

type WsWriteChanManager interface {
	GetWriterChannelForClientID(clientID string) (chan<- common.IntermittenMsg, bool)
	SetWriterChannelForClientID(clientID string, writerChan chan<- common.IntermittenMsg)
	DeleteClientID(clientID string)
}
