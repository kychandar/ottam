package wswritechannelmanager

import (
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/services"
)

// connWithMutex wraps a WebSocket connection with a mutex for safe concurrent writes
type connWithMutex struct {
	conn  *websocket.Conn
	mutex sync.Mutex
}

// wsWriteChanManager is a thread-safe manager for WebSocket connections.
type wsWriteChanManager struct {
	connections *haxmap.Map[string, *connWithMutex]
}

// NewClientWriterManager creates a new instance of ClientWriterManager.
func NewClientWriterManager() services.WsWriteChanManager {
	return &wsWriteChanManager{
		connections: haxmap.New[string, *connWithMutex](),
	}
}

// GetConnectionForClientID returns the WebSocket connection for a given client ID.
func (m *wsWriteChanManager) GetConnectionForClientID(clientID string) (*websocket.Conn, bool) {
	connInfo, ok := m.connections.Get(clientID)
	if !ok {
		return nil, false
	}
	return connInfo.conn, true
}

// SetConnectionForClientID sets the WebSocket connection
func (m *wsWriteChanManager) SetConnectionForClientID(clientID string, conn *websocket.Conn) {
	connWithMux := &connWithMutex{
		conn: conn,
	}
	m.connections.Set(clientID, connWithMux)
}

// DeleteClientID deletes the client ID
func (m *wsWriteChanManager) DeleteClientID(clientID string) {
	m.connections.Del(clientID)
}

// WritePreparedMessage writes a prepared message with mutex protection
func (m *wsWriteChanManager) WritePreparedMessage(clientID string, pm *websocket.PreparedMessage) error {
	connInfo, ok := m.connections.Get(clientID)
	if !ok {
		return websocket.ErrCloseSent
	}

	// Lock the mutex to ensure only one write at a time for this connection
	connInfo.mutex.Lock()
	defer connInfo.mutex.Unlock()

	// Write with deadline
	connInfo.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return connInfo.conn.WritePreparedMessage(pm)
}

// WriteMessage writes a message with mutex protection
func (m *wsWriteChanManager) WriteMessage(clientID string, messageType int, data []byte) error {
	connInfo, ok := m.connections.Get(clientID)
	if !ok {
		return websocket.ErrCloseSent
	}

	// Lock the mutex to ensure only one write at a time for this connection
	connInfo.mutex.Lock()
	defer connInfo.mutex.Unlock()

	// Write with deadline
	connInfo.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return connInfo.conn.WriteMessage(messageType, data)
}
