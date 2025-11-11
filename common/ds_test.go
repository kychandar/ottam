package common

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestIntermittenMsg(t *testing.T) {
	t.Run("create IntermittenMsg", func(t *testing.T) {
		now := time.Now()
		msgID := "msg-123"
		pm, _ := websocket.NewPreparedMessage(websocket.BinaryMessage, []byte("test"))

		msg := IntermittenMsg{
			PublishedTime:   now,
			Id:              msgID,
			PreparedMessage: pm,
		}

		assert.Equal(t, now, msg.PublishedTime)
		assert.Equal(t, msgID, msg.Id)
		assert.NotNil(t, msg.PreparedMessage)
	})

	t.Run("zero value IntermittenMsg", func(t *testing.T) {
		var msg IntermittenMsg
		assert.True(t, msg.PublishedTime.IsZero())
		assert.Equal(t, "", msg.Id)
		assert.Nil(t, msg.PreparedMessage)
	})

	t.Run("update IntermittenMsg fields", func(t *testing.T) {
		msg := IntermittenMsg{}
		
		newTime := time.Now()
		msg.PublishedTime = newTime
		assert.Equal(t, newTime, msg.PublishedTime)

		msg.Id = "updated-id"
		assert.Equal(t, "updated-id", msg.Id)

		pm, _ := websocket.NewPreparedMessage(websocket.TextMessage, []byte("data"))
		msg.PreparedMessage = pm
		assert.NotNil(t, msg.PreparedMessage)
	})
}

