package wswritechannelmanager

import (
	"time"

	"github.com/alphadose/haxmap"
	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/services"
)

// writeRequest represents a write operation to be performed
type writeRequest struct {
	msgType     int
	data        []byte
	preparedMsg *websocket.PreparedMessage
	errCh       chan error // Channel to receive write error
}

// connWithWriter wraps a WebSocket connection with a dedicated writer channel
type connWithWriter struct {
	conn    *websocket.Conn
	writeCh chan writeRequest
	closeCh chan struct{}
}

// wsWriteChanManager is a thread-safe manager for WebSocket connections.
type wsWriteChanManager struct {
	connections *haxmap.Map[string, *connWithWriter]
}

// NewClientWriterManager creates a new instance of ClientWriterManager.
func NewClientWriterManager() services.WsWriteChanManager {
	return &wsWriteChanManager{
		connections: haxmap.New[string, *connWithWriter](),
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

// SetConnectionForClientID sets the WebSocket connection and starts a dedicated writer goroutine
func (m *wsWriteChanManager) SetConnectionForClientID(clientID string, conn *websocket.Conn) {
	connWriter := &connWithWriter{
		conn:    conn,
		writeCh: make(chan writeRequest, 1000), // Buffered channel for better throughput
		closeCh: make(chan struct{}),
	}
	
	m.connections.Set(clientID, connWriter)
	
	// Start dedicated writer goroutine for this connection
	go m.writerLoop(connWriter)
}

// writerLoop handles all writes for a single connection
func (m *wsWriteChanManager) writerLoop(cw *connWithWriter) {
	for {
		select {
		case <-cw.closeCh:
			return
		case req := <-cw.writeCh:
			var err error
			if req.preparedMsg != nil {
				// Write prepared message
				err = cw.conn.WritePreparedMessage(req.preparedMsg)
			} else {
				// Write regular message with deadline
				cw.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err = cw.conn.WriteMessage(req.msgType, req.data)
			}
			// Send error back if channel is provided (for synchronous writes)
			if req.errCh != nil {
				req.errCh <- err
				close(req.errCh)
			}
		}
	}
}

// DeleteClientID deletes the client ID and stops the writer goroutine
func (m *wsWriteChanManager) DeleteClientID(clientID string) {
	if connInfo, ok := m.connections.Get(clientID); ok {
		close(connInfo.closeCh)
		m.connections.Del(clientID)
	}
}

// WritePreparedMessage writes a prepared message asynchronously (no blocking)
func (m *wsWriteChanManager) WritePreparedMessage(clientID string, pm *websocket.PreparedMessage) error {
	connInfo, ok := m.connections.Get(clientID)
	if !ok {
		return websocket.ErrCloseSent
	}
	
	// Non-blocking send
	select {
	case connInfo.writeCh <- writeRequest{preparedMsg: pm}:
		return nil
	default:
		// Channel full - connection is slow, drop message to avoid blocking
		return nil
	}
}

// WriteMessage writes a message asynchronously (no blocking)
func (m *wsWriteChanManager) WriteMessage(clientID string, messageType int, data []byte) error {
	connInfo, ok := m.connections.Get(clientID)
	if !ok {
		return websocket.ErrCloseSent
	}
	
	// Non-blocking send
	select {
	case connInfo.writeCh <- writeRequest{msgType: messageType, data: data}:
		return nil
	default:
		// Channel full - connection is slow
		return nil
	}
}
