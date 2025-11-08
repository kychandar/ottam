package centralisedSubscriber

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	set "github.com/duke-git/lancet/v2/datastructure/set"
	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/metrics"
	"github.com/kychandar/ottam/services"
	"github.com/kychandar/ottam/services/pool"
	slogctx "github.com/veqryn/slog-context"
)

type centralisedSubscriber struct {
	lock               sync.RWMutex
	channelsTracker    *haxmap.Map[common.ChannelName, *haxmap.Map[string, struct{}]]
	nodeID             common.NodeID
	pubSubProvider     services.PubSubProvider
	newMessageGetter   func() services.SerializableMessage
	wsWriteChanManager services.WsWriteChanManager
	workerPool         services.WorkerPool
	fanoutCh           chan *fanoutJob
	stopFanout         chan struct{}
	subscriptionSyncer chan struct{}
}

type fanoutJob struct {
	intMsg *common.IntermittenMsg
	set    *haxmap.Map[string, struct{}]
}

func New(
	nodeID common.NodeID,
	wsWriteChanManager services.WsWriteChanManager,
	workerPool services.WorkerPool,
	newMessageGetter func() services.SerializableMessage,
	pubSubProvider services.PubSubProvider) services.CentralisedSubscriber {
	return &centralisedSubscriber{
		nodeID:             nodeID,
		channelsTracker:    haxmap.New[common.ChannelName, *haxmap.Map[string, struct{}]](),
		wsWriteChanManager: wsWriteChanManager,
		workerPool:         workerPool,
		pubSubProvider:     pubSubProvider,
		newMessageGetter:   newMessageGetter,
		fanoutCh:           make(chan *fanoutJob, 10000), // Increased buffer for burst handling
		stopFanout:         make(chan struct{}),
		subscriptionSyncer: make(chan struct{}, 2),
	}
}

func (c *centralisedSubscriber) SubscriptionSyncer(ctx context.Context) {
	fmt.Println("SubscriptionSyncer startee")

	logger := slogctx.FromCtx(ctx).With("comoponent", "centralisedSubscriber")
	logger.DebugContext(ctx, "starting subscription syncer")
	actualSubsSet := set.New[common.ChannelName]()
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.subscriptionSyncer:
			fmt.Println("SubscriptionSyncer started")
			requiredSubsSet := set.New[common.ChannelName]()

			for x := range c.channelsTracker.Keys() {
				requiredSubsSet.Add(x)
			}

			if actualSubsSet.Equal(requiredSubsSet) {
				continue
			}

			keys := make([]string, 0, len(requiredSubsSet))
			for cn := range requiredSubsSet {
				keys = append(keys, common.ChannelSubjFormat(string(cn)))
			}

			err := c.pubSubProvider.CreateOrUpdateConsumer(ctx, common.OttamServerStreamName, string(c.nodeID), keys)
			if err != nil {
				logger.ErrorContext(ctx, "error in updating consumer", "err", err)
				c.subscriptionSyncer <- struct{}{}
			}
			fmt.Println("SubscriptionSyncer completed", keys)
		}
	}
}

func (c *centralisedSubscriber) ProcessDownstreamMessages(ctx context.Context) error {
	logger := slogctx.FromCtx(ctx).With("comoponent", "centralisedSubscriber")
	logger.InfoContext(ctx, "starting service")
	fmt.Println("ProcessDownstreamMessages startee")
	// Start fanout workers (async)
	// Increased from 50 to 500 to handle high connection count efficiently
	// Each worker handles ~10 connections, reducing contention
	const numWorkers = 200
	for i := 0; i < numWorkers; i++ {
		go c.fanoutWorker(ctx, logger)
	}

	objPool := pool.GetGlobalPool()

	err := c.pubSubProvider.CreateOrUpdateStream(ctx, common.OttamServerStreamName, []string{common.ChannelSubjFormat(">")})
	if err != nil {
		return err
	}

	err = c.pubSubProvider.CreateOrUpdateConsumer(ctx, common.OttamServerStreamName, string(c.nodeID), []string{common.ChannelSubjFormat("none")})
	if err != nil {
		return err
	}

	var callBack func(msgBytes []byte) bool
	callBack = func(msgBytes []byte) bool {
		msg := c.newMessageGetter()
		if err := msg.DeserializeFrom(msgBytes); err != nil {
			logger.ErrorContext(ctx, "failed to deserialize message", "err", err, "msg", string(msgBytes))
			return true
		}
		metrics.LatencyHist.WithLabelValues(metrics.Hostname, "cent_subscriber_start").Observe(float64(time.Since(msg.GetPublishedTime()).Milliseconds()))
		pm, err := websocket.NewPreparedMessage(websocket.BinaryMessage, msgBytes)
		if err != nil {
			logger.ErrorContext(ctx, "failed to prepare ws message", "err", err, "msg", string(msgBytes))
			return true
		}

		// Get IntermittenMsg from pool instead of allocating new
		intMsg := objPool.IntermittenMsg.Get()
		intMsg.PublishedTime = msg.GetPublishedTime()
		intMsg.Id = msg.GetMsgID()
		intMsg.PreparedMessage = pm

		logger := logger.With("msg-id", msg.GetMsgID())

		logger.Info("msg recieved after", "ms", time.Since(msg.GetPublishedTime()).String())

		set, exist := c.channelsTracker.Get(msg.GetChannelName())
		if !exist {
			objPool.ResetIntermittenMsg(intMsg)
			return true
		}

		logger.Info("channels map fetched", "ms", time.Since(msg.GetPublishedTime()).String())

		// Send to worker pool (non-blocking fanout)
		select {
		case c.fanoutCh <- &fanoutJob{
			intMsg: intMsg,
			set:    set,
		}:
			logger.Info("msg sent to fanout queue after", "ms", time.Since(msg.GetPublishedTime()).String())
		case <-ctx.Done():
			objPool.ResetIntermittenMsg(intMsg)
			return false
		}

		metrics.LatencyHist.WithLabelValues(metrics.Hostname, "cent_subscriber_done").Observe(float64(time.Since(msg.GetPublishedTime()).Milliseconds()))
		return true
	}

	err = c.pubSubProvider.Subscribe(ctx, common.OttamServerStreamName, string(c.nodeID), callBack)
	if err != nil {
		return err
	}
	return nil
}

