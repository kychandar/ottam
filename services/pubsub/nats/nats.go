package pubSubProvider

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/kychandar/ottam/services"
	"github.com/nats-io/nats.go"
)

type NatsPubSub struct {
	nc       *nats.Conn
	js       nats.JetStreamContext
	subs     map[string]*nats.Subscription
	mu       sync.Mutex
	streamMu sync.Mutex
}

func NewNatsPubSub(natsURL string) (services.PubSubProvider, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create jetstream: %w", err)
	}

	return &NatsPubSub{
		nc:   nc,
		js:   js,
		subs: make(map[string]*nats.Subscription),
	}, nil
}

func (n *NatsPubSub) CreateStream(streamName string, subjects []string) error {
	n.streamMu.Lock()
	defer n.streamMu.Unlock()

	// Try to fetch existing stream info
	streamInfo, err := n.js.StreamInfo(streamName)
	if err != nil {
		if err == nats.ErrStreamNotFound {
			// Not found -> create it
			cfg := &nats.StreamConfig{
				Name:     streamName,
				Subjects: subjects,
				MaxAge:   2 * time.Second,
			}
			if _, err := n.js.AddStream(cfg); err != nil {
				return fmt.Errorf("create stream: %w", err)
			}
			return nil
		}
		// Any other error
		return fmt.Errorf("get stream info: %w", err)
	}

	// Stream exists — check if subjects are up to date
	if !subjectsMatch(streamInfo.Config.Subjects, subjects) {
		updated := streamInfo.Config
		updated.Subjects = subjects
		if _, err := n.js.UpdateStream(&updated); err != nil {
			return fmt.Errorf("update stream: %w", err)
		}
	}

	return nil
}

func subjectsMatch(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (n *NatsPubSub) Publish(subjectName string, data []byte) error {
	_, err := n.js.Publish(subjectName, data)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}

func (n *NatsPubSub) Subscribe(consumerName string, subjectName string, callBack func(msg []byte) bool) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if _, ok := n.subs[consumerName]; ok {
		return fmt.Errorf("consumer %s already subscribed", consumerName)
	}
	maxWorkers := 100
	maxWorkersString, exist := os.LookupEnv("MAX_WORKERS")
	if exist {
		parsed, err := strconv.Atoi(maxWorkersString)
		if err != nil {
			fmt.Printf("invalid MAX_WORKERS=%q, using default %d\n", maxWorkersString, maxWorkers)
		} else {
			maxWorkers = parsed
		}
	}
	// const (
	// 	numWorkers = 200
	// 	bufferSize = 2000
	// )

	// type msgJob struct {
	// 	msg *nats.Msg
	// }

	// workCh := make(chan msgJob, bufferSize)

	// for i := 0; i < numWorkers; i++ {
	// 	go func(workerID int) {
	// 		for job := range workCh {
	// 			if callBack(job.msg.Data) {
	// 				if err := job.msg.Ack(); err != nil {
	// 					log.Printf("worker %d ack error: %v", workerID, err)
	// 				}
	// 			} else {
	// 				if err := job.msg.Nak(); err != nil {
	// 					log.Printf("worker %d nak error: %v", workerID, err)
	// 				}
	// 			}
	// 		}
	// 	}(i)
	// }

	sub, err := n.js.Subscribe(subjectName, func(m *nats.Msg) {
		// workCh <- msgJob{msg: m}
		if callBack(m.Data) {
			if err := m.Ack(); err != nil {
				log.Printf("ack error: %v", err)
			}
		} else {
			if err := m.Nak(); err != nil {
				log.Printf("nak error: %v", err)
			}
		}
	},
		nats.Durable(consumerName),
		nats.AckExplicit(),
		nats.MaxAckPending(maxWorkers),
		// nats.AckWait(2*time.Minute),
	)
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	n.subs[consumerName] = sub
	return nil
}

func (n *NatsPubSub) UnSubscribe(consumerName string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	sub, ok := n.subs[consumerName]
	if !ok {
		return fmt.Errorf("no subscription for consumer %s", consumerName)
	}

	if err := sub.Unsubscribe(); err != nil {
		return fmt.Errorf("unsubscribe: %w", err)
	}
	delete(n.subs, consumerName)
	return nil
}

func (n *NatsPubSub) Close() error {
	done := make(chan struct{})

	n.nc.SetClosedHandler(func(_ *nats.Conn) {
		close(done) // signal that drain is done
	})

	if err := n.nc.Drain(); err != nil {
		return err
	}

	<-done
	return nil
}
