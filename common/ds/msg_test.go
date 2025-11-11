package ds

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/kychandar/ottam/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMsg_Serialize(t *testing.T) {
	t.Run("serialize valid message", func(t *testing.T) {
		msg := &Msg{
			ChannelName:   "test-channel",
			PublishedTime: time.Now().UnixNano(),
			Data:          []byte("test data"),
			MsgID:         "msg-123",
		}

		data, err := msg.Serialize()
		assert.NoError(t, err)
		assert.NotNil(t, data)

		// Verify it's valid JSON
		var parsed map[string]interface{}
		err = json.Unmarshal(data, &parsed)
		assert.NoError(t, err)
		assert.Equal(t, "test-channel", parsed["channel_name"])
		assert.Equal(t, "msg-123", parsed["msg_id"])
	})

	t.Run("serialize message with empty data", func(t *testing.T) {
		msg := &Msg{
			ChannelName:   "channel",
			PublishedTime: 123456,
			Data:          []byte{},
			MsgID:         "id",
		}

		data, err := msg.Serialize()
		assert.NoError(t, err)
		assert.NotNil(t, data)
	})

	t.Run("serialize message with nil data", func(t *testing.T) {
		msg := &Msg{
			ChannelName:   "channel",
			PublishedTime: 123456,
			Data:          nil,
			MsgID:         "id",
		}

		data, err := msg.Serialize()
		assert.NoError(t, err)
		assert.NotNil(t, data)
	})
}

func TestMsg_DeserializeFrom(t *testing.T) {
	t.Run("deserialize valid message", func(t *testing.T) {
		jsonData := `{
			"channel_name": "test-channel",
			"published_time": 1234567890,
			"data": "dGVzdCBkYXRh",
			"msg_id": "msg-456"
		}`

		msg := &Msg{}
		err := msg.DeserializeFrom([]byte(jsonData))
		assert.NoError(t, err)
		assert.Equal(t, common.ChannelName("test-channel"), msg.ChannelName)
		assert.Equal(t, int64(1234567890), msg.PublishedTime)
		assert.Equal(t, "msg-456", msg.MsgID)
	})

	t.Run("deserialize invalid JSON", func(t *testing.T) {
		invalidJSON := `{invalid json`

		msg := &Msg{}
		err := msg.DeserializeFrom([]byte(invalidJSON))
		assert.Error(t, err)
	})

	t.Run("deserialize empty JSON object", func(t *testing.T) {
		jsonData := `{}`

		msg := &Msg{}
		err := msg.DeserializeFrom([]byte(jsonData))
		assert.NoError(t, err)
		assert.Equal(t, common.ChannelName(""), msg.ChannelName)
	})

	t.Run("roundtrip serialize and deserialize", func(t *testing.T) {
		original := &Msg{
			ChannelName:   "roundtrip-channel",
			PublishedTime: time.Now().UnixNano(),
			Data:          []byte("roundtrip data"),
			MsgID:         "roundtrip-id",
		}

		// Serialize
		serialized, err := original.Serialize()
		require.NoError(t, err)

		// Deserialize
		deserialized := &Msg{}
		err = deserialized.DeserializeFrom(serialized)
		require.NoError(t, err)

		// Verify equality
		assert.Equal(t, original.ChannelName, deserialized.ChannelName)
		assert.Equal(t, original.PublishedTime, deserialized.PublishedTime)
		assert.Equal(t, original.Data, deserialized.Data)
		assert.Equal(t, original.MsgID, deserialized.MsgID)
	})
}

func TestMsg_GetChannelName(t *testing.T) {
	t.Run("get channel name", func(t *testing.T) {
		msg := &Msg{ChannelName: "my-channel"}
		assert.Equal(t, common.ChannelName("my-channel"), msg.GetChannelName())
	})

	t.Run("get empty channel name", func(t *testing.T) {
		msg := &Msg{ChannelName: ""}
		assert.Equal(t, common.ChannelName(""), msg.GetChannelName())
	})
}

func TestMsg_GetMsgID(t *testing.T) {
	t.Run("get message ID", func(t *testing.T) {
		msg := &Msg{MsgID: "unique-id-123"}
		assert.Equal(t, "unique-id-123", msg.GetMsgID())
	})

	t.Run("get empty message ID", func(t *testing.T) {
		msg := &Msg{MsgID: ""}
		assert.Equal(t, "", msg.GetMsgID())
	})
}

func TestMsg_GetPublishedTime(t *testing.T) {
	t.Run("get published time", func(t *testing.T) {
		now := time.Now()
		nanoTime := now.UnixNano()
		
		msg := &Msg{PublishedTime: nanoTime}
		publishedTime := msg.GetPublishedTime()
		
		assert.Equal(t, nanoTime, publishedTime.UnixNano())
		assert.True(t, now.Sub(publishedTime) < time.Second)
	})

	t.Run("get zero published time", func(t *testing.T) {
		msg := &Msg{PublishedTime: 0}
		publishedTime := msg.GetPublishedTime()
		
		assert.Equal(t, time.Unix(0, 0), publishedTime)
	})
}

