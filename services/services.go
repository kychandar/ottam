package services

// fireAndForget pubSub service
import (
	"context"
)

type FireAndForgetPubSubService interface {
	Publish(ctx context.Context, subject string, data []byte) error
}
