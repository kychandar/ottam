package services

// fireAndForget pubSub service
import (
	"context"
)

type PubSubProvider interface {
	Publish(ctx context.Context, subject string, data []byte) error
	Subscribe(ctx context.Context, subject string, callBack SubscriptionCallBack) (func() error, error)
}

type WSPubSubBridge interface {
	ProcessMessagesFromClient(ctx context.Context)
	ProcessMessagesFromServer(ctx context.Context)
	AsyncWriteMessageToClient(ctx context.Context, subject string, data []byte)
}

type SubscriptionCallBack = func(ctx context.Context, subject string, data []byte)

type CentralisedSubscriber interface {
	Subscribe(ctx context.Context, subject string, id string, callBack SubscriptionCallBack) error
	Unsubscribe(ctx context.Context, subject string, id string) error
}
