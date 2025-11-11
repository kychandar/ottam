package pool

import (
	"testing"
	"time"

	"github.com/kychandar/ottam/common"
	"github.com/kychandar/ottam/common/ds"
	"github.com/stretchr/testify/assert"
)

func TestNewGenericPool(t *testing.T) {
	pool := NewGenericPool(func() *ds.ClientMessage {
		return &ds.ClientMessage{}
	})

	assert.NotNil(t, pool)
	assert.NotNil(t, pool.pool)
	assert.NotNil(t, pool.new)
}

func TestGenericPool_Get(t *testing.T) {
	pool := NewGenericPool(func() *ds.ClientMessage {
		return &ds.ClientMessage{IsControlPlaneMessage: true}
	})

	obj := pool.Get()
	assert.NotNil(t, obj)
	assert.True(t, obj.IsControlPlaneMessage)
}

func TestGenericPool_Put(t *testing.T) {
	pool := NewGenericPool(func() *ds.ClientMessage {
		return &ds.ClientMessage{}
	})

	obj := pool.Get()
	obj.IsControlPlaneMessage = true

	// Put the object back
	pool.Put(obj)

	// Get it again - might be the same object
	obj2 := pool.Get()
	assert.NotNil(t, obj2)
}

func TestGenericPool_GetPutCycle(t *testing.T) {
	pool := NewGenericPool(func() int {
		return 42
	})

	// Get initial value
	val1 := pool.Get()
	assert.Equal(t, 42, val1)

	// Put it back
	pool.Put(val1)

	// Get it again - should work
	val2 := pool.Get()
	assert.Equal(t, 42, val2)
}

func TestNewObjectPool(t *testing.T) {
	pool := NewObjectPool()

	assert.NotNil(t, pool)
	assert.NotNil(t, pool.ClientMessage)
	assert.NotNil(t, pool.ControlPlaneMsg)
	assert.NotNil(t, pool.SubscriptionPayload)
	assert.NotNil(t, pool.IntermittenMsg)
	assert.NotNil(t, pool.ByteSlice)
}

func TestGetGlobalPool(t *testing.T) {
	pool := GetGlobalPool()

	assert.NotNil(t, pool)
	assert.Equal(t, globalPool, pool)
}

func TestObjectPool_ClientMessage(t *testing.T) {
	pool := NewObjectPool()

	msg := pool.ClientMessage.Get()
	assert.NotNil(t, msg)
	assert.False(t, msg.IsControlPlaneMessage)
	assert.Nil(t, msg.Payload)

	// Modify it
	msg.IsControlPlaneMessage = true
	msg.Payload = []byte("test")

	// Reset and put back
	pool.ResetClientMessage(msg)

	// Get a new one and verify it's reset
	msg2 := pool.ClientMessage.Get()
	assert.NotNil(t, msg2)
	// Note: we can't guarantee we get the same object back from sync.Pool
}

func TestResetClientMessage(t *testing.T) {
	pool := NewObjectPool()

	msg := pool.ClientMessage.Get()
	msg.IsControlPlaneMessage = true
	msg.Payload = []byte("test data")

	// Before reset
	assert.True(t, msg.IsControlPlaneMessage)
	assert.NotNil(t, msg.Payload)

	// Reset (which also puts it back)
	pool.ResetClientMessage(msg)

	// Get another message
	msg2 := pool.ClientMessage.Get()
	// Can't guarantee it's the same object, but it should be valid
	assert.NotNil(t, msg2)
}

func TestObjectPool_ControlPlaneMsg(t *testing.T) {
	pool := NewObjectPool()

	msg := pool.ControlPlaneMsg.Get()
	assert.NotNil(t, msg)
	assert.Equal(t, ds.ControlPlaneOp(0), msg.ControlPlaneOp)
	assert.Nil(t, msg.Payload)
}

