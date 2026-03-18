package gmail

import (
	"context"
	"math"
	"math/rand/v2"
	"strconv"
	"time"

	"google.golang.org/api/googleapi"
)

// RetryConfig controls retry behavior for Gmail API calls.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	Cap         time.Duration
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   500 * time.Millisecond,
		Cap:         30 * time.Second,
	}
}

// Retry executes fn with exponential backoff and full jitter on retryable errors.
// It retries on HTTP 429 (rate limit) and 5xx (server errors).
// It respects the Retry-After header when present.
func Retry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	var zero T

	for attempt := range cfg.MaxAttempts {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		if !isRetryable(err) {
			return zero, err
		}

		if attempt == cfg.MaxAttempts-1 {
			return zero, err
		}

		delay := retryDelay(cfg, attempt, err)

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}

	return zero, nil // unreachable
}

func isRetryable(err error) bool {
	apiErr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}

	if apiErr.Code == 429 {
		return true
	}

	return apiErr.Code >= 500 && apiErr.Code < 600
}

func retryDelay(cfg RetryConfig, attempt int, err error) time.Duration {
	// Check for Retry-After header
	if apiErr, ok := err.(*googleapi.Error); ok {
		for _, h := range apiErr.Header["Retry-After"] {
			if secs, parseErr := strconv.Atoi(h); parseErr == nil {
				return time.Duration(secs) * time.Second
			}
		}
	}

	// Exponential backoff with full jitter
	// sleep = rand(0, min(cap, base * 2^attempt))
	exp := math.Pow(2, float64(attempt))
	maxDelay := time.Duration(float64(cfg.BaseDelay) * exp)
	if maxDelay > cfg.Cap {
		maxDelay = cfg.Cap
	}

	return time.Duration(rand.Int64N(int64(maxDelay)))
}
