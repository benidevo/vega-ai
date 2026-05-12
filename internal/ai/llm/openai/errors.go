package openai

import (
	"context"
	"errors"
	"strings"

	openaisdk "github.com/openai/openai-go"

	commonerrors "github.com/benidevo/vega/internal/common/errors"
)

var (
	// API connection errors
	ErrAPIKeyInvalid      = commonerrors.New("invalid OpenAI API key")
	ErrQuotaExceeded      = commonerrors.New("API quota exceeded")
	ErrServiceUnavailable = commonerrors.New("service temporarily unavailable")
	ErrRateLimitExceeded  = commonerrors.New("rate limit exceeded")
	ErrRequestTimeout     = commonerrors.New("request timeout")

	// Request/response errors
	ErrInvalidRequest      = commonerrors.New("invalid request parameters")
	ErrInvalidResponse     = commonerrors.New("invalid response from provider")
	ErrEmptyResponse       = commonerrors.New("empty response from provider")
	ErrResponseParseFailed = commonerrors.New("failed to parse provider response")

	// Generation errors
	ErrCoverLetterGenFailed = commonerrors.New("cover letter generation failed")
	ErrCVGenFailed          = commonerrors.New("CV generation failed")
	ErrCVParsingFailed      = commonerrors.New("CV parsing failed")
	ErrMatchAnalysisFailed  = commonerrors.New("job match analysis failed")

	// Infrastructure errors
	ErrClientInitFailed   = commonerrors.New("failed to initialize provider client")
	ErrMaxRetriesExceeded = commonerrors.New("maximum retry attempts exceeded")
)

// WrapError wraps innerErr with sentinelErr.
func WrapError(sentinelErr, innerErr error) error {
	return commonerrors.WrapError(sentinelErr, innerErr)
}

// GetSentinelError returns the sentinel error for err.
func GetSentinelError(err error) error {
	return commonerrors.GetSentinelError(err)
}

// ProviderError holds structured API error details.
type ProviderError struct {
	Code    int
	Message string
	Err     error
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return errors.Join(e.Err, errors.New(e.Message)).Error()
	}
	return e.Message
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// IsRetryableError reports whether err should trigger a retry.
// 504/deadline-exceeded reproduce on retry (request is too complex);
// 500 reproduces on the same payload; ctx cancellation means the caller is gone.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var apiErr *openaisdk.Error
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 429, 502, 503:
			return true
		default:
			return false
		}
	}

	msg := strings.ToLower(err.Error())

	if strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "gateway timeout") ||
		strings.Contains(msg, "504") {
		return false
	}

	if strings.Contains(msg, "429") ||
		strings.Contains(msg, "too many requests") ||
		strings.Contains(msg, "rate limit") ||
		strings.Contains(msg, "502") ||
		strings.Contains(msg, "503") ||
		strings.Contains(msg, "service unavailable") {
		return true
	}

	sentinel := GetSentinelError(err)
	return sentinel == ErrServiceUnavailable || sentinel == ErrRateLimitExceeded
}
