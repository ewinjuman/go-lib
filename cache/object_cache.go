package cache

import (
	"context"
	"time"

	"golang.org/x/sync/singleflight"
)

type ObjectCacheConfig struct {
	// TTL is how long a cached value is kept before it's treated as a miss.
	TTL time.Duration
	// LockTTL is how long the stampede-protection lock is held before it
	// auto-expires (in case the holder crashes mid-load). Default: 5s.
	LockTTL time.Duration
	// LockWait is how long a caller that lost the lock waits, polling the
	// cache, before giving up and loading directly. Default: 3s.
	LockWait time.Duration
	// LockPoll is the interval between cache-poll attempts while waiting for
	// the lock holder to populate the cache. Default: 100ms.
	LockPoll time.Duration
	// LockPrefix is prepended to the cache key to form the lock's Redis key,
	// so locks and cached values never collide. Default: "lock:".
	LockPrefix string
	// Policy decides whether a given key is cacheable at all; keys the
	// policy rejects bypass both the cache and stampede protection entirely.
	// Default: PolicyAll() (cache everything).
	Policy Policy
}

func (c *ObjectCacheConfig) withDefaults() {
	if c.LockTTL == 0 {
		c.LockTTL = 5 * time.Second
	}
	if c.LockWait == 0 {
		c.LockWait = 3 * time.Second
	}
	if c.LockPoll == 0 {
		c.LockPoll = 100 * time.Millisecond
	}
	if c.LockPrefix == "" {
		c.LockPrefix = "lock:"
	}
	if c.Policy == nil {
		c.Policy = PolicyAll()
	}
}

type ObjectCache[T any] struct {
	store      *Store[T]
	client     *RedisClient
	group      singleflight.Group
	policy     Policy
	lockPrefix string
	lockTTL    time.Duration
	lockWait   time.Duration
	lockPoll   time.Duration
}

func NewObjectCache[T any](client *RedisClient, cfg ObjectCacheConfig) *ObjectCache[T] {
	cfg.withDefaults()
	return &ObjectCache[T]{
		store:      NewStore[T](client, cfg.TTL),
		client:     client,
		policy:     cfg.Policy,
		lockPrefix: cfg.LockPrefix,
		lockTTL:    cfg.LockTTL,
		lockWait:   cfg.LockWait,
		lockPoll:   cfg.LockPoll,
	}
}

func (c *ObjectCache[T]) Get(ctx context.Context, key string) (*T, error) {
	return c.store.Get(ctx, key)
}

func (c *ObjectCache[T]) Set(ctx context.Context, key string, val *T) error {
	if !c.policy(key) {
		return nil
	}
	return c.store.Set(ctx, key, val)
}

func (c *ObjectCache[T]) Delete(ctx context.Context, key string) error {
	return c.store.Delete(ctx, key)
}

func (c *ObjectCache[T]) GetOrLoad(ctx context.Context, key string, loader func() (*T, error)) (*T, error) {
	if !c.policy(key) {
		return loader()
	}

	if cached, err := c.store.Get(ctx, key); err == nil {
		return cached, nil
	}

	v, err, _ := c.group.Do(key, func() (any, error) {
		if cached, err := c.store.Get(ctx, key); err == nil {
			return cached, nil
		}
		return c.loadWithLock(ctx, key, loader)
	})
	if err != nil {
		return nil, err
	}
	return v.(*T), nil
}

func (c *ObjectCache[T]) loadWithLock(ctx context.Context, key string, loader func() (*T, error)) (*T, error) {
	lock, acquired, lockErr := c.client.TryLock(ctx, c.lockPrefix+key, c.lockTTL)
	if lockErr == nil && acquired {
		defer func() { _ = lock.Unlock(ctx) }()

		if cached, err := c.store.Get(ctx, key); err == nil {
			return cached, nil
		}

		val, err := loader()
		if err != nil {
			return nil, err
		}
		_ = c.store.Set(ctx, key, val)
		return val, nil
	}

	if cached, ok := c.waitForCache(ctx, key); ok {
		return cached, nil
	}

	return loader()
}

func (c *ObjectCache[T]) waitForCache(ctx context.Context, key string) (*T, bool) {
	deadline := time.Now().Add(c.lockWait)
	ticker := time.NewTicker(c.lockPoll)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, false
		case <-ticker.C:
			if cached, err := c.store.Get(ctx, key); err == nil {
				return cached, true
			}
		}
	}
	return nil, false
}
