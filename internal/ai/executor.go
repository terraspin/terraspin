package ai

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"
)

// LLMOptions controls one LLM request and its retries.
type LLMOptions struct {
	Provider   string
	APIKey     string
	Model      string
	OllamaHost string
	Timeout    time.Duration
	MaxRetries int
}

// HTTPError records provider response metadata needed for retry decisions.
type HTTPError struct {
	Provider   string
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s: HTTP %d: %s", e.Provider, e.StatusCode, e.Message)
}

var retrySleep = func(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

var retryJitter = func(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	return time.Duration(rand.Int63n(int64(delay/5) + 1))
}

// Execute queries a provider with a timeout for each attempt and bounded retries.
func Execute(ctx context.Context, options LLMOptions, prompt string) (string, int, error) {
	if options.Timeout <= 0 {
		options.Timeout = 30 * time.Second
	}
	if options.MaxRetries < 0 {
		options.MaxRetries = 0
	}
	if options.Provider == "claude" || options.Provider == "openai" {
		if options.APIKey == "" {
			return "", 0, fmt.Errorf("%s: missing required %s; a gateway/proxy without authentication can use a placeholder value", options.Provider, apiKeyEnv(options.Provider))
		}
	}

	var lastErr error
	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, options.Timeout)
		text, tokens, err := QueryLLM(attemptCtx, options.Provider, options.APIKey, options.Model, options.OllamaHost, prompt)
		if err == nil {
			cancel()
			return text, tokens, nil
		}
		lastErr = err
		retry := attempt < options.MaxRetries && isRetryable(err)
		cancel()
		if !retry {
			break
		}
		if err := retrySleep(ctx, retryDelay(err, attempt)); err != nil {
			return "", 0, err
		}
	}
	return "", 0, lastErr
}

func parseRetryAfter(value string) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if when, err := httpDate(value); err == nil {
		return max(time.Until(when), 0)
	}
	return 0
}

var httpDate = func(value string) (time.Time, error) { return time.Parse(time.RFC1123, value) }

func apiKeyEnv(provider string) string {
	if provider == "openai" {
		return "OPENAI_API_KEY"
	}
	return "ANTHROPIC_API_KEY"
}

func isRetryable(err error) bool {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == 429 || httpErr.StatusCode >= 500
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) || strings.Contains(err.Error(), "context deadline exceeded")
}

func retryDelay(err error, attempt int) time.Duration {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) && httpErr.RetryAfter > 0 {
		if httpErr.RetryAfter > 10*time.Second {
			return 10 * time.Second
		}
		return httpErr.RetryAfter
	}
	delay := 250 * time.Millisecond * time.Duration(1<<attempt)
	if delay > 10*time.Second {
		delay = 10 * time.Second
	}
	return delay + retryJitter(delay)
}
