package centprocessor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/metrics"
	"github.com/kychandar/ottam/services"
	"github.com/nats-io/nats.go"
	slogctx "github.com/veqryn/slog-context"
)

type centralProcessorImpl struct {
	js         services.PubSubProvider
	dataStore  services.DataStore
	wg         sync.WaitGroup
	newMessage func() services.SerializableMessage
	stopOnce   sync.Once
	stopped    chan struct{}
	nodesMap   haxmap.Map[common.ChannelName, []string]
}

const (
	centProcessorStream   = "centralProcessor"
	centProcessorConsumer = "centralProcessor"
)

func NewCentralProcessor(
	ctx context.Context,
	js services.PubSubProvider,
	dataStore services.DataStore,
	newMessage func() services.SerializableMessage,
) services.CentralProcessor {
	logger := slogctx.FromCtx(ctx).With("component", "central-processor")
	ctx = slogctx.NewCtx(ctx, logger)
	return &centralProcessorImpl{
		js:         js,
		dataStore:  dataStore,
		newMessage: newMessage,
		stopped:    make(chan struct{}),
	}
}

const (
	numWorkers = 20
	bufferSize = 1000
)

type msgJob struct {
	msg *nats.Msg
}

func (c *centralProcessorImpl) Start(ctx context.Context) error {
	logger := slogctx.FromCtx(ctx)
	logger.InfoContext(ctx, "starting service")

	err := c.js.CreateOrUpdateStream(ctx, centProcessorStream, []string{centProcessorStream})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		return err
	}

	// Subscribe to NATS JetStream
	err = c.js.Subscribe(ctx, centProcessorConsumer, centProcessorStream, func(msgBytes []byte) bool {
		msg := c.newMessage()
		if err := msg.DeserializeFrom(msgBytes); err != nil {
			logger.Error("failed to deserialize message")
			return false
		}

		metrics.LatencyHist.WithLabelValues(metrics.Hostname, "cent_processor_start").Observe(float64(time.Since(msg.GetPublishedTime()).Milliseconds()))

		logger := logger.With("id", msg.GetMsgID())
		logger.InfoContext(ctx, "msg recieved after", "ms", time.Since(msg.GetPublishedTime()).String())

		// logger.Debug("processing message")

		// start := time.Now()
		// nodes, err := c.dataStore.ListNodesSubscribedForChannel(ctx, msg.GetChannelName())
		// if err != nil {
		// 	logger.Error("failed to fetch channel nodes")
		// 	return false
		// }
		// logger.InfoContext(ctx, "nodes list query latency", "ms", time.Since(start).String())
		// hasError := false
		// for _, node := range nodes {
		// 	subj := common.ServerSubjFormat(string(node))
		// 	err := c.js.Publish(subj, msgBytes)
		// 	logger.InfoContext(ctx, "published msg after", "ms", time.Since(msg.GetPublishedTime()).String())
		// 	if err != nil {
		// 		logger.ErrorContext(ctx, "publish failed")
		// 		hasError = true
		// 	} else {
		// 		logger.InfoContext(ctx, "published to node")
		// 	}
		// }
		// if hasError {
		// 	return false
		// }

		err := c.js.Publish(ctx, common.ServerSubjFormat("node1"), msgBytes)
		if err != nil {
			logger.ErrorContext(ctx, "error in publishing", "err", err)
			return false
		}
		logger.InfoContext(ctx, "published to all nodes after", "ms", time.Since(msg.GetPublishedTime()).String())
		metrics.LatencyHist.WithLabelValues(metrics.Hostname, "cent_processor_done").Observe(float64(time.Since(msg.GetPublishedTime()).Milliseconds()))

		return true
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to stream: %w", err)
	}

	logger.InfoContext(ctx, "centralProcessor started successfully")
	return nil
}

func (c *centralProcessorImpl) Stop(ctx context.Context) error {
	logger := slogctx.FromCtx(ctx)
	logger.Info("stopping service")
	err := c.js.UnSubscribe(centProcessorConsumer)
	if err != nil {
		logger.Error("failed to unsubscribe", slog.Any("error", err))
		return err
	}

	logger.Info("service stopped gracefully")
	return nil
}
