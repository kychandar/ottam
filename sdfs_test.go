package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/kychandar/ottam/ds"
	pubSubProvider "github.com/kychandar/ottam/services/pubsub/nats"
)

func TestXX(t *testing.T) {
	pubSub, err := pubSubProvider.NewNatsPubSub("100.130.101.125:30001")
	if err != nil {
		panic(err)
	}
	defer pubSub.Close()

	ticker := time.NewTicker(2 * time.Millisecond)
	interrupt := make(chan os.Signal, 1)

	for {
		select {
		case <-interrupt:
			fmt.Printf("Stopping ")
		case <-ticker.C:
			msg, _ := ds.New("t1", []byte("hi"))
			msgByte, err := msg.Serialize()
			if err != nil {
				panic(err)
			}
			err = pubSub.Publish("centralProcessor", msgByte)
			if err != nil {
				panic(err)
			}
		}
	}

}