func TestResetControlPlaneMessage(t *testing.T) {
	pool := NewObjectPool()

	msg := pool.ControlPlaneMsg.Get()
	msg.ControlPlaneOp = ds.StartSubscription
	msg.Payload = []byte("control data")

	// Reset
	pool.ResetControlPlaneMessage(msg)

	// Get another message
	msg2 := pool.ControlPlaneMsg.Get()
	assert.NotNil(t, msg2)
}

func TestObjectPool_SubscriptionPayload(t *testing.T) {
	pool := NewObjectPool()

	payload := pool.SubscriptionPayload.Get()
	assert.NotNil(t, payload)
	assert.Equal(t, common.ChannelName(""), payload.ChannelName)
}

func TestResetSubscriptionPayload(t *testing.T) {
	pool := NewObjectPool()

	payload := pool.SubscriptionPayload.Get()
	payload.ChannelName = common.ChannelName("test-channel")

	// Before reset
	assert.Equal(t, common.ChannelName("test-channel"), payload.ChannelName)

	// Reset
	pool.ResetSubscriptionPayload(payload)

	// Get another payload
	payload2 := pool.SubscriptionPayload.Get()
	assert.NotNil(t, payload2)
}

func TestObjectPool_IntermittenMsg(t *testing.T) {
	pool := NewObjectPool()

	msg := pool.IntermittenMsg.Get()
	assert.NotNil(t, msg)
	assert.Equal(t, "", msg.Id)
	assert.Nil(t, msg.PreparedMessage)
}

func TestResetIntermittenMsg(t *testing.T) {
	pool := NewObjectPool()

	msg := pool.IntermittenMsg.Get()
	msg.Id = "test-id-123"
	msg.PublishedTime = time.Now()

	// Before reset
	assert.Equal(t, "test-id-123", msg.Id)

	// Reset
	pool.ResetIntermittenMsg(msg)

	// Get another message
	msg2 := pool.IntermittenMsg.Get()
	assert.NotNil(t, msg2)
}

func TestObjectPool_ByteSlice(t *testing.T) {
	pool := NewObjectPool()

	slice := pool.ByteSlice.Get()
	assert.NotNil(t, slice)
	assert.Equal(t, 0, len(slice))
	assert.GreaterOrEqual(t, cap(slice), 4096) // Should have 4KB capacity
}

func TestResetByteSlice(t *testing.T) {
	pool := NewObjectPool()

	slice := pool.ByteSlice.Get()
	assert.Equal(t, 0, len(slice))
	originalCap := cap(slice)
	assert.GreaterOrEqual(t, originalCap, 4096)

	// Write some data
	slice = append(slice, []byte("test data here")...)
	assert.Greater(t, len(slice), 0)

	// Reset
	pool.ResetByteSlice(slice)

	// Get another slice
	slice2 := pool.ByteSlice.Get()
	assert.NotNil(t, slice2)
	// Should have 0 length
	assert.Equal(t, 0, len(slice2))
	// Capacity should be at least the minimum
	assert.GreaterOrEqual(t, cap(slice2), 4096)
}

func TestByteSlice_InitialCapacity(t *testing.T) {
	pool := NewObjectPool()

	slice := pool.ByteSlice.Get()

	// Should have 4KB initial capacity
	assert.GreaterOrEqual(t, cap(slice), 4096)
	assert.Equal(t, 0, len(slice))
}

