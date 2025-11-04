package centralisedSubscriber

import (
	"context"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/gorilla/websocket"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/metrics"
	"github.com/kychandar/ottam/services"
	"github.com/kychandar/ottam/services/pool"
	"github.com/nats-io/nats.go"
	slogctx "github.com/veqryn/slog-context"
)

type centralisedSubscriber struct {
	lock               sync.RWMutex
	channelsTracker    *haxmap.Map[common.ChannelName, *haxmap.Map[string, struct{}]]
	nodeID             common.NodeID
	dataStore          services.DataStore
	pubSubProvider     services.PubSubProvider
	newMessageGetter   func() services.SerializableMessage
	wsWriteChanManager services.WsWriteChanManager
}

func New(
	dataStore services.DataStore,
	nodeID common.NodeID,
	wsWriteChanManager services.WsWriteChanManager,
	newMessageGetter func() services.SerializableMessage,
	pubSubProvider services.PubSubProvider) services.CentralisedSubscriber {
	return &centralisedSubscriber{
		dataStore:          dataStore,
		nodeID:             nodeID,
		channelsTracker:    haxmap.New[common.ChannelName, *haxmap.Map[string, struct{}]](),
		wsWriteChanManager: wsWriteChanManager,
		pubSubProvider:     pubSubProvider,
		newMessageGetter:   newMessageGetter,
	}
}

func (c *centralisedSubscriber) ProcessDownstreamMessages(ctx context.Context) error {
	logger := slogctx.FromCtx(ctx).With("comoponent", "centralisedSubscriber")
	logger.InfoContext(ctx, "starting service")

	objPool := pool.GetGlobalPool()

	err := c.pubSubProvider.CreateStream(common.ServerSubjFormat(string(c.nodeID)), []string{common.ServerSubjFormat(string(c.nodeID))})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		return err
	}

	err = c.pubSubProvider.Subscribe(string(c.nodeID), common.ServerSubjFormat(string(c.nodeID)), func(msgBytes []byte) bool {
		msg := c.newMessageGetter()
		if err := msg.DeserializeFrom(msgBytes); err != nil {
			logger.ErrorContext(ctx, "failed to deserialize message", "err", err, "msg", string(msgBytes))
			return true
		}
		metrics.LatencyHist.WithLabelValues("cent_subscriber_start").Observe(float64(time.Since(msg.GetPublishedTime()).Milliseconds()))
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

		clientCount := 0
		set.ForEach(func(clientID string, s2 struct{}) bool {
			writeChan, exist := c.wsWriteChanManager.GetWriterChannelForClientID(clientID)
			if !exist {
				return true
			}

			writeChan <- *intMsg
			clientCount++
			logger.Info("msg written to channel after", "ms", time.Since(msg.GetPublishedTime()).String())
			return true
		})

		logger.Info("msg completely processed", "ms", time.Since(msg.GetPublishedTime()).String())
		metrics.LatencyHist.WithLabelValues("cent_subscriber_done").Observe(float64(time.Since(msg.GetPublishedTime()).Milliseconds()))
		
		// Return the IntermittenMsg to the pool after all clients have received it
		objPool.ResetIntermittenMsg(intMsg)
		return true
	})
	if err != nil {
		return err
	}
	return nil
}

// Subscribe adds a client to a channel
func (s *centralisedSubscriber) Subscribe(ctx context.Context, clientID string, channelName common.ChannelName) error {
	s.lock.RLock()
	subMap, exist := s.channelsTracker.Get(channelName)
	s.lock.RUnlock()
	if !exist {
		s.lock.Lock()
		subMap, exist = s.channelsTracker.Get(channelName)
		if !exist {

			err := s.dataStore.AddNodeSubscriptionForChannel(ctx, channelName, s.nodeID)
			if err != nil {
				s.lock.Unlock()
				return err
			}
			subMap = haxmap.New[string, struct{}]()
			s.channelsTracker.Set(channelName, subMap)
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
			err := s.dataStore.RemoveNodeSubscriptionForChannel(ctx, channelName, s.nodeID)
			if err != nil {
				s.lock.Unlock()
				return err
			}
			s.channelsTracker.Del(channelName)
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
