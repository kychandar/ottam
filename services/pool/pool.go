package pool

import (
	"sync"

	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/ds"
)

// GenericPool is a generic sync.Pool wrapper
type GenericPool[T any] struct {
	pool *sync.Pool
	new  func() T
}

// NewGenericPool creates a new generic pool with a factory function
func NewGenericPool[T any](factory func() T) *GenericPool[T] {
	return &GenericPool[T]{
		pool: &sync.Pool{
			New: func() interface{} {
				return factory()
			},
		},
		new: factory,
	}
}

// Get retrieves an object from the pool or creates a new one
func (p *GenericPool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put returns an object to the pool
func (p *GenericPool[T]) Put(obj T) {
	p.pool.Put(obj)
}

// ObjectPool holds all the pools for frequently used objects
type ObjectPool struct {
	ClientMessage      *GenericPool[*ds.ClientMessage]
	ControlPlaneMsg    *GenericPool[*ds.ControlPlaneMessage]
	SubscriptionPayload *GenericPool[*ds.SubscriptionPayload]
	IntermittenMsg     *GenericPool[*common.IntermittenMsg]
	ByteSlice          *GenericPool[[]byte]
}

// NewObjectPool creates and initializes all object pools
func NewObjectPool() *ObjectPool {
	return &ObjectPool{
		ClientMessage: NewGenericPool(func() *ds.ClientMessage {
			return &ds.ClientMessage{}
		}),
		ControlPlaneMsg: NewGenericPool(func() *ds.ControlPlaneMessage {
			return &ds.ControlPlaneMessage{}
		}),
		SubscriptionPayload: NewGenericPool(func() *ds.SubscriptionPayload {
			return &ds.SubscriptionPayload{}
		}),
		IntermittenMsg: NewGenericPool(func() *common.IntermittenMsg {
			return &common.IntermittenMsg{}
		}),
		ByteSlice: NewGenericPool(func() []byte {
			return make([]byte, 0, 4096) // 4KB initial buffer for JSON operations
		}),
	}
}

// Global pool instance
var globalPool = NewObjectPool()

// GetGlobalPool returns the global object pool
func GetGlobalPool() *ObjectPool {
	return globalPool
}

// Reset helper functions to clear pooled objects before reuse
func (p *ObjectPool) ResetClientMessage(msg *ds.ClientMessage) {
	msg.IsControlPlaneMessage = false
	msg.Payload = nil
	p.ClientMessage.Put(msg)
}

func (p *ObjectPool) ResetControlPlaneMessage(msg *ds.ControlPlaneMessage) {
	msg.Payload = nil
	p.ControlPlaneMsg.Put(msg)
}

func (p *ObjectPool) ResetSubscriptionPayload(payload *ds.SubscriptionPayload) {
	payload.ChannelName = ""
	p.SubscriptionPayload.Put(payload)
}

func (p *ObjectPool) ResetIntermittenMsg(msg *common.IntermittenMsg) {
	msg.PublishedTime = msg.PublishedTime.UTC()
	msg.Id = ""
	msg.PreparedMessage = nil
	p.IntermittenMsg.Put(msg)
}

func (p *ObjectPool) ResetByteSlice(b []byte) {
	p.ByteSlice.Put(b[:0]) // Reset slice to 0 length, keep capacity
}

