package centprocessor

import (
	"context"
	"testing"

	"github.com/kychandar/ottam/ds"
	"github.com/kychandar/ottam/mocks"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMessageProcessor(t *testing.T) {
	ctx := context.Background()
	mockCache := new(mocks.MockRedisProtoCache)
	mockCache.On("SADD", mock.Anything, "users", []string{"bob"}).Return(nil)
	mockCache.On("SMEMBERS", mock.Anything, ChannelSubscCacheKeyFormat("t1")).Return([]string{"node1"}, nil)
	mockCache.On("SMEMBERS", mock.Anything, ChannelSubscCacheKeyFormat("t2")).Return([]string{"node2"}, nil)
	mockCache.On("Close").Return()

	err := mockCache.SADD(ctx, "users", "bob")
	require.NoError(t, err)

	members, err := mockCache.SMEMBERS(ctx, ChannelSubscCacheKeyFormat("t1"))
	require.NoError(t, err)
	require.Equal(t, []string{"node1"}, members)

	members, err = mockCache.SMEMBERS(ctx, ChannelSubscCacheKeyFormat("t2"))
	require.NoError(t, err)
	require.Equal(t, []string{"node2"}, members)

	mockCache.Close()

	mockCache.AssertExpectations(t)

	mockPubSub := new(mocks.MockPubSubProvider)
	t1, err := ds.New("t1", []byte("hi"))
	msg1, err := t1.Serialize()
	require.NoError(t, err)
	mockPubSub.On("Publish", ServerSubjFormat("node1"), msg1).Return(nil)
	t2, err := ds.New("t2", []byte("hi"))
	msg2, err := t2.Serialize()
	require.NoError(t, err)
	mockPubSub.On("Publish", ServerSubjFormat("node2"), msg2).Return(nil)

	var capturedCallback func([]byte) bool
	mockPubSub.On("Subscribe", consumerNameCentProcessor, streamNamePublisher, mock.AnythingOfType("func([]uint8) bool")).
		Run(func(args mock.Arguments) {
			capturedCallback = args.Get(2).(func([]byte) bool)
		}).
		Return(nil)

	require.NoError(t, err)

	centProcessor := NewCentralProcessor(
		context.TODO(),
		mockPubSub,
		mockCache,
		streamNamePublisher,
		"publisher",
		consumerNameCentProcessor,
		ds.NewEmpty,
	)
	err = centProcessor.Start(ctx)
	require.NoError(t, err)

	require.NotNil(t, capturedCallback, "callback should be captured")

	capturedCallback(msg1)
	mockPubSub.AssertCalled(t, "Publish", ServerSubjFormat("node1"), msg1)

	capturedCallback(msg2)
	mockPubSub.AssertCalled(t, "Publish", ServerSubjFormat("node2"), msg2)

	mockPubSub.AssertExpectations(t)

}
