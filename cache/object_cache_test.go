package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testItem struct {
	Name  string
	Value int
}

func newTestObjectCache(t *testing.T, policy Policy) *ObjectCache[testItem] {
	t.Helper()
	client, _ := newTestRedisClient(t)
	return NewObjectCache[testItem](client, ObjectCacheConfig{
		TTL:    time.Minute,
		Policy: policy,
	})
}

func TestObjectCache_Set_Get_RoundTrip(t *testing.T) {
	oc := newTestObjectCache(t, PolicyAll())
	ctx := context.Background()

	item := &testItem{Name: "Alice", Value: 42}
	assert.NoError(t, oc.Set(ctx, "obj:1", item))

	got, err := oc.Get(ctx, "obj:1")
	assert.NoError(t, err)
	assert.Equal(t, "Alice", got.Name)
	assert.Equal(t, 42, got.Value)
}

func TestObjectCache_Get_MissingKey_ReturnsError(t *testing.T) {
	oc := newTestObjectCache(t, PolicyAll())
	_, err := oc.Get(context.Background(), "does-not-exist")
	assert.Error(t, err)
}

func TestObjectCache_Delete_RemovesKey(t *testing.T) {
	oc := newTestObjectCache(t, PolicyAll())
	ctx := context.Background()

	assert.NoError(t, oc.Set(ctx, "obj:del", &testItem{Name: "Bob"}))
	assert.NoError(t, oc.Delete(ctx, "obj:del"))
	_, err := oc.Get(ctx, "obj:del")
	assert.Error(t, err)
}

func TestObjectCache_Set_PolicyDenied_SkipsCache(t *testing.T) {
	oc := newTestObjectCache(t, PolicyNone())
	ctx := context.Background()

	assert.NoError(t, oc.Set(ctx, "obj:denied", &testItem{Name: "Eve"}))
	_, err := oc.Get(ctx, "obj:denied")
	assert.Error(t, err, "policy denied — value must not be stored")
}

func TestObjectCache_Set_PolicyKeys_SelectiveCache(t *testing.T) {
	oc := newTestObjectCache(t, PolicyKeys("obj:allowed"))
	ctx := context.Background()

	assert.NoError(t, oc.Set(ctx, "obj:allowed", &testItem{Name: "Yes"}))
	assert.NoError(t, oc.Set(ctx, "obj:blocked", &testItem{Name: "No"}))

	got, err := oc.Get(ctx, "obj:allowed")
	assert.NoError(t, err)
	assert.Equal(t, "Yes", got.Name)

	_, err = oc.Get(ctx, "obj:blocked")
	assert.Error(t, err, "policy excluded this key")
}

