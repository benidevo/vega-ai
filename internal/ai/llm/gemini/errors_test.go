package gemini

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeminiError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *GeminiError
		expected string
	}{
		{
			name: "error with wrapped error",
			err: &GeminiError{
				Code:    500,
				Message: "Internal server error",
				Err:     errors.New("connection timeout"),
			},
			expected: "gemini API error (code 500): Internal server error - connection timeout",
		},
		{
			name: "error without wrapped error",
			err: &GeminiError{
				Code:    400,
				Message: "Bad request",
				Err:     nil,
			},
			expected: "gemini API error (code 400): Bad request",
		},
		{
			name: "error with empty message",
			err: &GeminiError{
				Code:    404,
				Message: "",
				Err:     errors.New("not found"),
			},
			expected: "gemini API error (code 404):  - not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestWrapError(t *testing.T) {
	sentinel := errors.New("sentinel error")
	original := errors.New("original error")

	result := WrapError(sentinel, original)

	assert.NotNil(t, result)
	assert.True(t, errors.Is(result, sentinel))
	assert.Equal(t, "sentinel error: original error", result.Error())
}

func TestGetSentinelError(t *testing.T) {
	// Test that GetSentinelError works with wrapped errors
	wrappedErr := WrapError(ErrServiceUnavailable, errors.New("service down"))
	result := GetSentinelError(wrappedErr)
	assert.Equal(t, ErrServiceUnavailable, result)

	// Test with non-wrapped sentinel error
	result2 := GetSentinelError(ErrRateLimitExceeded)
	assert.Equal(t, ErrRateLimitExceeded, result2)

	// Test with normal error that's not a sentinel
	normalErr := errors.New("normal error")
	result3 := GetSentinelError(normalErr)
	// GetSentinelError from common/errors might return the error itself if not wrapped
	assert.Equal(t, normalErr, result3)
}

func TestNewGeminiError(t *testing.T) {
	err := NewGeminiError(404, "Not found", nil)

	assert.NotNil(t, err)
	assert.Equal(t, 404, err.Code)
	assert.Equal(t, "Not found", err.Message)
	assert.Nil(t, err.Err)

	// Test with wrapped error
	wrapped := errors.New("wrapped error")
	err2 := NewGeminiError(500, "Server error", wrapped)

	assert.NotNil(t, err2)
	assert.Equal(t, 500, err2.Code)
	assert.Equal(t, "Server error", err2.Message)
	assert.Equal(t, wrapped, err2.Err)
}

func TestGeminiError_Unwrap(t *testing.T) {
	wrapped := errors.New("wrapped error")
	err := &GeminiError{
		Code:    500,
		Message: "Server error",
		Err:     wrapped,
	}

	assert.Equal(t, wrapped, err.Unwrap())

	// Test with no wrapped error
	err2 := &GeminiError{
		Code:    404,
		Message: "Not found",
		Err:     nil,
	}

	assert.Nil(t, err2.Unwrap())
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "rate limit exceeded is retryable",
			err:      ErrRateLimitExceeded,
			expected: true,
		},
		{
			name:     "service unavailable is retryable",
			err:      ErrServiceUnavailable,
			expected: true,
		},
		{
			name:     "request timeout is not retryable",
			err:      ErrRequestTimeout,
			expected: false,
		},
		{
			name:     "invalid request is not retryable",
			err:      ErrInvalidRequest,
			expected: false,
		},
		{
			name:     "empty response is not retryable",
			err:      ErrEmptyResponse,
			expected: false,
		},
		{
			name:     "wrapped rate limit error is retryable",
			err:      WrapError(ErrRateLimitExceeded, errors.New("429 from API")),
			expected: true,
		},
		{
			name:     "gemini error 429 is retryable",
			err:      NewGeminiError(429, "Too many requests", nil),
			expected: true,
		},
		{
			name:     "gemini error 500 is retryable",
			err:      NewGeminiError(500, "Internal error", nil),
			expected: true,
		},
		{
			name:     "gemini error 502 is retryable",
			err:      NewGeminiError(502, "Bad gateway", nil),
			expected: true,
		},
		{
			name:     "gemini error 503 is retryable",
			err:      NewGeminiError(503, "Service unavailable", nil),
			expected: true,
		},
		{
			name:     "gemini error 504 is not retryable",
			err:      NewGeminiError(504, "Gateway timeout", nil),
			expected: false,
		},
		{
			name:     "gemini error 400 is not retryable",
			err:      NewGeminiError(400, "Bad request", nil),
			expected: false,
		},
		{
			name:     "generic error is not retryable",
			err:      errors.New("some random error"),
			expected: false,
		},
		{
			name:     "nil error is not retryable",
			err:      nil,
			expected: false,
		},
		{
			name:     "error with 'Error 504' in message is not retryable",
			err:      errors.New("generate content error: Error 504, Message: The request timed out"),
			expected: false,
		},
		{
			name:     "error with 'DEADLINE_EXCEEDED' in message is not retryable",
			err:      errors.New("rpc error: code = DeadlineExceeded desc = DEADLINE_EXCEEDED"),
			expected: false,
		},
		{
			name:     "error with 'deadline exceeded' lowercase is not retryable",
			err:      errors.New("context deadline exceeded"),
			expected: false,
		},
		{
			name:     "error with 'Gateway Timeout' title case is not retryable",
			err:      errors.New("504 Gateway Timeout"),
			expected: false,
		},
		{
			name:     "wrapped error with 504 in message is not retryable",
			err:      WrapError(ErrCVGenFailed, errors.New("Error 504, Message: Gateway timeout")),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryableError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