// fanoutWorker processes messages and distributes them to subscribed clients
func (c *centralisedSubscriber) fanoutWorker(ctx context.Context, logger *slog.Logger) {
	objPool := pool.GetGlobalPool()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-c.fanoutCh:
			if job == nil {
				return
			}

			// We need to preserve the PreparedMessage for all clients since we're resetting intMsg
			// Multiple workers will use this same PreparedMessage concurrently (which is safe per gorilla/websocket docs)
			preparedMsg := job.intMsg.PreparedMessage
			publishedTime := job.intMsg.PublishedTime
			msgID := job.intMsg.Id

			job.set.ForEach(func(clientID string, _ struct{}) bool {
				conn, exist := c.wsWriteChanManager.GetConnectionForClientID(clientID)
				if !exist {
					return true
				}

				// Submit directly to worker pool with the extracted values
				c.workerPool.SubmitMessageJob(ctx, conn, preparedMsg, publishedTime, msgID, clientID)
				return true
			})

			// Now safe to reset since we extracted what we need
			objPool.ResetIntermittenMsg(job.intMsg)
		}
	}
}

// Subscribe adds a client to a channel
func (s *centralisedSubscriber) Subscribe(ctx context.Context, clientID string, channelName common.ChannelName) error {
	fmt.Println("subscribe called for ", clientID, channelName)
	s.lock.RLock()
	subMap, exist := s.channelsTracker.Get(channelName)
	s.lock.RUnlock()
	if !exist {
		fmt.Println("channel does not exist")
		s.lock.Lock()
		subMap, exist = s.channelsTracker.Get(channelName)
		if !exist {
			subMap = haxmap.New[string, struct{}]()
			s.channelsTracker.Set(channelName, subMap)
			fmt.Println("added channel to sync map", s.channelsTracker.Len())
			select {
			case s.subscriptionSyncer <- struct{}{}:
				fmt.Println("triggerd syncer")
			default:
				fmt.Println("not triggerd syncer")
			}
		}
		s.lock.Unlock()
	}
	subMap.Set(clientID, struct{}{})
	return nil
}

// UnSubscribe removes a client from a specific channel
func (s *centralisedSubscriber) UnSubscribe(ctx context.Context, clientID string, channelName common.ChannelName) error {
	s.lock.RLock()
	subMap, exist := s.channelsTracker.Get(channelName)
	s.lock.RUnlock()
	if !exist {
		return nil
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	subMap, exist = s.channelsTracker.Get(channelName)
	if !exist {
		return nil
	} else {
		_, exist := subMap.Get(clientID)
		if !exist {
			return nil
		}
		if subMap.Len() == 1 {
			s.channelsTracker.Del(channelName)
			select {
			case s.subscriptionSyncer <- struct{}{}:
			default:
			}
		} else {
			subMap.Del(clientID)
		}
	}
	return nil
}

// UnsubscribeAll removes the client from all channels
func (s *centralisedSubscriber) UnsubscribeAll(ctx context.Context, clientID string) error {
	var err error
	s.channelsTracker.ForEach(func(channelName common.ChannelName, subMap *haxmap.Map[string, struct{}]) bool {
		err = s.UnSubscribe(ctx, clientID, channelName)
		if err != nil {
			return false
		}
		return true
	})
	return err
}
