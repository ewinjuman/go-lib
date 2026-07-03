package cache

import "time"

type RedisOption struct {
	// Address is a single host:port. For cluster or Sentinel deployments with
	// multiple seed nodes, use Addresses instead (Address is ignored whenever
	// Addresses is non-empty).
	Address   string
	Addresses []string

	// Username authenticates via Redis ACL (Redis 6+); leave empty for the
	// legacy single-password auth scheme.
	Username string
	// Password is the Redis AUTH password (or ACL user password when
	// Username is set); leave empty if the server requires no auth.
	Password string
	// Database selects the logical DB index (SELECT n) after connecting.
	// Ignored in cluster mode — Redis Cluster only supports DB 0.
	Database int
	// PoolSize is the per-CPU connection pool size; the effective pool size
	// is PoolSize * runtime.GOMAXPROCS(0).
	PoolSize int

	// MasterName enables Sentinel-backed failover mode using the given
	// sentinel master name; Addresses (or Address) must point at the sentinel
	// nodes, not the Redis nodes directly.
	MasterName string

	// IsClusterMode forces cluster mode even when only one address is given,
	// e.g. managed offerings that expose a single cluster configuration
	// endpoint (such as AWS ElastiCache).
	IsClusterMode bool

	// MaxRetries is the maximum number of retries before giving up on a
	// command. Zero uses the go-redis default (3); -1 disables retries.
	MaxRetries int

	// DialTimeout, ReadTimeout, WriteTimeout: zero uses the go-redis default
	// for each (5s, 3s, 3s respectively).
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}
