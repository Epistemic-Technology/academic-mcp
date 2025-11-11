package llm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
)

func TestRateLimitedCall_Success(t *testing.T) {
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	// Test successful call
	result, err := RateLimitedCall(ctx, 100, log, func(ctx context.Context) (string, error) {
		return "success", nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result != "success" {
		t.Errorf("Expected 'success', got: %s", result)
	}
}

func TestRateLimitedCall_NonRateLimitError(t *testing.T) {
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	// Test non-rate-limit error (should not retry)
	testErr := errors.New("some other error")
	_, err := RateLimitedCall(ctx, 100, log, func(ctx context.Context) (string, error) {
		return "", testErr
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err != testErr {
		t.Errorf("Expected original error, got: %v", err)
	}
}

func TestRateLimitedCall_RateLimitRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping retry test in short mode")
	}

	ctx := context.Background()
	log := logger.NewNoOpLogger()

	// Test rate limit error with retry
	callCount := 0
	result, err := RateLimitedCall(ctx, 100, log, func(ctx context.Context) (string, error) {
		callCount++
		if callCount < 3 {
			return "", errors.New("429 Too Many Requests")
		}
		return "success after retry", nil
	})

	if err != nil {
		t.Fatalf("Expected no error after retry, got: %v", err)
	}

	if result != "success after retry" {
		t.Errorf("Expected 'success after retry', got: %s", result)
	}

	if callCount != 3 {
		t.Errorf("Expected 3 calls, got: %d", callCount)
	}
}

func TestRateLimitedCall_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	log := logger.NewNoOpLogger()

	// Cancel context immediately
	cancel()

	_, err := RateLimitedCall(ctx, 100, log, func(ctx context.Context) (string, error) {
		t.Error("Function should not be called with cancelled context")
		return "", nil
	})

	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got: %v", err)
	}
}

func TestIsRateLimitError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"429 error", errors.New("429 Too Many Requests"), true},
		{"rate limit text", errors.New("rate limit exceeded"), true},
		{"rate_limit_exceeded", errors.New("rate_limit_exceeded"), true},
		{"other error", errors.New("some other error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRateLimitError(tt.err)
			if result != tt.expected {
				t.Errorf("isRateLimitError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestWorkerPool(t *testing.T) {
	ctx := context.Background()
	wp := NewWorkerPool(2) // Pool with 2 workers

	// Test acquiring workers
	if err := wp.Acquire(ctx); err != nil {
		t.Fatalf("Failed to acquire first worker: %v", err)
	}

	if err := wp.Acquire(ctx); err != nil {
		t.Fatalf("Failed to acquire second worker: %v", err)
	}

	// Test that third acquire blocks (use timeout to verify)
	ctx2, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	err := wp.Acquire(ctx2)
	if err == nil {
		t.Error("Expected timeout error when pool is full, got nil")
	}

	// Release a worker and try again
	wp.Release()
	if err := wp.Acquire(ctx); err != nil {
		t.Fatalf("Failed to acquire worker after release: %v", err)
	}
}

func TestParallelProcess(t *testing.T) {
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	// Test processing multiple items
	items := []int{1, 2, 3, 4, 5}

	results, err := ParallelProcess(ctx, items, log, func(ctx context.Context, idx int, item int) (int, error) {
		return item * 2, nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(results) != len(items) {
		t.Fatalf("Expected %d results, got %d", len(items), len(results))
	}

	for i, result := range results {
		expected := items[i] * 2
		if result != expected {
			t.Errorf("Result[%d] = %d, want %d", i, result, expected)
		}
	}
}

func TestParallelProcess_Error(t *testing.T) {
	ctx := context.Background()
	log := logger.NewNoOpLogger()

	// Test that error is propagated
	items := []int{1, 2, 3}
	testErr := errors.New("processing error")

	_, err := ParallelProcess(ctx, items, log, func(ctx context.Context, idx int, item int) (int, error) {
		if item == 2 {
			return 0, testErr
		}
		return item, nil
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestParallelProcess_ContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cancellation test in short mode")
	}

	ctx, cancel := context.WithCancel(context.Background())
	log := logger.NewNoOpLogger()

	// Cancel context before starting to ensure quick failure
	cancel()

	items := []int{1, 2, 3}

	_, err := ParallelProcess(ctx, items, log, func(ctx context.Context, idx int, item int) (int, error) {
		return item, nil
	})

	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}
}
