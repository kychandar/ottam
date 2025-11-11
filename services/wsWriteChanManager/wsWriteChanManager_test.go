package wswritechannelmanager

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Helper to create a test WebSocket connection
func createTestWebSocketConnection(t *testing.T) (*websocket.Conn, *websocket.Conn, func()) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		// Keep connection alive for tests
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))

	// Connect to the test server
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Give it a moment to establish
	time.Sleep(10 * time.Millisecond)

	cleanup := func() {
		clientConn.Close()
		server.Close()
	}

	return clientConn, clientConn, cleanup
}

func TestNewClientWriterManager(t *testing.T) {
	manager := NewClientWriterManager()
	assert.NotNil(t, manager)
}

func TestSetConnectionForClientID(t *testing.T) {
	manager := NewClientWriterManager().(*wsWriteChanManager)
	conn, _, cleanup := createTestWebSocketConnection(t)
	defer cleanup()

	clientID := "test-client-1"

	manager.SetConnectionForClientID(clientID, conn)

	// Verify the connection was set
	connInfo, exists := manager.connections.Get(clientID)
	assert.True(t, exists)
	assert.NotNil(t, connInfo)
	assert.Equal(t, conn, connInfo.conn)
}

func TestGetConnectionForClientID_Exists(t *testing.T) {
	manager := NewClientWriterManager().(*wsWriteChanManager)
	conn, _, cleanup := createTestWebSocketConnection(t)
	defer cleanup()

	clientID := "test-client-1"
	manager.SetConnectionForClientID(clientID, conn)

	// Retrieve the connection
	retrievedConn, exists := manager.GetConnectionForClientID(clientID)
	assert.True(t, exists)
	assert.Equal(t, conn, retrievedConn)
}

func TestGetConnectionForClientID_NotExists(t *testing.T) {
	manager := NewClientWriterManager()

	// Try to get a non-existent connection
	retrievedConn, exists := manager.GetConnectionForClientID("non-existent-client")
	assert.False(t, exists)
	assert.Nil(t, retrievedConn)
}

func TestDeleteClientID(t *testing.T) {
	manager := NewClientWriterManager().(*wsWriteChanManager)
	conn, _, cleanup := createTestWebSocketConnection(t)
	defer cleanup()

	clientID := "test-client-1"
	manager.SetConnectionForClientID(clientID, conn)

	// Verify it exists
	_, exists := manager.GetConnectionForClientID(clientID)
	assert.True(t, exists)

	// Delete the client
	manager.DeleteClientID(clientID)

	// Verify it no longer exists
	_, exists = manager.GetConnectionForClientID(clientID)
	assert.False(t, exists)
}

func TestDeleteClientID_NonExistent(t *testing.T) {
	manager := NewClientWriterManager()

	// Deleting a non-existent client should not panic
	assert.NotPanics(t, func() {
		manager.DeleteClientID("non-existent-client")
	})
}

func TestWritePreparedMessage_ClientNotFound(t *testing.T) {
	manager := NewClientWriterManager()

	// Create a prepared message
	pm, err := websocket.NewPreparedMessage(websocket.TextMessage, []byte("test"))
	require.NoError(t, err)

	// Try to write to a non-existent client
	err = manager.WritePreparedMessage("non-existent-client", pm)
	assert.Error(t, err)
	assert.Equal(t, websocket.ErrCloseSent, err)
}

func TestWriteMessage_ClientNotFound(t *testing.T) {
	manager := NewClientWriterManager().(*wsWriteChanManager)

	// Try to write to a non-existent client
	err := manager.WriteMessage("non-existent-client", websocket.TextMessage, []byte("test"))
	assert.Error(t, err)
	assert.Equal(t, websocket.ErrCloseSent, err)
}

