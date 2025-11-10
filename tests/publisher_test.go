package tests

import (
	"context"
	"testing"
	"time"

	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/ds"
	pubSubProvider "github.com/kychandar/ottam/services/pubsub/nats"
)

func TestPub(t *testing.T) {
	pubSub, err := pubSubProvider.NewNatsPubSub("nats1:4222")
	if err != nil {
		panic(err)
	}
	defer pubSub.Close()

	ticker1 := time.NewTicker(200 * time.Millisecond)
	for {
		select {
		case <-ticker1.C:
			msg, _ := ds.New("t1", []byte("hi"))
			msgByte, err := msg.Serialize()
			if err != nil {
				panic(err)
			}

			err = pubSub.Publish(context.TODO(), common.ChannelSubjFormat(string(msg.GetChannelName())), msgByte)
			if err != nil {
				panic(err)
			}
		}
	}
}
