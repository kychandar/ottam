package ds

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/kychandar/ottam/services"
)

type Msg struct {
	ChannelName   string    `json:"channel_name"`
	PublishedTime time.Time `json:"published_time"`
	Data          []byte    `json:"data"`
}

// DeserializeFrom implements services.SerializableMessage.
func (m *Msg) DeserializeFrom(b []byte) error {
	return json.Unmarshal(b, m)
}

// GetChannelName implements services.SerializableMessage.
func (m *Msg) GetChannelName() string {
	return m.ChannelName
}

// GetPublishedTime implements services.SerializableMessage.
func (m *Msg) GetPublishedTime() time.Time {
	return m.PublishedTime
}

// Serialize implements services.SerializableMessage.
func (m *Msg) Serialize() ([]byte, error) {
	return json.Marshal(m)
}

func New(channelName string, data []byte) (services.SerializableMessage, error) {
	if len(channelName) < 1 {
		return nil, fmt.Errorf("invalid channel name")
	}
	return &Msg{
		ChannelName:   channelName,
		Data:          data,
		PublishedTime: time.Now(),
	}, nil
}

func NewEmpty() services.SerializableMessage {
	return &Msg{}
}
