package valkey

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const addr = "100.130.101.125:30079"

func TestPublish(t *testing.T) {
	conn := New(addr)
	err := conn.Publish(context.TODO(), "t1", []byte(time.Now().String()))

	assert.Nil(t, err)
}

func TestSubscribe(t *testing.T) {
	conn := New(addr)
	err := conn.Subscribe(context.TODO(), "t1", []byte(time.Now().String()))
	assert.Nil(t, err)
}
