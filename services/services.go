package services

import (
	"context"
	"time"
)

type PubSubProvider interface {
	CreateStream(streamName string, subjects []string) error
	Publish(subjectName string, data []byte) error
	Subscribe(consumerName string, subjectName string, callBack func(msg []byte) bool) error
	UnSubscribe(consumerName string) error
	Close() error
}

type RedisProtoCache interface {
	SADD(ctx context.Context, key string, member ...string) error
	SREM(ctx context.Context, key string, member ...string) error
	SMEMBERS(ctx context.Context, key string) ([]string, error)
	Close()
}

type CentralProcessor interface {
	Start(ctx context.Context) error
	Stop(context.Context) error
}

type SerializableMessage interface {
	Serialize() ([]byte, error)
	DeserializeFrom([]byte) error
	GetChannelName() string
	GetPublishedTime() time.Time
}
