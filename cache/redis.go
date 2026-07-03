package cache

import (
	"context"
	"runtime"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisClient wraps redis.UniversalClient, so it works identically whether
// the underlying connection is a standalone *redis.Client (including
// Sentinel-backed failover, which go-redis also implements as *redis.Client)
// or a *redis.ClusterClient.
type RedisClient struct {
	client redis.UniversalClient
}

// NewRedisClient func for get connect to redis server. The concrete client
// type is selected from RedisOption: multiple Addresses (or IsClusterMode)
// yields a cluster client, MasterName yields a Sentinel-backed failover
// client, otherwise a standalone client is used.
func NewRedisClient(config RedisOption) (*RedisClient, error) {

	conn, err := createRedisConnection(config)
	if err != nil {
		return nil, err // Return existing redis instance even if connection fails
	}

	return &RedisClient{client: conn}, nil

}

// WrapRedisClient wraps an already-connected redis.UniversalClient (standalone
// *redis.Client, Sentinel-backed failover, or *redis.ClusterClient) instead of
// dialing a new connection. Use this when the caller already owns a shared
// client (e.g. wired elsewhere in the app) and just needs the RedisClient
// helper methods (Store, ObjectCache, Lock, ...) on top of it.
func WrapRedisClient(client redis.UniversalClient) *RedisClient {
	return &RedisClient{client: client}
}

func (c *RedisClient) GetClient() redis.UniversalClient {
	return c.client
}

func (c *RedisClient) IsAlive() bool {
	return pingRedis(c.client) == nil
}

// resolveAddrs returns the seed address list passed to redis.UniversalOptions:
// Addresses when given (cluster/sentinel seed list), otherwise the single
// Address for a standalone connection.
func resolveAddrs(config RedisOption) []string {
	if len(config.Addresses) > 0 {
		return config.Addresses
	}
	return []string{config.Address}
}

func createRedisConnection(config RedisOption) (redis.UniversalClient, error) {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:         resolveAddrs(config),
		Username:      config.Username,
		Password:      config.Password,
		DB:            config.Database,
		PoolSize:      config.PoolSize * runtime.GOMAXPROCS(0),
		MaxRetries:    config.MaxRetries,
		DialTimeout:   config.DialTimeout,
		ReadTimeout:   config.ReadTimeout,
		WriteTimeout:  config.WriteTimeout,
		MasterName:    config.MasterName,
		IsClusterMode: config.IsClusterMode,
	})

	if err := pingRedis(client); err != nil {
		return nil, err
	}
	return client, nil
}

func pingRedis(client redis.UniversalClient) error {
	return client.Ping(context.Background()).Err()
}

func (r *RedisClient) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *RedisClient) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisClient) Pipeline() redis.Pipeliner {
	return r.client.Pipeline()
}

func (c *RedisClient) Reset() error {
	return c.client.FlushAll(context.Background()).Err()
}

func (c *RedisClient) Close() error {
	return c.client.Close()
}

func (c *RedisClient) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	return c.client.SetNX(ctx, key, value, expiration).Result()
}
