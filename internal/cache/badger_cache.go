package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/rs/zerolog/log"
)

var (
	ErrCacheMiss = errors.New("cache miss")
	ErrCacheNil  = errors.New("cache is nil")
)

// BadgerCache implements Cache interface using Badger v4
type BadgerCache struct {
	db     *badger.DB
	ctx    context.Context
	cancel context.CancelFunc
}

func NewBadgerCache(path string, maxMemoryMB int) (*BadgerCache, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil // Disable Badger's internal logger

	if maxMemoryMB > 0 {
		opts.MemTableSize = int64(maxMemoryMB) * 1024 * 1024 / 8
		opts.BaseTableSize = int64(maxMemoryMB) * 1024 * 1024 / 16
		opts.BaseLevelSize = int64(maxMemoryMB) * 1024 * 1024 / 8
		opts.NumLevelZeroTables = 5
		opts.NumLevelZeroTablesStall = 10
	}

	opts.CompactL0OnClose = true
	opts.ValueLogFileSize = 100 << 20 // 100MB
	opts.ValueLogMaxEntries = 10000
	opts.NumCompactors = 2

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger cache: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	bc := &BadgerCache{
		db:     db,
		ctx:    ctx,
		cancel: cancel,
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("Cache GC panic recovered")
			}
		}()
		bc.runGC(ctx)
	}()

	return bc, nil
}

func (c *BadgerCache) runGC(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := c.db.RunValueLogGC(0.5)
			if err != nil && err != badger.ErrNoRewrite {
				log.Warn().Err(err).Msg("Cache GC error")
			}
		}
	}
}

func (c *BadgerCache) Get(ctx context.Context, key string, value interface{}) error {
	if c == nil || c.db == nil {
		return ErrCacheNil
	}

	var data []byte
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		if item.IsDeletedOrExpired() {
			return badger.ErrKeyNotFound
		}

		return item.Value(func(val []byte) error {
			data = append([]byte{}, val...)
			return nil
		})
	})

	if err != nil {
		if err == badger.ErrKeyNotFound {
			return ErrCacheMiss
		}
		return fmt.Errorf("cache get error: %w", err)
	}

	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("cache deserialization error: %w", err)
	}

	return nil
}

func (c *BadgerCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	if c == nil || c.db == nil {
		return ErrCacheNil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache serialization error: %w", err)
	}

	// Enforce maximum TTL of 1 hour to prevent excessive memory usage
	const maxTTL = time.Hour

	effectiveTTL := ttl
	if ttl == 0 || ttl > maxTTL {
		effectiveTTL = maxTTL
	}

	return c.db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), data)
		e.WithTTL(effectiveTTL)
		return txn.SetEntry(e)
	})
}

func (c *BadgerCache) Delete(ctx context.Context, keys ...string) error {
	if c == nil || c.db == nil {
		return ErrCacheNil
	}

	return c.db.Update(func(txn *badger.Txn) error {
		for _, key := range keys {
			if err := txn.Delete([]byte(key)); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
		return nil
	})
}

func (c *BadgerCache) DeletePattern(ctx context.Context, pattern string) error {
	if c == nil || c.db == nil {
		return ErrCacheNil
	}

	// Supports patterns like "user:*" -> prefix "user:"
	prefix := strings.TrimSuffix(pattern, "*")

	return c.db.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10
		it := txn.NewIterator(opts)
		defer it.Close()

		keysToDelete := [][]byte{}

		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			item := it.Item()
			k := item.KeyCopy(nil)
			keysToDelete = append(keysToDelete, k)
		}

		for _, k := range keysToDelete {
			if err := txn.Delete(k); err != nil {
				return err
			}
		}

		return nil
	})
}

func (c *BadgerCache) Exists(ctx context.Context, key string) (bool, error) {
	if c == nil || c.db == nil {
		return false, ErrCacheNil
	}

	exists := false
	err := c.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		exists = !item.IsDeletedOrExpired()
		return nil
	})

	if err == badger.ErrKeyNotFound {
		return false, nil
	}

	return exists, err
}

func (c *BadgerCache) Close() error {
	if c == nil || c.db == nil {
		return nil
	}

	c.cancel()
	return c.db.Close()
}

// NoOpCache is a no-operation cache that always returns cache miss
// Used for graceful degradation when cache is unavailable
type NoOpCache struct{}

func NewNoOpCache() *NoOpCache {
	return &NoOpCache{}
}

func (n *NoOpCache) Get(ctx context.Context, key string, value interface{}) error {
	return ErrCacheMiss
}

func (n *NoOpCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}

func (n *NoOpCache) Delete(ctx context.Context, keys ...string) error {
	return nil
}

func (n *NoOpCache) DeletePattern(ctx context.Context, pattern string) error {
	return nil
}

func (n *NoOpCache) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

func (n *NoOpCache) Close() error {
	return nil
}
