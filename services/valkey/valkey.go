package valkey

import (
	"context"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

type ValKeyConnection struct {
	// addr:=[]string{"100.130.101.125:30079"}
	// addr   []string
	client valkey.Client
}

func New(addr string) *ValKeyConnection {
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{addr}})
	if err != nil {
		panic(err)
	}
	return &ValKeyConnection{
		client: client,
	}
}

func (conn *ValKeyConnection) Publish(ctx context.Context, subject string, data []byte) error {
	start := time.Now()
	err := conn.client.Do(ctx, conn.client.B().Publish().Channel(subject).Message(string(data)).Build()).Error()
	if err != nil {
		return err
	}
	fmt.Println("completed in", time.Since(start))
	return nil
}

func (conn *ValKeyConnection) Subscribe(ctx context.Context, subject string, data []byte) error {
	go func() {
		err := conn.client.Receive(ctx, conn.client.B().Subscribe().Channel("t1").Build(), func(msg valkey.PubSubMessage) {
			fmt.Println(msg.Channel, msg.Message)
		})
		if err != nil {
			panic(err)
		}

		fmt.Println("subscription closed")
	}()

	time.Sleep(5 * time.Second)

	err := conn.client.Do(ctx, conn.client.B().Unsubscribe().Channel("t1").Build()).Error()
	if err != nil {
		return nil
	}

	time.Sleep(1 * time.Second)

	return nil
}
