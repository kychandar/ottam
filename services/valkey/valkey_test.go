package valkey

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPubSub(t *testing.T) {
	addr, exist := os.LookupEnv("VALKEY_ADDR")
	if !exist {
		t.Error("VALKEY_ADDR required")
		t.FailNow()
	}

	conn, _ := New(addr)
	wait := make(chan struct{})

	const (
		subj = "t1"
	)
	msg := time.Now().String()
	_, err := conn.Subscribe(context.TODO(), subj, func(ctx context.Context, subject string, data []byte) {
		assert.Equal(t, fmt.Sprintf("%s:%s", subj, msg), string(data))
		wait <- struct{}{}
	})
	assert.Nil(t, err)
	err = conn.Publish(context.TODO(), subj, []byte(msg))
	assert.Nil(t, err)
	<-wait
}
