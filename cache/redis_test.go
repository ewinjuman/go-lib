package cache

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

// newTestRedisClient is defined in lock_test.go (same package).

func TestNewRedisClient_Success(t *testing.T) {
	client, _ := newTestRedisClient(t)
	assert.NotNil(t, client)
}

func TestNewRedisClient_Failure_UnreachableHost(t *testing.T) {
	_, err := NewRedisClient(RedisOption{Address: "127.0.0.1:19998", PoolSize: 1})
	assert.Error(t, err)
}

func TestResolveAddrs_SingleAddress_UsesAddressField(t *testing.T) {
	addrs := resolveAddrs(RedisOption{Address: "127.0.0.1:6379"})
	assert.Equal(t, []string{"127.0.0.1:6379"}, addrs)
}

func TestResolveAddrs_MultipleAddresses_PrefersAddressesField(t *testing.T) {
	addrs := resolveAddrs(RedisOption{
		Address:   "ignored:6379",
		Addresses: []string{"node1:7000", "node2:7000", "node3:7000"},
	})
	assert.Equal(t, []string{"node1:7000", "node2:7000", "node3:7000"}, addrs)
}

func TestCreateRedisConnection_MultipleAddresses_SelectsClusterClient(t *testing.T) {
	// redis.NewUniversalClient only picks the client type from the option
	// shape; it does not dial until the first command, so this needs no
	// reachable Redis Cluster.
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: resolveAddrs(RedisOption{
			Addresses: []string{"node1:7000", "node2:7000"},
		}),
	})
	defer func() { _ = client.Close() }()

	_, isCluster := client.(*redis.ClusterClient)
	assert.True(t, isCluster, "two or more addresses must select a cluster client")
}

func TestCreateRedisConnection_MasterName_SelectsFailoverClient(t *testing.T) {
	// go-redis implements Sentinel failover as a *redis.Client configured with
	// a Sentinel-aware dialer (NewFailoverClient), not a distinct concrete
	// type — so this just confirms it doesn't route to the cluster client and
	// that the sentinel addresses are threaded through correctly.
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:      resolveAddrs(RedisOption{Address: "sentinel1:26379"}),
		MasterName: "mymaster",
	})
	defer func() { _ = client.Close() }()

	_, isCluster := client.(*redis.ClusterClient)
	assert.False(t, isCluster, "MasterName without RouteByLatency/RouteRandomly/IsClusterMode must not select a cluster client")
}

func TestCreateRedisConnection_SingleAddress_SelectsStandaloneClient(t *testing.T) {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: resolveAddrs(RedisOption{Address: "127.0.0.1:6379"}),
	})
	defer func() { _ = client.Close() }()

	_, isStandalone := client.(*redis.Client)
	assert.True(t, isStandalone, "a single address with no MasterName/IsClusterMode must select a standalone client")
}

func TestWrapRedisClient_WrapsGivenClient(t *testing.T) {
	existing, mr := newTestRedisClient(t)
	wrapped := WrapRedisClient(existing.GetClient())

	ctx := context.Background()
	assert.NoError(t, wrapped.Set(ctx, "shared-key", "shared-value", time.Minute))

	// same underlying redis instance is visible from the original client too
	val, err := existing.Get(ctx, "shared-key")
	assert.NoError(t, err)
	assert.Equal(t, "shared-value", val)
	assert.True(t, mr.Exists("shared-key"))
}

func TestRedisClient_GetClient_ReturnsNonNil(t *testing.T) {
	client, _ := newTestRedisClient(t)
	assert.NotNil(t, client.GetClient())
}

func TestRedisClient_IsAlive_True(t *testing.T) {
	client, _ := newTestRedisClient(t)
	assert.True(t, client.IsAlive())
}

func TestRedisClient_Set_Get_RoundTrip(t *testing.T) {
	client, _ := newTestRedisClient(t)
	ctx := context.Background()

	assert.NoError(t, client.Set(ctx, "key1", "value1", time.Minute))
	val, err := client.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Equal(t, "value1", val)
}

func TestRedisClient_Get_MissingKey_ReturnsError(t *testing.T) {
	client, _ := newTestRedisClient(t)
	_, err := client.Get(context.Background(), "nonexistent")
	assert.Error(t, err)
}

func TestRedisClient_Delete_RemovesKey(t *testing.T) {
	client, _ := newTestRedisClient(t)
	ctx := context.Background()

	assert.NoError(t, client.Set(ctx, "del-key", "v", time.Minute))
	assert.NoError(t, client.Delete(ctx, "del-key"))
	_, err := client.Get(ctx, "del-key")
	assert.Error(t, err, "key should be gone after delete")
}

func TestRedisClient_Pipeline_ReturnsNonNil(t *testing.T) {
	client, _ := newTestRedisClient(t)
	pipe := client.Pipeline()
	assert.NotNil(t, pipe)
	pipe.Discard()
}

func TestRedisClient_Reset_FlushesAllKeys(t *testing.T) {
	client, _ := newTestRedisClient(t)
	ctx := context.Background()

	assert.NoError(t, client.Set(ctx, "k1", "v1", time.Minute))
	assert.NoError(t, client.Set(ctx, "k2", "v2", time.Minute))
	assert.NoError(t, client.Reset())

	_, err1 := client.Get(ctx, "k1")
	_, err2 := client.Get(ctx, "k2")
	assert.Error(t, err1)
	assert.Error(t, err2)
}

func TestRedisClient_SetNX_FirstCallSucceeds_SecondFails(t *testing.T) {
	client, _ := newTestRedisClient(t)
	ctx := context.Background()

	ok1, err := client.SetNX(ctx, "nx-key", "v", time.Minute)
	assert.NoError(t, err)
	assert.True(t, ok1)

	ok2, err := client.SetNX(ctx, "nx-key", "v2", time.Minute)
	assert.NoError(t, err)
	assert.False(t, ok2)
}

func TestRedisClient_Close_NoError(t *testing.T) {
	client, _ := newTestRedisClient(t)
	assert.NoError(t, client.Close())
}
