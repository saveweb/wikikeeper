package services

import (
	"errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	baseRateLimitBackoff = 30 * time.Second
	maxRateLimitBackoff  = 6 * time.Hour
)

func asRateLimitError(err error) (*HTTPStatusError, bool) {
	var statusErr *HTTPStatusError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusTooManyRequests {
		return nil, false
	}
	return statusErr, true
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second, true
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		if delay := retryAt.Sub(now); delay > 0 {
			return delay, true
		}
		return 0, true
	}
	return 0, false
}

func rateLimitBackoff(err error, consecutiveFailures int, now time.Time, jitter func(time.Duration) time.Duration) (time.Duration, bool) {
	statusErr, ok := asRateLimitError(err)
	if !ok {
		return 0, false
	}
	if delay, ok := parseRetryAfter(statusErr.RetryAfter, now); ok {
		return delay, true
	}

	if consecutiveFailures < 1 {
		consecutiveFailures = 1
	}
	shift := consecutiveFailures - 1
	if shift > 10 {
		shift = 10
	}
	delay := baseRateLimitBackoff * time.Duration(1<<shift)
	if delay > maxRateLimitBackoff {
		delay = maxRateLimitBackoff
	}
	if jitter != nil {
		delay += jitter(delay)
	}
	return delay, true
}

func positiveJitter(delay time.Duration) time.Duration {
	window := delay / 4
	if window <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(window) + 1))
}
