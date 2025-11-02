package nats

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
)

func runEmbeddedNATSServer(t *testing.T) *server.Server {
	opts := &server.Options{
		Port:      -1, // random available port
		JetStream: true,
	}
	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("failed to start embedded NATS: %v", err)
	}
	go s.Start()
	if !s.ReadyForConnections(2 * time.Second) {
		t.Fatal("nats-server not ready")
	}
	t.Cleanup(func() {
		s.Shutdown()
		s.WaitForShutdown()
	})
	return s
}

func TestCreateStream_NewAndExisting(t *testing.T) {
	s := runEmbeddedNATSServer(t)
	url := fmt.Sprintf("nats://%s", s.Addr().String())

	pubsub, err := NewNatsPubSub(url)
	if err != nil {
		t.Fatalf("failed to create NatsPubSub: %v", err)
	}
	defer pubsub.Close()

	subject := "test.subject"
	streamName := uuid.New().String()
	// First create — should create a new stream
	if err := pubsub.CreateStream(streamName, []string{subject}); err != nil {
		t.Fatalf("CreateStream (first): %v", err)
	}

	// Second create — should be idempotent (no error)
	if err := pubsub.CreateStream(streamName, []string{subject}); err != nil {
		t.Fatalf("CreateStream (second): %v", err)
	}

	// Modify subjects manually to simulate config drift
	js := pubsub.(*NatsPubSub).js
	info, _ := js.StreamInfo(streamName)
	updated := info.Config
	updated.Subjects = []string{"different"}
	if _, err := js.UpdateStream(&updated); err != nil {
		t.Fatalf("update stream manually: %v", err)
	}

	// Recreate should detect mismatch and update
	if err := pubsub.CreateStream(streamName, []string{subject}); err != nil {
		t.Fatalf("CreateStream (update case): %v", err)
	}

	// Verify subject restored
	info, _ = js.StreamInfo(streamName)
	if len(info.Config.Subjects) != 1 || info.Config.Subjects[0] != subject {
		t.Fatalf("expected subjects=[%s], got %v", subject, info.Config.Subjects)
	}
}

func TestPublishSubscribe(t *testing.T) {
	s := runEmbeddedNATSServer(t)
	url := fmt.Sprintf("nats://%s", s.Addr().String())

	pubsub, err := NewNatsPubSub(url)
	if err != nil {
		t.Fatalf("failed to create NatsPubSub: %v", err)
	}
	defer pubsub.Close()

	subject := "chat.room"
	streamName := uuid.New().String()
	if err := pubsub.CreateStream(streamName, []string{subject}); err != nil {
		t.Fatalf("CreateStream: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	received := make(chan []byte, 1)

	err = pubsub.Subscribe("consumer1", subject, func(msg []byte) {
		received <- msg
		wg.Done()
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	data := []byte("hello world")
	if err := pubsub.Publish(subject, data); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		got := <-received
		if string(got) != string(data) {
			t.Fatalf("expected %s, got %s", data, got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestUnSubscribe(t *testing.T) {
	s := runEmbeddedNATSServer(t)
	url := fmt.Sprintf("nats://%s", s.Addr().String())

	pubsub, err := NewNatsPubSub(url)
	if err != nil {
		t.Fatalf("failed to create NatsPubSub: %v", err)
	}
	defer pubsub.Close()

	subject := "room.42"
	streamName := uuid.New().String()
	if err := pubsub.CreateStream(streamName, []string{subject}); err != nil {
		t.Fatalf("CreateStream: %v", err)
	}

	if err := pubsub.Subscribe("c1", subject, func([]byte) {}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := pubsub.UnSubscribe("c1"); err != nil {
		t.Fatalf("UnSubscribe: %v", err)
	}

	// Unsubscribing again should fail
	if err := pubsub.UnSubscribe("c1"); err == nil {
		t.Fatal("expected error on second UnSubscribe, got nil")
	}
}

func TestCloseIsGraceful(t *testing.T) {
	s := runEmbeddedNATSServer(t)
	url := fmt.Sprintf("nats://%s", s.Addr().String())

	pubsub, err := NewNatsPubSub(url)
	if err != nil {
		t.Fatalf("failed to create NatsPubSub: %v", err)
	}

	done := make(chan struct{})
	go func() {
		_ = pubsub.Close()
		close(done)
	}()

	select {
	case <-done:
		// good, closed gracefully
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not complete in time")
	}
}
