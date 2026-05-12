package openai

import (
	"context"
	"errors"
	"fmt"
	"testing"

	openaisdk "github.com/openai/openai-go"
)

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},

		{"sdk 429", &openaisdk.Error{StatusCode: 429}, true},
		{"sdk 502", &openaisdk.Error{StatusCode: 502}, true},
		{"sdk 503", &openaisdk.Error{StatusCode: 503}, true},
		{"sdk 504", &openaisdk.Error{StatusCode: 504}, false},
		{"sdk 500", &openaisdk.Error{StatusCode: 500}, false},
		{"sdk 400", &openaisdk.Error{StatusCode: 400}, false},
		{"sdk 401", &openaisdk.Error{StatusCode: 401}, false},

		{"wrapped sdk 429", fmt.Errorf("chat completion error: %w", &openaisdk.Error{StatusCode: 429}), true},

		// Regression: the original log message that the previous implementation
		// failed to recognize. Format produced by the openai-go SDK.
		{
			"raw 429 message from sdk",
			errors.New(`POST "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions": 429 Too Many Requests`),
			true,
		},
		{"too many requests", errors.New("Too Many Requests"), true},
		{"rate limit phrase", errors.New("rate limit exceeded"), true},
		{"service unavailable phrase", errors.New("service unavailable"), true},

		{"504 gateway timeout", errors.New("504 Gateway Timeout"), false},
		{"deadline exceeded", errors.New("context deadline exceeded"), false},

		{"context.Canceled", context.Canceled, false},
		{"context.DeadlineExceeded", context.DeadlineExceeded, false},

		{"sentinel ErrRateLimitExceeded", ErrRateLimitExceeded, true},
		{"sentinel ErrServiceUnavailable", ErrServiceUnavailable, true},
		{"sentinel ErrAPIKeyInvalid", ErrAPIKeyInvalid, false},

		{"unrelated error", errors.New("something went wrong"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableError(tt.err); got != tt.want {
				t.Errorf("IsRetryableError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
