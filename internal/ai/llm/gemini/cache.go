package gemini

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/benidevo/vega/internal/ai/llm"
	"github.com/benidevo/vega/internal/ai/models"
)

// CacheEntry represents a cached response with expiration time and hit tracking.
type CacheEntry struct {
	Response  llm.GenerateResponse
	ExpiresAt time.Time
	HitCount  int
}

// ResponseCache implements an LRU cache for LLM responses with TTL expiration.
type ResponseCache struct {
	mu         sync.RWMutex
	entries    map[string]*CacheEntry
	maxEntries int
	ttl        time.Duration

	accessOrder []string
	accessMap   map[string]int
	hits        int64
	misses      int64
	evictions   int64
}

// NewResponseCache creates a new response cache with the specified capacity and TTL.
func NewResponseCache(maxEntries int, ttl time.Duration) *ResponseCache {
	return &ResponseCache{
		entries:     make(map[string]*CacheEntry),
		maxEntries:  maxEntries,
		ttl:         ttl,
		accessOrder: make([]string, 0, maxEntries),
		accessMap:   make(map[string]int),
	}
}

func (rc *ResponseCache) generateCacheKey(request llm.GenerateRequest) string {
	keyData := struct {
		ResponseType string
		Prompt       interface{}
	}{
		ResponseType: string(request.ResponseType),
		Prompt:       request.Prompt,
	}

	jsonBytes, err := json.Marshal(keyData)
	if err != nil {
		return fmt.Sprintf("%s_%d_%d", request.ResponseType, time.Now().UnixNano(), rand.Int63())
	}
	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:])
}

// Get retrieves a cached response if it exists and hasn't expired.
func (rc *ResponseCache) Get(request llm.GenerateRequest) (llm.GenerateResponse, bool) {
	key := rc.generateCacheKey(request)

	rc.mu.Lock()
	defer rc.mu.Unlock()

	entry, exists := rc.entries[key]
	if !exists {
		rc.misses++
		return llm.GenerateResponse{}, false
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(rc.entries, key)
		rc.removeFromAccessOrder(key)
		rc.misses++
		return llm.GenerateResponse{}, false
	}

	entry.HitCount++
	rc.hits++
	rc.updateAccessOrder(key)

	return entry.Response, true
}

// Set stores a response in the cache with the configured TTL.
func (rc *ResponseCache) Set(request llm.GenerateRequest, response llm.GenerateResponse) {
	key := rc.generateCacheKey(request)

	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Check if this is a new entry that would exceed capacity
	_, existingEntry := rc.entries[key]
	if !existingEntry && len(rc.entries) >= rc.maxEntries {
		rc.evictLRU()
	}

	rc.entries[key] = &CacheEntry{
		Response:  response,
		ExpiresAt: time.Now().Add(rc.ttl),
		HitCount:  0,
	}

	rc.updateAccessOrder(key)
}

func (rc *ResponseCache) evictLRU() {
	if len(rc.accessOrder) == 0 {
		return
	}

	lruKey := rc.accessOrder[0]

	delete(rc.entries, lruKey)
	rc.removeFromAccessOrder(lruKey)
	rc.evictions++
}

func (rc *ResponseCache) updateAccessOrder(key string) {
	if pos, exists := rc.accessMap[key]; exists {
		if pos < len(rc.accessOrder) {
			if pos == len(rc.accessOrder)-1 {
				rc.accessOrder = rc.accessOrder[:pos]
			} else {
				rc.accessOrder = append(rc.accessOrder[:pos], rc.accessOrder[pos+1:]...)
			}
		}
		delete(rc.accessMap, key)
		for i := pos; i < len(rc.accessOrder); i++ {
			rc.accessMap[rc.accessOrder[i]] = i
		}
	}

	rc.accessOrder = append(rc.accessOrder, key)
	rc.accessMap[key] = len(rc.accessOrder) - 1
}

func (rc *ResponseCache) removeFromAccessOrder(key string) {
	if pos, exists := rc.accessMap[key]; exists {
		if pos < len(rc.accessOrder) {
			if pos == len(rc.accessOrder)-1 {
				rc.accessOrder = rc.accessOrder[:pos]
			} else {
				rc.accessOrder = append(rc.accessOrder[:pos], rc.accessOrder[pos+1:]...)
			}
		}
		delete(rc.accessMap, key)

		for i := pos; i < len(rc.accessOrder); i++ {
			rc.accessMap[rc.accessOrder[i]] = i
		}
	}
}

// GetStats returns current cache performance metrics.
func (rc *ResponseCache) GetStats() CacheStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	hitRate := float64(0)
	if total := rc.hits + rc.misses; total > 0 {
		hitRate = float64(rc.hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:      rc.hits,
		Misses:    rc.misses,
		Evictions: rc.evictions,
		Size:      len(rc.entries),
		HitRate:   hitRate,
	}
}

// CacheStats contains metrics about cache performance.
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int
	HitRate   float64
}

// Clear removes all entries from the cache and resets statistics.
func (rc *ResponseCache) Clear() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	rc.entries = make(map[string]*CacheEntry)
	rc.accessOrder = rc.accessOrder[:0]
	rc.accessMap = make(map[string]int)
	rc.hits = 0
	rc.misses = 0
	rc.evictions = 0
}

// ShouldCache determines if a response type should be cached.
func ShouldCache(responseType llm.ResponseType) bool {
	switch responseType {
	case llm.ResponseTypeCVParsing, llm.ResponseTypeMatchResult:
		return true
	case llm.ResponseTypeCoverLetter, llm.ResponseTypeCV:
		return false
	default:
		return false
	}
}

// CacheKeyPrefix returns a cache key prefix for the given task type.
func CacheKeyPrefix(taskType models.AITaskType) string {
	switch taskType {
	case models.TaskTypeCVParsing:
		return "cv_parse:"
	case models.TaskTypeJobAnalysis:
		return "job_match:"
	case models.TaskTypeCoverLetter:
		return "cover_letter:"
	case models.TaskTypeCVGeneration:
		return "cv_gen:"
	default:
		return "unknown:"
	}
}
