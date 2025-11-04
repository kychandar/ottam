package ds

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/services"
)

type Msg struct {
	ChannelName   common.ChannelName `json:"channel_name"`
	PublishedTime int64              `json:"published_time"`
	Data          []byte             `json:"data"`
	MsgID         string             `json:"msg_id"`
}

// DeserializeFrom implements services.SerializableMessage.
func (m *Msg) DeserializeFrom(b []byte) error {
	return json.Unmarshal(b, m)
}

// GetChannelName implements services.SerializableMessage.
func (m *Msg) GetChannelName() common.ChannelName {
	return m.ChannelName
}

func (m *Msg) GetMsgID() string {
	return m.MsgID
}

// GetPublishedTime implements services.SerializableMessage.
func (m *Msg) GetPublishedTime() time.Time {
	return time.Unix(0, m.PublishedTime)
}

// Serialize implements services.SerializableMessage.
func (m *Msg) Serialize() ([]byte, error) {
	return json.Marshal(m)
}

func New(channelName common.ChannelName, data []byte) (services.SerializableMessage, error) {
	if len(channelName) < 1 {
		return nil, fmt.Errorf("invalid channel name")
	}
	return &Msg{
		ChannelName:   channelName,
		Data:          data,
		PublishedTime: time.Now().UnixNano(),
		MsgID:         uuid.New().String(),
	}, nil
}

func NewEmpty() services.SerializableMessage {
	return &Msg{}
}
