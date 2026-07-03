package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewStore_ReturnsNonNil(t *testing.T) {
	client, _ := newTestRedisClient(t)
	s := NewStore[testItem](client, time.Minute)
	assert.NotNil(t, s)
}

func TestStore_Set_Get_RoundTrip(t *testing.T) {
	client, _ := newTestRedisClient(t)
	s := NewStore[testItem](client, time.Minute)
	ctx := context.Background()

	item := &testItem{Name: "Alice", Value: 42}
	assert.NoError(t, s.Set(ctx, "store:1", item))

	got, err := s.Get(ctx, "store:1")
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "Alice", got.Name)
	assert.Equal(t, 42, got.Value)
}

func TestStore_Get_MissingKey_ReturnsError(t *testing.T) {
	client, _ := newTestRedisClient(t)
	s := NewStore[testItem](client, time.Minute)
	_, err := s.Get(context.Background(), "does-not-exist")
	assert.Error(t, err)
}

func TestStore_Delete_RemovesKey(t *testing.T) {
	client, _ := newTestRedisClient(t)
	s := NewStore[testItem](client, time.Minute)
	ctx := context.Background()

	assert.NoError(t, s.Set(ctx, "store:del", &testItem{Name: "Bob"}))
	assert.NoError(t, s.Delete(ctx, "store:del"))
	_, err := s.Get(ctx, "store:del")
	assert.Error(t, err, "key must be gone after delete")
}