func TestNew(t *testing.T) {
	t.Run("create new message", func(t *testing.T) {
		channelName := common.ChannelName("test-channel")
		data := []byte("test data")

		msg, err := New(channelName, data)
		require.NoError(t, err)
		require.NotNil(t, msg)

		typedMsg := msg.(*Msg)
		assert.Equal(t, channelName, typedMsg.ChannelName)
		assert.Equal(t, data, typedMsg.Data)
		assert.NotZero(t, typedMsg.PublishedTime)
		assert.NotEmpty(t, typedMsg.MsgID)
	})

	t.Run("create message with empty data", func(t *testing.T) {
		channelName := common.ChannelName("channel")
		data := []byte{}

		msg, err := New(channelName, data)
		require.NoError(t, err)
		require.NotNil(t, msg)

		typedMsg := msg.(*Msg)
		assert.Equal(t, channelName, typedMsg.ChannelName)
		assert.Equal(t, data, typedMsg.Data)
	})

	t.Run("create message with nil data", func(t *testing.T) {
		channelName := common.ChannelName("channel")

		msg, err := New(channelName, nil)
		require.NoError(t, err)
		require.NotNil(t, msg)

		typedMsg := msg.(*Msg)
		assert.Equal(t, channelName, typedMsg.ChannelName)
		assert.Nil(t, typedMsg.Data)
	})

	t.Run("create message with empty channel name", func(t *testing.T) {
		channelName := common.ChannelName("")
		data := []byte("data")

		msg, err := New(channelName, data)
		assert.Error(t, err)
		assert.Nil(t, msg)
		assert.Contains(t, err.Error(), "invalid channel name")
	})

	t.Run("message ID is unique", func(t *testing.T) {
		msg1, _ := New("channel1", []byte("data1"))
		msg2, _ := New("channel2", []byte("data2"))

		id1 := msg1.(*Msg).MsgID
		id2 := msg2.(*Msg).MsgID

		assert.NotEqual(t, id1, id2)
		assert.NotEmpty(t, id1)
		assert.NotEmpty(t, id2)
	})

	t.Run("published time is recent", func(t *testing.T) {
		before := time.Now()
		msg, _ := New("channel", []byte("data"))
		after := time.Now()

		publishedTime := msg.(*Msg).GetPublishedTime()
		assert.True(t, publishedTime.After(before) || publishedTime.Equal(before))
		assert.True(t, publishedTime.Before(after) || publishedTime.Equal(after))
	})
}

func TestNewEmpty(t *testing.T) {
	t.Run("create empty message", func(t *testing.T) {
		msg := NewEmpty()
		assert.NotNil(t, msg)

		typedMsg := msg.(*Msg)
		assert.Equal(t, common.ChannelName(""), typedMsg.ChannelName)
		assert.Nil(t, typedMsg.Data)
		assert.Equal(t, int64(0), typedMsg.PublishedTime)
		assert.Equal(t, "", typedMsg.MsgID)
	})

	t.Run("empty message can be populated", func(t *testing.T) {
		msg := NewEmpty()
		typedMsg := msg.(*Msg)

		typedMsg.ChannelName = "populated-channel"
		typedMsg.Data = []byte("populated data")
		typedMsg.PublishedTime = time.Now().UnixNano()
		typedMsg.MsgID = "populated-id"

		assert.Equal(t, common.ChannelName("populated-channel"), typedMsg.ChannelName)
		assert.Equal(t, []byte("populated data"), typedMsg.Data)
		assert.NotZero(t, typedMsg.PublishedTime)
		assert.Equal(t, "populated-id", typedMsg.MsgID)
	})

	t.Run("empty message implements SerializableMessage", func(t *testing.T) {
		msg := NewEmpty()
		
		// Should be able to call all interface methods
		_ = msg.GetChannelName()
		_ = msg.GetPublishedTime()
		_ = msg.GetMsgID()
		_, _ = msg.Serialize()
		_ = msg.DeserializeFrom([]byte("{}"))
	})
}

func TestMsg_InterfaceCompliance(t *testing.T) {
	t.Run("Msg implements SerializableMessage", func(t *testing.T) {
		msg, err := New("channel", []byte("data"))
		require.NoError(t, err)

		// Verify all interface methods are callable
		channelName := msg.GetChannelName()
		assert.NotEmpty(t, channelName)

		publishedTime := msg.GetPublishedTime()
		assert.False(t, publishedTime.IsZero())

		msgID := msg.GetMsgID()
		assert.NotEmpty(t, msgID)

		serialized, err := msg.Serialize()
		assert.NoError(t, err)
		assert.NotNil(t, serialized)

		err = msg.DeserializeFrom(serialized)
		assert.NoError(t, err)
	})
}

func TestMsg_LargeData(t *testing.T) {
	t.Run("message with large data", func(t *testing.T) {
		// Create 1MB of data
		largeData := make([]byte, 1024*1024)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		msg, err := New("large-channel", largeData)
		require.NoError(t, err)

		// Serialize and deserialize
		serialized, err := msg.Serialize()
		require.NoError(t, err)

		deserialized := NewEmpty()
		err = deserialized.DeserializeFrom(serialized)
		require.NoError(t, err)

		typedMsg := deserialized.(*Msg)
		assert.Equal(t, largeData, typedMsg.Data)
	})
}

