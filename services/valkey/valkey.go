package valkey

import (
	"context"
	"fmt"

	"github.com/kychandar/ottam/services"
	"github.com/valkey-io/valkey-go"
)

type ValKeyConnection struct {
	client valkey.Client
}

func New(addr string) (services.PubSubProvider, error) {
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}})
	if err != nil {
		return nil, err
	}
	return &ValKeyConnection{
		client: client,
	}, nil
}

func (conn *ValKeyConnection) Publish(ctx context.Context, subject string, data []byte) error {
	err := conn.client.Do(ctx, conn.client.B().Publish().Channel(subject).Message(string(data)).Build()).Error()
	if err != nil {
		return err
	}
	return nil
}

func (conn *ValKeyConnection) Subscribe(ctx context.Context, subject string, callBack services.SubscriptionCallBack) (func() error, error) {
	cmd := conn.client.B().Subscribe().Channel(subject).Build().Pin()
	err := conn.client.Do(ctx, cmd).Error()
	if err != nil {
		return nil, err
	}

	go func() {
		err = conn.client.Receive(ctx, cmd, func(msg valkey.PubSubMessage) {
			callBack(ctx, subject, []byte(fmt.Sprintf("%s:%s", msg.Channel, msg.Message)))
		})
		if err != nil {
			panic(err)
		}
	}()
	return func() error {
		return conn.client.Do(ctx, conn.client.B().Unsubscribe().Channel(subject).Build()).Error()
	}, nil
}
