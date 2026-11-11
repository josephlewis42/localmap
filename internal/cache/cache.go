package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Cache manages filesystem-based caching for a specific directory.
type Cache struct {
	baseDir string
	ttl     time.Duration
}

// New creates a new cache using the given directory and TTL.
func New(baseDir string, ttl time.Duration) *Cache {
	return &Cache{baseDir: baseDir, ttl: ttl}
}

// Load returns cached bytes for the given key if the cache entry is still valid.
func (c *Cache) Load(key string) ([]byte, bool, error) {
	cachePath := c.cachePath(key)
	info, err := os.Stat(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if time.Since(info.ModTime()) > c.ttl {
		_ = os.Remove(cachePath)
		return nil, false, nil
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// Save writes bytes to the cache for the given key.
func (c *Cache) Save(key string, data []byte) error {
	cachePath := c.cachePath(key)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(cachePath, data, 0o644)
}

// Cleanup removes expired cached entries in the directory.
func (c *Cache) Cleanup() error {
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(c.baseDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) > c.ttl {
			_ = os.Remove(fullPath)
		}
	}
	return nil
}

func (c *Cache) cachePath(key string) string {
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(c.baseDir, fmt.Sprintf("%s.bin", hex.EncodeToString(hash[:])))
}

// MakeKey deterministically converts query parameters and request context into a stable cache key.
func MakeKey(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, params[k]))
	}
	return strings.Join(parts, "&")
}

// Manager manages multiple named caches and handles cleanup.
type Manager struct {
	baseDir       string
	ttl           time.Duration
	caches        map[string]*Cache
	mu            sync.RWMutex
	stopChan      chan struct{}
	cleanupTicker *time.Ticker
}

// NewManager creates a new cache manager with the given base directory and TTL.
func NewManager(baseDir string, ttl time.Duration) *Manager {
	return &Manager{
		baseDir:  baseDir,
		ttl:      ttl,
		caches:   make(map[string]*Cache),
		stopChan: make(chan struct{}),
	}
}

// GetCache returns or creates a cache for the given name.
func (m *Manager) GetCache(name string) *Cache {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, ok := m.caches[name]; ok {
		return c
	}

	cacheDir := filepath.Join(m.baseDir, name)
	c := New(cacheDir, m.ttl)
	m.caches[name] = c
	return c
}

// Start begins the cleanup routine. Should be called in a goroutine.
func (m *Manager) Start(ctx context.Context) {
	m.cleanupTicker = time.NewTicker(10 * time.Minute)
	defer m.cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.cleanupOnce()
			return
		case <-m.stopChan:
			m.cleanupOnce()
			return
		case <-m.cleanupTicker.C:
			m.cleanupOnce()
		}
	}
}

// Stop stops the cleanup routine.
func (m *Manager) Stop() {
	close(m.stopChan)
}

func (m *Manager) cleanupOnce() {
	m.mu.RLock()
	caches := make([]*Cache, 0, len(m.caches))
	for _, c := range m.caches {
		caches = append(caches, c)
	}
	m.mu.RUnlock()

	for _, c := range caches {
		if err := c.Cleanup(); err != nil {
			log.Printf("cache cleanup failed: %v", err)
		}
	}
}
