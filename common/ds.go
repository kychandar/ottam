package common

import (
	"time"

	"github.com/gorilla/websocket"
)

type IntermittenMsg struct {
	PublishedTime time.Time
	Id            string
	// Data          []byte
	PreparedMessage *websocket.PreparedMessage
}
