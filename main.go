package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/kychandar/ottam/services/valkey"
	wspooler "github.com/kychandar/ottam/services/wsPooler"
)

func main() {
	addr, exist := os.LookupEnv("VALKEY_ADDR")
	if !exist {
		panic("VALKEY_ADDR required")
	}

	conn, _ := valkey.New(addr)
	go func() {
		ticker := time.NewTicker(1 * time.Millisecond)
		for {
			select {
			case <-ticker.C:
				conn.Publish(context.TODO(), "t1", []byte(fmt.Sprintf("%v", time.Now().UnixNano())))
			}
		}
	}()
	pooler := wspooler.New(context.TODO(), conn)
	pooler.Start()
}