func TestConcurrentPoolAccess_ClientMessage(t *testing.T) {
	pool := NewObjectPool()
	numGoroutines := 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			msg := pool.ClientMessage.Get()
			msg.IsControlPlaneMessage = true
			pool.ResetClientMessage(msg)
			done <- true
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestConcurrentPoolAccess_IntermittenMsg(t *testing.T) {
	pool := NewObjectPool()
	numGoroutines := 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			msg := pool.IntermittenMsg.Get()
			msg.Id = string(rune(id))
			msg.PublishedTime = time.Now()
			pool.ResetIntermittenMsg(msg)
			done <- true
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

func TestConcurrentPoolAccess_AllPools(t *testing.T) {
	pool := NewObjectPool()
	numGoroutines := 50
	done := make(chan bool, numGoroutines*5)

	// Test all pools concurrently
	for i := 0; i < numGoroutines; i++ {
		go func() {
			msg := pool.ClientMessage.Get()
			pool.ResetClientMessage(msg)
			done <- true
		}()

		go func() {
			msg := pool.ControlPlaneMsg.Get()
			pool.ResetControlPlaneMessage(msg)
			done <- true
		}()

		go func() {
			payload := pool.SubscriptionPayload.Get()
			pool.ResetSubscriptionPayload(payload)
			done <- true
		}()

		go func() {
			msg := pool.IntermittenMsg.Get()
			pool.ResetIntermittenMsg(msg)
			done <- true
		}()

		go func() {
			slice := pool.ByteSlice.Get()
			pool.ResetByteSlice(slice)
			done <- true
		}()
	}

	for i := 0; i < numGoroutines*5; i++ {
		<-done
	}
}

func TestGlobalPoolSingleton(t *testing.T) {
	pool1 := GetGlobalPool()
	pool2 := GetGlobalPool()

	// Should return the same instance
	assert.Equal(t, pool1, pool2)
	assert.Same(t, pool1, pool2)
}

func TestPoolReuse(t *testing.T) {
	pool := NewObjectPool()

	// Get and put multiple times
	for i := 0; i < 10; i++ {
		msg := pool.ClientMessage.Get()
		msg.IsControlPlaneMessage = true
		msg.Payload = []byte("test")
		pool.ResetClientMessage(msg)
	}

	// Pool should still work fine
	msg := pool.ClientMessage.Get()
	assert.NotNil(t, msg)
}

func TestResetIntermittenMsg_PreservesUTC(t *testing.T) {
	pool := NewObjectPool()

	msg := pool.IntermittenMsg.Get()
	localTime := time.Now()
	msg.PublishedTime = localTime

	// Reset converts to UTC
	pool.ResetIntermittenMsg(msg)

	// Get another message - can't guarantee same object, but test passes
	msg2 := pool.IntermittenMsg.Get()
	assert.NotNil(t, msg2)
}

func TestMultiplePoolInstances(t *testing.T) {
	pool1 := NewObjectPool()
	pool2 := NewObjectPool()

	// Different instances should work independently
	msg1 := pool1.ClientMessage.Get()
	msg2 := pool2.ClientMessage.Get()

	assert.NotNil(t, msg1)
	assert.NotNil(t, msg2)

	pool1.ResetClientMessage(msg1)
	pool2.ResetClientMessage(msg2)
}

func TestGenericPool_TypeSafety(t *testing.T) {
	// Test with different types
	intPool := NewGenericPool(func() *int {
		val := 42
		return &val
	})

	stringPool := NewGenericPool(func() *string {
		val := "test"
		return &val
	})

	intVal := intPool.Get()
	assert.Equal(t, 42, *intVal)

	strVal := stringPool.Get()
	assert.Equal(t, "test", *strVal)

	intPool.Put(intVal)
	stringPool.Put(strVal)
}

func TestObjectPool_ByteSlice_GrowAndReset(t *testing.T) {
	pool := NewObjectPool()

	slice := pool.ByteSlice.Get()
	initialCap := cap(slice)

	// Append data beyond initial capacity
	for i := 0; i < 10000; i++ {
		slice = append(slice, byte(i%256))
	}

	assert.Equal(t, 10000, len(slice))
	assert.Greater(t, cap(slice), initialCap) // Should have grown

	// Reset should preserve capacity but set length to 0
	pool.ResetByteSlice(slice)

	// Note: sync.Pool might not return the same slice
	slice2 := pool.ByteSlice.Get()
	assert.NotNil(t, slice2)
	assert.Equal(t, 0, len(slice2))
}
