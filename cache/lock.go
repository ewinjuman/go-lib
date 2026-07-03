package cache

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// releaseScript deletes the lock key only if it still holds the token we set,
// so a holder never releases a lock that expired and was re-acquired by someone else.
const releaseScript = `
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
end
return 0`

// Lock is a distributed lock acquired via SETNX. It is safe across multiple
// pods/instances sharing the same Redis: only one holder can own a given key
// at a time.
type Lock struct {
	client *RedisClient
	key    string
	token  string
}

// TryLock attempts to acquire a distributed lock on key, auto-expiring after
// ttl in case the holder crashes before calling Unlock. ok is false when
// another holder already owns the lock.
func (c *RedisClient) TryLock(ctx context.Context, key string, ttl time.Duration) (lock *Lock, ok bool, err error) {
	token := uuid.NewString()
	ok, err = c.SetNX(ctx, key, token, ttl)
	if err != nil || !ok {
		return nil, false, err
	}
	return &Lock{client: c, key: key, token: token}, true, nil
}

// Unlock releases the lock, but only if it is still owned by this holder.
func (l *Lock) Unlock(ctx context.Context) error {
	return l.client.client.Eval(ctx, releaseScript, []string{l.key}, l.token).Err()
}
