package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
)

func newTestRedisClient(t *testing.T) (*RedisClient, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)

	client, err := NewRedisClient(RedisOption{Address: mr.Addr(), PoolSize: 1})
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	return client, mr
}

func TestRedisClient_TryLock_AcquiresWhenFree(t *testing.T) {
	client, _ := newTestRedisClient(t)
	ctx := context.Background()

	lock, ok, err := client.TryLock(ctx, "lock:key", time.Second)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.NotNil(t, lock)
}

func TestRedisClient_TryLock_FailsWhenHeld(t *testing.T) {
	client, _ := newTestRedisClient(t)
	ctx := context.Background()

	_, ok, err := client.TryLock(ctx, "lock:key", time.Second)
	assert.NoError(t, err)
	assert.True(t, ok)

	_, ok, err = client.TryLock(ctx, "lock:key", time.Second)
	assert.NoError(t, err)
	assert.False(t, ok)
}

func TestRedisClient_Unlock_ReleasesForOthers(t *testing.T) {
	client, _ := newTestRedisClient(t)
	ctx := context.Background()

	lock, ok, err := client.TryLock(ctx, "lock:key", time.Second)
	assert.NoError(t, err)
	assert.True(t, ok)

	assert.NoError(t, lock.Unlock(ctx))

	_, ok, err = client.TryLock(ctx, "lock:key", time.Second)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestRedisClient_Unlock_DoesNotReleaseSomeoneElsesLock(t *testing.T) {
	client, mr := newTestRedisClient(t)
	ctx := context.Background()

	first, ok, err := client.TryLock(ctx, "lock:key", 50*time.Millisecond)
	assert.NoError(t, err)
	assert.True(t, ok)

	// simulate expiry (miniredis uses a virtual clock), then another holder acquires the lock
	mr.FastForward(60 * time.Millisecond)
	second, ok, err := client.TryLock(ctx, "lock:key", time.Second)
	assert.NoError(t, err)
	assert.True(t, ok)

	// the original (expired) holder must not be able to release the new lock
	assert.NoError(t, first.Unlock(ctx))
	_, ok, err = client.TryLock(ctx, "lock:key", time.Second)
	assert.NoError(t, err)
	assert.False(t, ok, "second holder's lock must still be active")

	assert.NoError(t, second.Unlock(ctx))
}
