package gemini

import (
	"testing"
	"time"

	"github.com/benidevo/vega/internal/ai/llm"
	"github.com/benidevo/vega/internal/ai/models"
	"github.com/stretchr/testify/assert"
)

func TestResponseCache_EvictionLogic(t *testing.T) {
	tests := []struct {
		name       string
		operations func(*ResponseCache)
		assertions func(*testing.T, *ResponseCache)
	}{
		{
			name: "new entry triggers eviction when cache is full",
			operations: func(cache *ResponseCache) {
				for i := 0; i < 3; i++ {
					req := llm.GenerateRequest{
						ResponseType: llm.ResponseTypeCVParsing,
						Prompt: models.Prompt{
							Instructions: string(rune('A' + i)),
						},
					}
					resp := llm.GenerateResponse{
						Data: string(rune('A' + i)),
					}
					cache.Set(req, resp)
				}

				firstReq := llm.GenerateRequest{
					ResponseType: llm.ResponseTypeCVParsing,
					Prompt: models.Prompt{
						Instructions: "A",
					},
				}
				cache.Get(firstReq)

				newReq := llm.GenerateRequest{
					ResponseType: llm.ResponseTypeCVParsing,
					Prompt: models.Prompt{
						Instructions: "D",
					},
				}
				cache.Set(newReq, llm.GenerateResponse{Data: "D"})
			},
			assertions: func(t *testing.T, cache *ResponseCache) {
				stats := cache.GetStats()
				assert.Equal(t, 3, stats.Size, "Cache should remain at capacity")

				reqA := llm.GenerateRequest{
					ResponseType: llm.ResponseTypeCVParsing,
					Prompt:       models.Prompt{Instructions: "A"},
				}
				_, found := cache.Get(reqA)
				assert.True(t, found, "Entry A should still be in cache")

				reqB := llm.GenerateRequest{
					ResponseType: llm.ResponseTypeCVParsing,
					Prompt:       models.Prompt{Instructions: "B"},
				}
				_, found = cache.Get(reqB)
				assert.False(t, found, "Entry B should have been evicted")

				reqD := llm.GenerateRequest{
					ResponseType: llm.ResponseTypeCVParsing,
					Prompt:       models.Prompt{Instructions: "D"},
				}
				_, found = cache.Get(reqD)
				assert.True(t, found, "Entry D should be in cache")
			},
		},
		{
			name: "updating existing entry does not trigger eviction",
			operations: func(cache *ResponseCache) {
				for i := 0; i < 3; i++ {
					req := llm.GenerateRequest{
						ResponseType: llm.ResponseTypeCVParsing,
						Prompt: models.Prompt{
							Instructions: string(rune('A' + i)),
						},
					}
					resp := llm.GenerateResponse{
						Data: string(rune('A' + i)),
					}
					cache.Set(req, resp)
				}

				reqB := llm.GenerateRequest{
					ResponseType: llm.ResponseTypeCVParsing,
					Prompt: models.Prompt{
						Instructions: "B",
					},
				}
				cache.Set(reqB, llm.GenerateResponse{Data: "B-updated"})
			},
			assertions: func(t *testing.T, cache *ResponseCache) {
				stats := cache.GetStats()
				assert.Equal(t, 3, stats.Size, "Cache size should remain unchanged")
				assert.Equal(t, int64(0), stats.Evictions, "No evictions should occur")

				for i := 0; i < 3; i++ {
					req := llm.GenerateRequest{
						ResponseType: llm.ResponseTypeCVParsing,
						Prompt: models.Prompt{
							Instructions: string(rune('A' + i)),
						},
					}
					_, found := cache.Get(req)
					assert.True(t, found, "Entry %s should still be in cache", string(rune('A'+i)))
				}

				reqB := llm.GenerateRequest{
					ResponseType: llm.ResponseTypeCVParsing,
					Prompt:       models.Prompt{Instructions: "B"},
				}
				resp, _ := cache.Get(reqB)
				assert.Equal(t, "B-updated", resp.Data, "Entry B should be updated")
			},
		},
		{
			name: "cache not at capacity does not trigger eviction",
			operations: func(cache *ResponseCache) {
				for i := 0; i < 2; i++ {
					req := llm.GenerateRequest{
						ResponseType: llm.ResponseTypeCVParsing,
						Prompt: models.Prompt{
							Instructions: string(rune('A' + i)),
						},
					}
					resp := llm.GenerateResponse{
						Data: string(rune('A' + i)),
					}
					cache.Set(req, resp)
				}

				req := llm.GenerateRequest{
					ResponseType: llm.ResponseTypeCVParsing,
					Prompt: models.Prompt{
						Instructions: "C",
					},
				}
				cache.Set(req, llm.GenerateResponse{Data: "C"})
			},
			assertions: func(t *testing.T, cache *ResponseCache) {
				stats := cache.GetStats()
				assert.Equal(t, 3, stats.Size, "Cache should have 3 entries")
				assert.Equal(t, int64(0), stats.Evictions, "No evictions should occur")

				for i := 0; i < 3; i++ {
					req := llm.GenerateRequest{
						ResponseType: llm.ResponseTypeCVParsing,
						Prompt: models.Prompt{
							Instructions: string(rune('A' + i)),
						},
					}
					_, found := cache.Get(req)
					assert.True(t, found, "Entry %s should be in cache", string(rune('A'+i)))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewResponseCache(3, 5*time.Minute)

			tt.operations(cache)

			tt.assertions(t, cache)
		})
	}
}
