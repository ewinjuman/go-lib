package cache

import (
	"context"
	"time"

	"github.com/ewinjuman/go-lib/v2/utils/convert"
)

// Store is a generic Redis cache for a single value type T.
// TTL is fixed per Store instance; key construction is the caller's responsibility.
type Store[T any] struct {
	client *RedisClient
	ttl    time.Duration
}

func NewStore[T any](client *RedisClient, ttl time.Duration) *Store[T] {
	return &Store[T]{client: client, ttl: ttl}
}

func (s *Store[T]) Get(ctx context.Context, key string) (*T, error) {
	data, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	var v T
	convert.StringToObject(data, &v)
	return &v, nil
}

func (s *Store[T]) Set(ctx context.Context, key string, val *T) error {
	return s.client.Set(ctx, key, convert.ObjectToString(val), s.ttl)
}

func (s *Store[T]) Delete(ctx context.Context, key string) error {
	return s.client.Delete(ctx, key)
}
