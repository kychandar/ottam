package valkey

import (
	"context"
	"sync"

	"github.com/alphadose/haxmap"
	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/config"
	"github.com/kychandar/ottam/services"
	"github.com/valkey-io/valkey-go"
)

// ValkeySetCache implements InMemCache using valkey-go client.
type ValkeySetCache struct {
	client valkey.Client
	cache  *haxmap.Map[common.ChannelName, []string]
	lock   sync.Mutex
}

// NewValkeySetCache returns a new instance.
func NewValkeySetCache(config *config.Config) (services.DataStore, error) {
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:  config.RedisProtoCache.Addr,
		DisableCache: false,
	})
	return &ValkeySetCache{
		client: client,
		cache:  haxmap.New[common.ChannelName, []string](),
	}, err
}

// Close gracefully shuts down the Valkey client.
func (c *ValkeySetCache) Close() {
	if c.client == nil {
	}
	c.client.Close()
}

// AddNodeSubscriptionForChannel implements services.DataStore.
func (c *ValkeySetCache) AddNodeSubscriptionForChannel(ctx context.Context, channelName common.ChannelName, nodeID common.NodeID) error {
	cmd := c.client.B().Sadd().Key(common.ChannelSubscCacheKeyFormat(string(channelName))).Member(string(nodeID)).Build()
	if err := c.client.Do(ctx, cmd).Error(); err != nil {
		return err
	}
	return nil
}

func (c *ValkeySetCache) ListNodesSubscribedForChannel(ctx context.Context, channelName common.ChannelName) ([]common.NodeID, error) {
	cmd := c.client.B().Smembers().Key(common.ChannelSubscCacheKeyFormat(string(channelName))).Build()
	result, err := c.client.Do(ctx, cmd).ToArray()
	if err != nil {
		return nil, err
	}
	nodes := make([]common.NodeID, 0, len(result))
	for _, x := range result {
		val, err := x.ToString()
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, common.NodeID(val))
	}
	return nodes, nil
}

func (c *ValkeySetCache) RemoveNodeSubscriptionForChannel(ctx context.Context, channelName common.ChannelName, nodeID common.NodeID) error {
	cmd := c.client.B().Srem().Key(common.ChannelSubscCacheKeyFormat(string(channelName))).Member(string(nodeID)).Build()
	if err := c.client.Do(ctx, cmd).Error(); err != nil {
		return err
	}
	return nil
}
