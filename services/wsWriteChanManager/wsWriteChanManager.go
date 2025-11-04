package wswritechannelmanager

import (
	"github.com/alphadose/haxmap"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/services"
)

// wsWriteChanManager is a thread-safe implementation of ClientWriterManager.
type wsWriteChanManager struct {
	clients *haxmap.Map[string, chan<- common.IntermittenMsg]
}

// NewClientWriterManager creates a new instance of ClientWriterManager.
func NewClientWriterManager() services.WsWriteChanManager {
	return &wsWriteChanManager{
		clients: haxmap.New[string, chan<- common.IntermittenMsg](),
	}
}

// GetWriterChannelForClientID returns the writer channel associated with a given client ID.
func (m *wsWriteChanManager) GetWriterChannelForClientID(clientID string) (chan<- common.IntermittenMsg, bool) {
	return m.clients.Get(clientID)
}

// SetWriterChannelForClientID sets the writer channel for a client ID.
func (m *wsWriteChanManager) SetWriterChannelForClientID(clientID string, writerChan chan<- common.IntermittenMsg) {
	m.clients.Set(clientID, writerChan)
}

// SetWriterChannelForClientID deletes the client ID.
func (m *wsWriteChanManager) DeleteClientID(clientID string) {
	m.clients.Del(clientID, clientID)
}
