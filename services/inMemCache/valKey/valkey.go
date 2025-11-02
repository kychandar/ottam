package valkey

import (
	"context"
	"fmt"

	"github.com/kychandar/ottam/config"
	"github.com/kychandar/ottam/services"
	"github.com/valkey-io/valkey-go"
)

// ValkeySetCache implements InMemCache using valkey-go client.
type ValkeySetCache struct {
	client valkey.Client
}

// NewValkeySetCache returns a new instance.
func NewValkeySetCache(config config.Config) (services.RedisProtoCache, error) {
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress:  config.RedisProtoCache.Addr,
		DisableCache: true,
	})
	return &ValkeySetCache{
		client: client,
	}, err
}

// SADD adds one or more members to a set.
func (c *ValkeySetCache) SADD(ctx context.Context, key string, member ...string) error {
	// Build command: “SADD key member1 member2 ...”
	cmd := c.client.B().Sadd().Key(key).Member(member...).Build()
	if err := c.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("SADD failed for key %q: %w", key, err)
	}
	return nil
}

// SREM removes one or more members from a set.
func (c *ValkeySetCache) SREM(ctx context.Context, key string, member ...string) error {
	cmd := c.client.B().Srem().Key(key).Member(member...).Build()
	if err := c.client.Do(ctx, cmd).Error(); err != nil {
		return fmt.Errorf("SREM failed for key %q: %w", key, err)
	}
	return nil
}

// SMEMBERS retrieves all members of the set.
func (c *ValkeySetCache) SMEMBERS(ctx context.Context, key string) ([]string, error) {
	cmd := c.client.B().Smembers().Key(key).Build()
	result, err := c.client.Do(ctx, cmd).AsStrSlice()
	if err != nil {
		return nil, fmt.Errorf("SMEMBERS failed for key %q: %w", key, err)
	}
	return result, nil
}

// Close gracefully shuts down the Valkey client.
func (c *ValkeySetCache) Close() {
	if c.client == nil {
	}
	c.client.Close()
}
