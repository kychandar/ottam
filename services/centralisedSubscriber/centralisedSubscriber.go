package centralisedSubscriber

import (
	"context"
	"sync"

	"github.com/alphadose/haxmap"
	"github.com/kychandar/ottam/services"
)

type centralisedSubscriber struct {
	lock            sync.RWMutex
	channelsTracker *haxmap.Map[string, *haxmap.Map[string, struct{}]]
	nodeID          string
	cache           services.RedisProtoCache
}

func New(cache services.RedisProtoCache, nodeID string) services.CentralisedSubscriber {
	return &centralisedSubscriber{
		cache:           cache,
		nodeID:          nodeID,
		channelsTracker: haxmap.New[string, *haxmap.Map[string, struct{}]](),
	}
}

// Subscribe adds a client to a channel
func (s *centralisedSubscriber) Subscribe(ctx context.Context, clientID string, channelName string) error {
	s.lock.RLock()
	subMap, exist := s.channelsTracker.Get(channelName)
	s.lock.RUnlock()
	if !exist {
		s.lock.Lock()
		subMap, exist = s.channelsTracker.Get(channelName)
		if !exist {
			err := s.cache.SADD(ctx, "nodesubscription", channelName)
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
func (s *centralisedSubscriber) UnSubscribe(ctx context.Context, clientID string, channelName string) error {
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
			err := s.cache.SREM(ctx, "nodesubscription", channelName)
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
	s.channelsTracker.ForEach(func(channelName string, subMap *haxmap.Map[string, struct{}]) bool {
		err = s.UnSubscribe(ctx, clientID, channelName)
		if err != nil {
			return false
		}
		return true
	})
	return err
}
