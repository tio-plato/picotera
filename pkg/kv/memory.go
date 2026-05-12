package kv

import (
	"context"
	"math"
	"time"

	"github.com/jellydator/ttlcache/v3"
)

// MemoryStore implements Store backed by an in-process ttlcache.
type MemoryStore struct {
	cache *ttlcache.Cache[string, string]
}

// NewMemoryStore creates a MemoryStore and starts the background expiration
// goroutine. Call Close to stop it.
func NewMemoryStore() *MemoryStore {
	cache := ttlcache.New[string, string]()
	go cache.Start()
	return &MemoryStore{cache: cache}
}

func (m *MemoryStore) Get(_ context.Context, key string) (string, error) {
	item := m.cache.Get(key)
	if item == nil {
		return "", ErrKeyNotFound
	}
	return item.Value(), nil
}

func (m *MemoryStore) Set(_ context.Context, key, value string) error {
	m.cache.Set(key, value, ttlcache.NoTTL)
	return nil
}

func (m *MemoryStore) SetEx(_ context.Context, key, value string, ttl time.Duration) error {
	m.cache.Set(key, value, ttl)
	return nil
}

func (m *MemoryStore) TTL(_ context.Context, key string) (int64, error) {
	item := m.cache.Get(key)
	if item == nil {
		return -2, nil
	}
	if item.TTL() == ttlcache.NoTTL {
		return -1, nil
	}
	remaining := time.Until(item.ExpiresAt())
	if remaining <= 0 {
		return 0, nil
	}
	return int64(math.Ceil(remaining.Seconds())), nil
}

func (m *MemoryStore) Del(_ context.Context, key string) error {
	m.cache.Delete(key)
	return nil
}

func (m *MemoryStore) Close() error {
	m.cache.Stop()
	return nil
}
