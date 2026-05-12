package kv

import (
	"context"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements Store backed by a Redis server.
type RedisStore struct {
	client *redis.Client
}

// NewRedisStore creates a RedisStore. url is a host:port address
// (e.g. "localhost:6379"). The constructor pings the server to verify
// connectivity.
func NewRedisStore(url string) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{Addr: url})
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &RedisStore{client: client}, nil
}

func (r *RedisStore) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", ErrKeyNotFound
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

func (r *RedisStore) Set(ctx context.Context, key, value string) error {
	return r.client.Set(ctx, key, value, 0).Err()
}

func (r *RedisStore) SetEx(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisStore) TTL(ctx context.Context, key string) (int64, error) {
	d, err := r.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	switch {
	case d == -2*time.Second:
		return -2, nil
	case d == -1*time.Second:
		return -1, nil
	default:
		secs := d.Seconds()
		if secs <= 0 {
			return 0, nil
		}
		return int64(math.Ceil(secs)), nil
	}
}

func (r *RedisStore) Del(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisStore) Close() error {
	return r.client.Close()
}
