package valkey

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/kychandar/ottam/config"
	"github.com/kychandar/ottam/services"
)

func newTestCache(t *testing.T) services.RedisProtoCache {
	t.Helper()

	// Start an in-memory Redis-compatible server
	mr := miniredis.RunT(t)

	cfg := config.Config{}
	cfg.RedisProtoCache.Addr = []string{mr.Addr()}
	client, err := NewValkeySetCache(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})
	return client
}

func TestValkeySetCache_SADD_SMEMBERS_SREM(t *testing.T) {
	cache := newTestCache(t)
	key := "test:set"

	ctx := context.Background()
	// --- Test SADD ---
	err := cache.SADD(ctx, key, "a", "b", "c")
	require.NoError(t, err, "SADD should not error")

	// --- Test SMEMBERS ---
	members, err := cache.SMEMBERS(ctx, key)
	require.NoError(t, err, "SMEMBERS should not error")
	require.ElementsMatch(t, []string{"a", "b", "c"}, members, "SMEMBERS should return all added elements")

	// --- Test SREM ---
	err = cache.SREM(ctx, key, "b")
	require.NoError(t, err, "SREM should not error")

	members, err = cache.SMEMBERS(ctx, key)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"a", "c"}, members, "SREM should remove only 'b'")

	// --- Test idempotent remove ---
	err = cache.SREM(ctx, key, "nonexistent")
	require.NoError(t, err, "SREM should not error even for nonexistent member")
}

func TestValkeySetCache_EmptySet(t *testing.T) {
	cache := newTestCache(t)
	key := "empty:set"
	ctx := context.Background()

	// Calling SMEMBERS on a key that doesn't exist should return empty slice, not error
	members, err := cache.SMEMBERS(ctx, key)
	require.NoError(t, err)
	require.Empty(t, members)
}
