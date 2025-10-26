package wspooler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/services/valkey"
)

// Define an upgrader to upgrade HTTP connections to WebSocket
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all connections (for demo)
	},
}

func (pooler *Pooler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	log.Println("Client connected!")

	// lock := &sync.Mutex{}
	for {
		// Read message from client
		// mt, message, err := conn.ReadMessage()
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}
		log.Printf("Received: %s\n", message)

		// asyncWriter(conn, lock)

		go func() {
			pooler.conn.Publish(context.TODO(), "t1", message)
		}()

		// Echo the message back
		// lock.Lock()
		// response := fmt.Sprintf("Server echo : recieved message : %s", message)
		// if err := conn.WriteMessage(mt, []byte(response)); err != nil {
		// 	log.Println("Write error:", err)
		// 	break
		// }
		// lock.Unlock()

	}
}

func asyncWriter(conn *websocket.Conn, lock *sync.Mutex) {
	for range time.NewTicker(2 * time.Second).C {
		lock.Lock()
		fmt.Println("initiating msg send...")
		err := conn.WriteMessage(websocket.TextMessage, []byte("ping"))
		if err != nil {
			log.Println("Write error:", err)
			// break
		} else {
			fmt.Println("successfully msg send...")
		}
		lock.Unlock()
	}

}

type Pooler struct {
	conn *valkey.ValKeyConnection
}

func New(conn *valkey.ValKeyConnection) *Pooler {
	return &Pooler{conn: conn}
}

func (pooler *Pooler) Start() {
	http.HandleFunc("/ws", pooler.handleWebSocket)
	fmt.Println("Server started at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