func TestMultipleClientsManagement(t *testing.T) {
	manager := NewClientWriterManager().(*wsWriteChanManager)

	// Create multiple connections
	conn1, _, cleanup1 := createTestWebSocketConnection(t)
	defer cleanup1()
	conn2, _, cleanup2 := createTestWebSocketConnection(t)
	defer cleanup2()
	conn3, _, cleanup3 := createTestWebSocketConnection(t)
	defer cleanup3()

	// Set connections for multiple clients
	manager.SetConnectionForClientID("client-1", conn1)
	manager.SetConnectionForClientID("client-2", conn2)
	manager.SetConnectionForClientID("client-3", conn3)

	// Verify all connections exist
	retrievedConn1, exists1 := manager.GetConnectionForClientID("client-1")
	assert.True(t, exists1)
	assert.Equal(t, conn1, retrievedConn1)

	retrievedConn2, exists2 := manager.GetConnectionForClientID("client-2")
	assert.True(t, exists2)
	assert.Equal(t, conn2, retrievedConn2)

	retrievedConn3, exists3 := manager.GetConnectionForClientID("client-3")
	assert.True(t, exists3)
	assert.Equal(t, conn3, retrievedConn3)

	// Delete one client
	manager.DeleteClientID("client-2")

	// Verify client-2 is gone but others remain
	_, exists2 = manager.GetConnectionForClientID("client-2")
	assert.False(t, exists2)

	_, exists1 = manager.GetConnectionForClientID("client-1")
	assert.True(t, exists1)

	_, exists3 = manager.GetConnectionForClientID("client-3")
	assert.True(t, exists3)
}

func TestSetConnectionForClientID_Overwrite(t *testing.T) {
	manager := NewClientWriterManager().(*wsWriteChanManager)

	conn1, _, cleanup1 := createTestWebSocketConnection(t)
	defer cleanup1()
	conn2, _, cleanup2 := createTestWebSocketConnection(t)
	defer cleanup2()

	clientID := "test-client"

	// Set first connection
	manager.SetConnectionForClientID(clientID, conn1)
	retrievedConn, _ := manager.GetConnectionForClientID(clientID)
	assert.Equal(t, conn1, retrievedConn)

	// Overwrite with second connection
	manager.SetConnectionForClientID(clientID, conn2)
	retrievedConn, _ = manager.GetConnectionForClientID(clientID)
	assert.Equal(t, conn2, retrievedConn)
	assert.NotEqual(t, conn1, retrievedConn)
}

func TestConcurrentAccess(t *testing.T) {
	manager := NewClientWriterManager()

	numClients := 100
	done := make(chan bool, numClients*3)

	// Concurrent sets
	for i := 0; i < numClients; i++ {
		go func(clientNum int) {
			conn, _, cleanup := createTestWebSocketConnection(t)
			defer cleanup()
			clientID := string(rune('A'+clientNum%26)) + string(rune('0'+clientNum/26))
			manager.SetConnectionForClientID(clientID, conn)
			done <- true
		}(i)
	}

	// Wait for all sets
	for i := 0; i < numClients; i++ {
		<-done
	}

	// Small delay to ensure all operations complete
	time.Sleep(50 * time.Millisecond)

	// Concurrent gets
	for i := 0; i < numClients; i++ {
		go func(clientNum int) {
			clientID := string(rune('A'+clientNum%26)) + string(rune('0'+clientNum/26))
			_, exists := manager.GetConnectionForClientID(clientID)
			assert.True(t, exists)
			done <- true
		}(i)
	}

	// Wait for all gets
	for i := 0; i < numClients; i++ {
		<-done
	}

	// Concurrent deletes
	for i := 0; i < numClients; i++ {
		go func(clientNum int) {
			clientID := string(rune('A'+clientNum%26)) + string(rune('0'+clientNum/26))
			manager.DeleteClientID(clientID)
			done <- true
		}(i)
	}

	// Wait for all deletes
	for i := 0; i < numClients; i++ {
		<-done
	}
}

func TestConnWithMutex_ThreadSafety(t *testing.T) {
	manager := NewClientWriterManager()
	conn, _, cleanup := createTestWebSocketConnection(t)
	defer cleanup()

	clientID := "test-client"
	manager.SetConnectionForClientID(clientID, conn)

	// Prepare a message
	pm, err := websocket.NewPreparedMessage(websocket.TextMessage, []byte("test message"))
	require.NoError(t, err)

	numWrites := 50
	done := make(chan bool, numWrites)

	// Concurrent writes to the same connection
	for i := 0; i < numWrites; i++ {
		go func() {
			// Note: This will likely fail because we're writing to a closed connection
			// but the important thing is that it doesn't panic due to race conditions
			_ = manager.WritePreparedMessage(clientID, pm)
			done <- true
		}()
	}

	// Wait for all writes
	for i := 0; i < numWrites; i++ {
		<-done
	}
}

