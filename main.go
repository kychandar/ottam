package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/kychandar/ottam/services/valkey"
	wspooler "github.com/kychandar/ottam/services/wsPooler"
)

func main() {
	addr, exist := os.LookupEnv("VALKEY_ADDR")
	if !exist {
		panic("VALKEY_ADDR required")
	}

	go func() {
		// pprof listens on :6060 by default
		http.ListenAndServe("localhost:6060", nil)
	}()

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

func init() {
	runtime.SetMutexProfileFraction(1)
}
