package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

// MockRedisProtoCache is a mock implementation of RedisProtoCache.
type MockRedisProtoCache struct {
	mock.Mock
}

func (m *MockRedisProtoCache) SADD(ctx context.Context, key string, member ...string) error {
	args := m.Called(ctx, key, member)
	return args.Error(0)
}

func (m *MockRedisProtoCache) SREM(ctx context.Context, key string, member ...string) error {
	args := m.Called(ctx, key, member)
	return args.Error(0)
}

func (m *MockRedisProtoCache) SMEMBERS(ctx context.Context, key string) ([]string, error) {
	args := m.Called(ctx, key)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockRedisProtoCache) Close() {
	m.Called()
}
