package services

import (
	"context"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/common"
)

type PubSubProvider interface {
	CreateOrUpdateStream(ctx context.Context, streamName string, subjects []string) error
	Publish(ctx context.Context, subjectName string, data []byte) error
	CreateOrUpdateConsumer(ctx context.Context, streamName, consumerName string, subjects []string) error
	Subscribe(ctx context.Context, streamName, consumerName string, callBack func(msg []byte) bool) error
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
	SubscriptionSyncer(ctx context.Context)
	ProcessDownstreamMessages(ctx context.Context) error
}

type WsWriteChanManager interface {
	GetConnectionForClientID(clientID string) (*websocket.Conn, bool)
	SetConnectionForClientID(clientID string, conn *websocket.Conn)
	DeleteClientID(clientID string)
}

type WorkerPool interface {
	SubmitMessageJob(ctx context.Context, conn *websocket.Conn, preparedMsg *websocket.PreparedMessage, publishedTime time.Time, msgID string, wsConnID string)
}