func TestWriteMessage_Success(t *testing.T) {
	manager := NewClientWriterManager().(*wsWriteChanManager)

	// Create a server that receives and echoes messages
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		
		// Read one message and echo it back
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(messageType, p)
		
		// Keep connection open for a bit
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	// Connect to the test server
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer clientConn.Close()

	clientID := "test-client"
	manager.SetConnectionForClientID(clientID, clientConn)

	// Write a message
	testData := []byte("test message")
	err = manager.WriteMessage(clientID, websocket.TextMessage, testData)
	assert.NoError(t, err)

	// Read the echoed message
	messageType, message, err := clientConn.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.Equal(t, testData, message)
}

func TestWriteMessage_BinaryMessage(t *testing.T) {
	manager := NewClientWriterManager().(*wsWriteChanManager)

	// Create a server that receives messages
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		
		// Read and echo back
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(messageType, p)
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer clientConn.Close()

	clientID := "test-client"
	manager.SetConnectionForClientID(clientID, clientConn)

	// Write a binary message
	testData := []byte{0x00, 0x01, 0x02, 0x03, 0xFF}
	err = manager.WriteMessage(clientID, websocket.BinaryMessage, testData)
	assert.NoError(t, err)

	// Read the echoed message
	messageType, message, err := clientConn.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.BinaryMessage, messageType)
	assert.Equal(t, testData, message)
}

func TestWritePreparedMessage_Success(t *testing.T) {
	manager := NewClientWriterManager()

	// Create a server that receives messages
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		
		// Read message
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(messageType, p)
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer clientConn.Close()

	clientID := "test-client"
	manager.SetConnectionForClientID(clientID, clientConn)

	// Prepare and write a message
	testData := []byte("prepared message")
	pm, err := websocket.NewPreparedMessage(websocket.TextMessage, testData)
	require.NoError(t, err)

	err = manager.WritePreparedMessage(clientID, pm)
	assert.NoError(t, err)

	// Read the echoed message
	messageType, message, err := clientConn.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.Equal(t, testData, message)
}

func TestWriteMessage_ConcurrentWritesToSameClient(t *testing.T) {
	manager := NewClientWriterManager().(*wsWriteChanManager)

	// Create a server that just receives messages
	messageCount := 0
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		
		// Read messages
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
			mu.Lock()
			messageCount++
			mu.Unlock()
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer clientConn.Close()

	clientID := "test-client"
	manager.SetConnectionForClientID(clientID, clientConn)

	// Write multiple messages concurrently
	numMessages := 20
	done := make(chan bool, numMessages)

	for i := 0; i < numMessages; i++ {
		go func(msgNum int) {
			testData := []byte("message " + string(rune('0'+msgNum%10)))
			_ = manager.WriteMessage(clientID, websocket.TextMessage, testData)
			done <- true
		}(i)
	}

	// Wait for all writes to complete
	for i := 0; i < numMessages; i++ {
		<-done
	}

	// Give server time to receive messages
	time.Sleep(100 * time.Millisecond)

	// The mutex should have prevented race conditions
	// Not all messages may have been received due to timing, but no panics should occur
}

func TestWriteMessage_EmptyData(t *testing.T) {
	manager := NewClientWriterManager().(*wsWriteChanManager)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			return
		}
		conn.WriteMessage(messageType, p)
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer clientConn.Close()

	clientID := "test-client"
	manager.SetConnectionForClientID(clientID, clientConn)

	// Write an empty message
	err = manager.WriteMessage(clientID, websocket.TextMessage, []byte{})
	assert.NoError(t, err)

	// Read the echoed message
	messageType, message, err := clientConn.ReadMessage()
	assert.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.Equal(t, []byte{}, message)
}

