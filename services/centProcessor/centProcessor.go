package centprocessor

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/services"
	slogctx "github.com/veqryn/slog-context"
)

type centralProcessorImpl struct {
	js           services.PubSubProvider
	cache        services.RedisProtoCache
	streamName   string
	subject      string
	consumerName string
	wg           sync.WaitGroup
	newMessage   func() services.SerializableMessage
	stopOnce     sync.Once
	stopped      chan struct{}
}

func NewCentralProcessor(
	ctx context.Context,
	js services.PubSubProvider,
	cache services.RedisProtoCache,
	streamName string,
	subject string,
	consumerName string,
	newMessage func() services.SerializableMessage,
) services.CentralProcessor {
	logger := slogctx.FromCtx(ctx).With("component", "central-processor")
	ctx = slogctx.NewCtx(ctx, logger)
	return &centralProcessorImpl{
		js:           js,
		cache:        cache,
		streamName:   streamName,
		subject:      subject,
		consumerName: consumerName,
		newMessage:   newMessage,
		stopped:      make(chan struct{}),
	}
}

func (c *centralProcessorImpl) Start(ctx context.Context) error {
	logger := slogctx.FromCtx(ctx)
	logger.Info("starting service")

	// Subscribe to NATS JetStream
	err := c.js.Subscribe(c.consumerName, c.subject, func(msgBytes []byte) bool {
		msg := c.newMessage()
		if err := msg.DeserializeFrom(msgBytes); err != nil {
			logger.Error("failed to deserialize message")
			return false
		}

		logger.Debug("processing message")

		nodes, err := c.cache.SMEMBERS(ctx, common.ChannelSubscCacheKeyFormat(msg.GetChannelName()))
		if err != nil {
			logger.Error("failed to fetch channel nodes")
			return false
		}

		hasError := false
		for _, node := range nodes {
			subj := common.ServerSubjFormat(node)
			if err := c.js.Publish(subj, msgBytes); err != nil {
				logger.Error("publish failed")
				hasError = true
			} else {
				logger.Debug("published to node")
			}
		}
		if hasError {
			return false
		}
		return true
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to stream: %w", err)
	}

	logger.Info("centralProcessor started successfully")
	return nil
}

func (c *centralProcessorImpl) Stop(ctx context.Context) error {
	logger := slogctx.FromCtx(ctx)
	logger.Info("stopping service")
	err := c.js.UnSubscribe(c.consumerName)
	if err != nil {
		logger.Error("failed to unsubscribe", slog.Any("error", err))
		return err
	}

	logger.Info("service stopped gracefully")
	return nil
}
