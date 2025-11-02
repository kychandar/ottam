package services

import "context"

type PubSubProvider interface {
	CreateStream(streamName string, subjects []string) error
	Publish(subjectName string, data []byte) error
	Subscribe(consumerName string, subjectName string, callBack func(msg []byte)) error
	UnSubscribe(consumerName string) error
	Close() error
}

type RedisProtoCache interface {
	SADD(ctx context.Context, key string, member ...string) error
	SREM(ctx context.Context, key string, member ...string) error
	SMEMBERS(ctx context.Context, key string) ([]string, error)
	Close()
}