func TestObjectCache_GetOrLoad_CacheHit_SkipsLoader(t *testing.T) {
	oc := newTestObjectCache(t, PolicyAll())
	ctx := context.Background()
	item := &testItem{Name: "Cached", Value: 1}
	assert.NoError(t, oc.Set(ctx, "obj:hit", item))

	var loaderCalls int32
	result, err := oc.GetOrLoad(ctx, "obj:hit", func() (*testItem, error) {
		atomic.AddInt32(&loaderCalls, 1)
		return item, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "Cached", result.Name)
	assert.Equal(t, int32(0), atomic.LoadInt32(&loaderCalls))
}

func TestObjectCache_GetOrLoad_CacheMiss_LoadsAndPopulates(t *testing.T) {
	oc := newTestObjectCache(t, PolicyAll())
	ctx := context.Background()
	item := &testItem{Name: "Loaded", Value: 2}

	var loaderCalls int32
	result, err := oc.GetOrLoad(ctx, "obj:miss", func() (*testItem, error) {
		atomic.AddInt32(&loaderCalls, 1)
		return item, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "Loaded", result.Name)
	assert.Equal(t, int32(1), atomic.LoadInt32(&loaderCalls))

	cached, err := oc.Get(ctx, "obj:miss")
	assert.NoError(t, err)
	assert.Equal(t, "Loaded", cached.Name)
}

func TestObjectCache_GetOrLoad_LoaderError_NotCached(t *testing.T) {
	oc := newTestObjectCache(t, PolicyAll())
	ctx := context.Background()

	_, err := oc.GetOrLoad(ctx, "obj:err", func() (*testItem, error) {
		return nil, fmt.Errorf("source down")
	})
	assert.Error(t, err)

	_, cacheErr := oc.Get(ctx, "obj:err")
	assert.Error(t, cacheErr, "failed load must not poison the cache")
}

func TestObjectCache_GetOrLoad_PolicyDenied_BypassesCache(t *testing.T) {
	oc := newTestObjectCache(t, PolicyNone())
	ctx := context.Background()
	item := &testItem{Name: "Direct", Value: 3}

	var loaderCalls int32
	result, err := oc.GetOrLoad(ctx, "obj:bypass", func() (*testItem, error) {
		atomic.AddInt32(&loaderCalls, 1)
		return item, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "Direct", result.Name)
	assert.Equal(t, int32(1), atomic.LoadInt32(&loaderCalls))

	_, cacheErr := oc.Get(ctx, "obj:bypass")
	assert.Error(t, cacheErr, "policy denied — value must not be stored")
}

func TestObjectCache_GetOrLoad_ConcurrentMiss_CoalescesLoader(t *testing.T) {
	oc := newTestObjectCache(t, PolicyAll())
	item := &testItem{Name: "Coalesced", Value: 4}

	var loaderCalls int32
	const concurrency = 25

	var wg sync.WaitGroup
	wg.Add(concurrency)
	for range concurrency {
		go func() {
			defer wg.Done()
			result, err := oc.GetOrLoad(context.Background(), "obj:stampede", func() (*testItem, error) {
				atomic.AddInt32(&loaderCalls, 1)
				time.Sleep(50 * time.Millisecond)
				return item, nil
			})
			assert.NoError(t, err)
			if result != nil {
				assert.Equal(t, "Coalesced", result.Name)
			}
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), atomic.LoadInt32(&loaderCalls),
		"stampede protection must coalesce concurrent misses into a single load")
}

func TestObjectCache_LoadWithLock_RecheckCacheHit(t *testing.T) {
	client, _ := newTestRedisClient(t)
	oc := NewObjectCache[testItem](client, ObjectCacheConfig{
		TTL:    time.Minute,
		Policy: PolicyAll(),
	})
	ctx := context.Background()
	key := "obj:recheck"

	item := &testItem{Name: "PrePopulated", Value: 99}
	assert.NoError(t, oc.store.Set(ctx, key, item))

	var loaderCalled bool
	result, err := oc.loadWithLock(ctx, key, func() (*testItem, error) {
		loaderCalled = true
		return item, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "PrePopulated", result.Name)
	assert.False(t, loaderCalled, "loader must not be called when re-check finds cached value")
}

func TestObjectCache_LoadWithLock_LockNotAcquired_FallsBackToLoader(t *testing.T) {
	client, _ := newTestRedisClient(t)
	oc := NewObjectCache[testItem](client, ObjectCacheConfig{
		TTL:    time.Minute,
		Policy: PolicyAll(),
	})
	ctx := context.Background()
	key := "obj:locked"

	lock, acquired, err := client.TryLock(ctx, oc.lockPrefix+key, oc.lockTTL)
	assert.NoError(t, err)
	assert.True(t, acquired)
	defer func() { _ = lock.Unlock(ctx) }()

	expected := &testItem{Name: "Fallback", Value: 77}

	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	result, err := oc.loadWithLock(cancelledCtx, key, func() (*testItem, error) {
		return expected, nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "Fallback", result.Name)
}

func TestObjectCache_WaitForCache_ContextCancelled_ReturnsFalse(t *testing.T) {
	oc := newTestObjectCache(t, PolicyAll())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got, ok := oc.waitForCache(ctx, "obj:cancelled")
	assert.False(t, ok)
	assert.Nil(t, got)
}
