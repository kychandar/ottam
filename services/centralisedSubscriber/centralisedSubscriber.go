package centralisedsubscriber

import (
	"context"
	"slices"
	"sync"

	"github.com/kychandar/ottam/services"
	slogctx "github.com/veqryn/slog-context"
)

type centralisedSubscriber struct {
	mu            sync.RWMutex
	subjectHelper map[string]subjectHelper // channel -> wsID ->

	pubSubProvider services.PubSubProvider
}

func New(ctx context.Context, pubSubProvider services.PubSubProvider) services.CentralisedSubscriber {
	return &centralisedSubscriber{
		pubSubProvider: pubSubProvider,
		subjectHelper:  make(map[string]subjectHelper),
	}
}

type subjectHelper struct {
	bridges      map[string]services.SubscriptionCallBack
	unsubscriber func() error
}

// Subscribe implements services.CentralisedSubscriber.
func (c *centralisedSubscriber) Subscribe(ctx context.Context, subject string, wsConnID string, callBack services.SubscriptionCallBack) error {
	logger := slogctx.FromCtx(ctx).With("channel", subject, "wsConnID", wsConnID)
	c.mu.Lock()
	defer c.mu.Unlock()

	_, exist := c.subjectHelper[subject]
	if !exist {
		dataChan := make(chan []byte)
		unsubScriber, err := c.pubSubProvider.Subscribe(ctx, subject, func(ctx context.Context, subject string, data []byte) {
			dataChan <- data
		})
		if err != nil {
			return err
		}
		c.subjectHelper[subject] = subjectHelper{bridges: make(map[string]services.SubscriptionCallBack), unsubscriber: unsubScriber}
		go c.S(subject, dataChan)
	}

	c.subjectHelper[subject].bridges[wsConnID] = callBack
	logger.InfoContext(ctx, "subscription initialised")
	return nil
}

func (c *centralisedSubscriber) S(subject string, dataChan chan []byte) {
	for {
		select {
		case msg := <-dataChan:
			c.mu.Lock()
			for _, callBack := range c.subjectHelper[subject].bridges {
				go func(msg []byte) {
					callBack(context.TODO(), subject, msg)
				}(slices.Clone(msg))
			}
			c.mu.Unlock()
		}
	}
}

// Unsubscribe implements services.CentralisedSubscriber.
func (c *centralisedSubscriber) Unsubscribe(ctx context.Context, subject string, wsConnID string) error {
	logger := slogctx.FromCtx(ctx).With("channel", subject, "wsConnID", wsConnID)

	c.mu.Lock()
	defer c.mu.Unlock()

	_, exist := c.subjectHelper[subject]
	if !exist {
		return nil
	}

	err := c.subjectHelper[subject].unsubscriber()
	if err != nil {
		return nil
	}

	delete(c.subjectHelper[subject].bridges, wsConnID)
	if len(c.subjectHelper[subject].bridges) < 1 {
		err := c.subjectHelper[subject].unsubscriber()
		if err != nil {
			return nil
		}
	}

	delete(c.subjectHelper, subject)
	logger.InfoContext(ctx, "subscription deleted")
	return nil
}
