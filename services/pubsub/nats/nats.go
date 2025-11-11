package pubSubProvider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/kychandar/ottam/config"
	"github.com/kychandar/ottam/services"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type NatsPubSub struct {
	nc                  *nats.Conn
	js                  jetstream.JetStream
	consumerConsumption map[string]jetstream.ConsumeContext
	lock                sync.Mutex
}

func NewNatsPubSub(natsURL string) (services.PubSubProvider, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &NatsPubSub{
		nc:                  nc,
		js:                  js,
		consumerConsumption: make(map[string]jetstream.ConsumeContext),
	}, nil
}

// NewNatsPubSubWithTLS creates a new NATS PubSub provider with TLS support
func NewNatsPubSubWithTLS(natsURL string, tlsConfig config.TLSConfig) (services.PubSubProvider, error) {
	var opts []nats.Option

	if tlsConfig.Enabled {
		// Load TLS configuration
		tlsConf, err := loadTLSConfig(tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS config: %w", err)
		}
		opts = append(opts, nats.Secure(tlsConf))
	}

	nc, err := nats.Connect(natsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	return &NatsPubSub{
		nc:                  nc,
		js:                  js,
		consumerConsumption: make(map[string]jetstream.ConsumeContext),
	}, nil
}

// loadTLSConfig loads TLS certificates and creates a tls.Config
func loadTLSConfig(tlsConfig config.TLSConfig) (*tls.Config, error) {
	tlsConf := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Load client certificates if provided
	if tlsConfig.CertFile != "" && tlsConfig.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert/key: %w", err)
		}
		tlsConf.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate if provided
	if tlsConfig.CAFile != "" {
		caCert, err := os.ReadFile(tlsConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConf.RootCAs = caCertPool
	}

	return tlsConf, nil
}

func (n *NatsPubSub) CreateOrUpdateStream(ctx context.Context, streamName string, subjects []string) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	_, err := n.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     streamName,
		Subjects: subjects,
		MaxAge:   2 * time.Minute,
	})

	return err
}

func (n *NatsPubSub) Publish(ctx context.Context, subjectName string, data []byte) error {
	_, err := n.js.Publish(ctx, subjectName, data)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}

func (n *NatsPubSub) CreateOrUpdateConsumer(ctx context.Context, streamName, consumerName string, subjects []string) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	_, err := n.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Name:           consumerName,
		MaxAckPending:  100,
		FilterSubjects: subjects,
		DeliverPolicy:  jetstream.DeliverNewPolicy,
	})

	return err
}

func (n *NatsPubSub) Subscribe(ctx context.Context, streamName, consumerName string, callBack func(msg []byte) bool) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	consumer, err := n.js.Consumer(ctx, streamName, consumerName)
	if err != nil {
		return err
	}

	consumerCont := n.consumerConsumption[consumerName]
	if consumerCont != nil {
		consumerCont.Stop()
	}

	consumerContext, err := consumer.Consume(func(msg jetstream.Msg) {
		if callBack(msg.Data()) {
			if err := msg.Ack(); err != nil {
				slog.Error("ack error", "err", err)
			}
		} else {
			if err := msg.Nak(); err != nil {
				slog.Error("nak error", "err", err)
			}
		}
	})

	n.consumerConsumption[consumerName] = consumerContext

	return nil
}

func (n *NatsPubSub) UnSubscribe(consumerName string) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	consumerCont := n.consumerConsumption[consumerName]
	if consumerCont != nil {
		consumerCont.Stop()
	}

	delete(n.consumerConsumption, consumerName)

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
