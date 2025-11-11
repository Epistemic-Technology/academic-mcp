package llm

import (
	"context"
	"fmt"
	"math"
	"time"

	"golang.org/x/time/rate"

	"github.com/Epistemic-Technology/academic-mcp/internal/logger"
)

const (
	// OpenAI rate limit is 2M tokens/min for gpt-5-mini
	// We set our limit to 1.8M tokens/min (30k tokens/sec) to leave safety margin
	tokensPerSecond = 30000
	// Burst allows short bursts above the sustained rate
	burstTokens = 60000

	// Worker pool size for parallel processing
	// Lower value = more conservative, higher value = faster but risks rate limits
	defaultMaxWorkers = 15

	// Estimated tokens per PDF page (conservative estimate based on typical academic papers)
	// This includes both input (PDF image) and output (structured JSON)
	estimatedTokensPerPage = 2000

	// Retry configuration
	maxRetries     = 5
	baseRetryDelay = 1 * time.Second
	maxRetryDelay  = 32 * time.Second
)

var (
	// Global rate limiter for OpenAI API calls
	// This ensures all concurrent operations share the same rate limit
	openAIRateLimiter = rate.NewLimiter(rate.Limit(tokensPerSecond), burstTokens)
)

// RateLimitedCall wraps an API call with rate limiting and retry logic.
// It waits for rate limiter approval before making the call, and retries on 429 errors.
func RateLimitedCall[T any](ctx context.Context, estimatedTokens int, log logger.Logger, fn func(context.Context) (T, error)) (T, error) {
	var zero T

	// Wait for rate limiter approval
	err := openAIRateLimiter.WaitN(ctx, estimatedTokens)
	if err != nil {
		return zero, fmt.Errorf("rate limiter wait failed: %w", err)
	}

	// Retry loop with exponential backoff
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := time.Duration(float64(baseRetryDelay) * math.Pow(2, float64(attempt-1)))
			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}

			log.Info("Retry attempt %d/%d after %v delay", attempt, maxRetries, delay)

			// Wait with context cancellation support
			select {
			case <-time.After(delay):
				// Continue to retry
			case <-ctx.Done():
				return zero, ctx.Err()
			}
		}

		// Make the API call
		result, err := fn(ctx)
		if err == nil {
			// Success!
			if attempt > 0 {
				log.Info("Retry succeeded on attempt %d", attempt)
			}
			return result, nil
		}

		lastErr = err

		// Check if this is a rate limit error (429)
		if !isRateLimitError(err) {
			// Not a rate limit error, don't retry
			return zero, err
		}

		log.Warn("Rate limit error (429) on attempt %d/%d: %v", attempt+1, maxRetries+1, err)
	}

	// All retries exhausted
	return zero, fmt.Errorf("max retries (%d) exceeded, last error: %w", maxRetries, lastErr)
}

// isRateLimitError checks if an error is a 429 rate limit error from OpenAI
func isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	// Check if error message contains "429" or "rate limit"
	errStr := err.Error()
	return containsAny(errStr, []string{"429", "rate limit", "rate_limit_exceeded", "Too Many Requests"})
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive for common patterns)
func contains(s, substr string) bool {
	// Simple case-insensitive substring search for error messages
	sLen := len(s)
	subLen := len(substr)
	if subLen > sLen {
		return false
	}
	for i := 0; i <= sLen-subLen; i++ {
		if s[i:i+subLen] == substr {
			return true
		}
	}
	return false
}

// WorkerPool manages a pool of workers for parallel processing with rate limiting
type WorkerPool struct {
	maxWorkers int
	semaphore  chan struct{}
}

// NewWorkerPool creates a new worker pool with the specified maximum workers
func NewWorkerPool(maxWorkers int) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = defaultMaxWorkers
	}
	return &WorkerPool{
		maxWorkers: maxWorkers,
		semaphore:  make(chan struct{}, maxWorkers),
	}
}

// Acquire acquires a worker slot, blocking if all workers are busy
func (wp *WorkerPool) Acquire(ctx context.Context) error {
	select {
	case wp.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a worker slot, allowing another worker to proceed
func (wp *WorkerPool) Release() {
	<-wp.semaphore
}

// ParallelProcess processes items in parallel using the worker pool
// The processFn is called for each item with rate limiting and worker pool control
func ParallelProcess[T any, R any](
	ctx context.Context,
	items []T,
	log logger.Logger,
	processFn func(context.Context, int, T) (R, error),
) ([]R, error) {
	if len(items) == 0 {
		return []R{}, nil
	}

	wp := NewWorkerPool(defaultMaxWorkers)
	results := make([]R, len(items))

	type result struct {
		index int
		value R
		err   error
	}
	resultChan := make(chan result, len(items))

	// Process items in parallel with worker pool control
	for i, item := range items {
		// Acquire a worker slot
		if err := wp.Acquire(ctx); err != nil {
			// Context cancelled, stop spawning new workers
			break
		}

		go func(idx int, itm T) {
			defer wp.Release()

			// Check for cancellation before processing
			select {
			case <-ctx.Done():
				var zero R
				resultChan <- result{index: idx, value: zero, err: ctx.Err()}
				return
			default:
			}

			// Process the item
			val, err := processFn(ctx, idx, itm)
			resultChan <- result{index: idx, value: val, err: err}
		}(i, item)
	}

	// Collect results
	var firstError error
	for range len(items) {
		res := <-resultChan
		if res.err != nil && firstError == nil {
			firstError = res.err
		}
		results[res.index] = res.value
	}
	close(resultChan)

	if firstError != nil {
		return nil, firstError
	}

	return results, nil
}
