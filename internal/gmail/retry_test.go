package gmail

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/api/googleapi"
)

func TestRetrySucceedsFirstAttempt(t *testing.T) {
	t.Parallel()

	calls := 0
	result, err := Retry(context.Background(), DefaultRetryConfig(), func() (string, error) {
		calls++
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want ok", result)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetryRetriesOn429(t *testing.T) {
	t.Parallel()

	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, Cap: 10 * time.Millisecond}
	calls := 0

	result, err := Retry(context.Background(), cfg, func() (string, error) {
		calls++
		if calls < 3 {
			return "", &googleapi.Error{Code: 429}
		}
		return "ok", nil
	})

	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want ok", result)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRetryRetriesOn500(t *testing.T) {
	t.Parallel()

	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, Cap: 10 * time.Millisecond}
	calls := 0

	result, err := Retry(context.Background(), cfg, func() (int, error) {
		calls++
		if calls < 2 {
			return 0, &googleapi.Error{Code: 500}
		}
		return 42, nil
	})

	if err != nil {
		t.Fatalf("Retry() error = %v", err)
	}
	if result != 42 {
		t.Errorf("result = %d, want 42", result)
	}
}

func TestRetryDoesNotRetryOn400(t *testing.T) {
	t.Parallel()

	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, Cap: 10 * time.Millisecond}
	calls := 0

	_, err := Retry(context.Background(), cfg, func() (string, error) {
		calls++
		return "", &googleapi.Error{Code: 400, Message: "bad request"}
	})

	if err == nil {
		t.Fatal("Retry() expected error for 400")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry for 400)", calls)
	}
}

func TestRetryDoesNotRetryOnNonAPIError(t *testing.T) {
	t.Parallel()

	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, Cap: 10 * time.Millisecond}
	calls := 0

	_, err := Retry(context.Background(), cfg, func() (string, error) {
		calls++
		return "", errors.New("not an API error")
	})

	if err == nil {
		t.Fatal("Retry() expected error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestRetryExhaustsAttempts(t *testing.T) {
	t.Parallel()

	cfg := RetryConfig{MaxAttempts: 3, BaseDelay: time.Millisecond, Cap: 10 * time.Millisecond}
	calls := 0

	_, err := Retry(context.Background(), cfg, func() (string, error) {
		calls++
		return "", &googleapi.Error{Code: 503}
	})

	if err == nil {
		t.Fatal("Retry() expected error after exhausting attempts")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestRetryRespectsContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cfg := RetryConfig{MaxAttempts: 5, BaseDelay: 100 * time.Millisecond, Cap: time.Second}

	calls := 0
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := Retry(ctx, cfg, func() (string, error) {
		calls++
		return "", &googleapi.Error{Code: 429}
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}
