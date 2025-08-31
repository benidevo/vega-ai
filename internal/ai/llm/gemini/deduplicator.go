package gemini

import (
	"context"
	"sync"

	"github.com/benidevo/vega/internal/ai/llm"
)

// RequestDeduplicator prevents duplicate concurrent requests for the same key.
type RequestDeduplicator struct {
	mu       sync.Mutex
	inFlight map[string]*inflightRequest

	deduplicatedCount int64
	totalRequests     int64
}

type inflightRequest struct {
	done     chan struct{}
	response llm.GenerateResponse
	err      error
	waiters  int
}

// NewRequestDeduplicator creates a new request deduplicator.
func NewRequestDeduplicator() *RequestDeduplicator {
	return &RequestDeduplicator{
		inFlight: make(map[string]*inflightRequest),
	}
}

// Do executes the function, deduplicating concurrent identical requests.
func (rd *RequestDeduplicator) Do(
	ctx context.Context,
	key string,
	fn func() (llm.GenerateResponse, error),
) (llm.GenerateResponse, error) {
	rd.mu.Lock()
	rd.totalRequests++

	if req, exists := rd.inFlight[key]; exists {
		req.waiters++
		rd.deduplicatedCount++
		rd.mu.Unlock()

		select {
		case <-ctx.Done():
			return llm.GenerateResponse{}, ctx.Err()
		case <-req.done:
			return req.response, req.err
		}
	}

	req := &inflightRequest{
		done:    make(chan struct{}),
		waiters: 0,
	}
	rd.inFlight[key] = req
	rd.mu.Unlock()

	response, err := fn()

	req.response = response
	req.err = err

	rd.mu.Lock()
	if rd.inFlight[key] == req {
		delete(rd.inFlight, key)
	}
	rd.mu.Unlock()

	close(req.done)

	return response, err
}

// GetStats returns deduplication performance metrics.
func (rd *RequestDeduplicator) GetStats() DeduplicatorStats {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	deduplicationRate := float64(0)
	if rd.totalRequests > 0 {
		deduplicationRate = float64(rd.deduplicatedCount) / float64(rd.totalRequests) * 100
	}

	return DeduplicatorStats{
		TotalRequests:     rd.totalRequests,
		DeduplicatedCount: rd.deduplicatedCount,
		InFlightCount:     len(rd.inFlight),
		DeduplicationRate: deduplicationRate,
	}
}

// DeduplicatorStats contains metrics about request deduplication.
type DeduplicatorStats struct {
	TotalRequests     int64
	DeduplicatedCount int64
	InFlightCount     int
	DeduplicationRate float64
}

// Clear resets all deduplication state and statistics.
func (rd *RequestDeduplicator) Clear() {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	rd.inFlight = make(map[string]*inflightRequest)
	rd.deduplicatedCount = 0
	rd.totalRequests = 0
}
