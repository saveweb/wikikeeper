package services

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

const providerRequestInterval = 500 * time.Millisecond

type providerGate struct {
	token                 chan struct{}
	nextAllowed           time.Time
	consecutiveRateLimits int
	now                   func() time.Time
	sleep                 func(context.Context, time.Duration) error
	jitter                func(time.Duration) time.Duration
}

func newProviderGate() *providerGate {
	return newProviderGateWithClock(
		time.Now,
		func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		positiveJitter,
	)
}

func newProviderGateWithClock(
	now func() time.Time,
	sleep func(context.Context, time.Duration) error,
	jitter func(time.Duration) time.Duration,
) *providerGate {
	gate := &providerGate{
		token:  make(chan struct{}, 1),
		now:    now,
		sleep:  sleep,
		jitter: jitter,
	}
	gate.token <- struct{}{}
	return gate
}

func (g *providerGate) run(ctx context.Context, collect func() error) (bool, error) {
	select {
	case <-g.token:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() { g.token <- struct{}{} }()

	if delay := g.nextAllowed.Sub(g.now()); delay > 0 {
		if g.consecutiveRateLimits > 0 {
			seconds := (delay + time.Second - 1) / time.Second
			return false, &HTTPStatusError{
				StatusCode: http.StatusTooManyRequests,
				RetryAfter: strconv.FormatInt(int64(seconds), 10),
				Body:       "provider backoff active",
			}
		}
		if err := g.sleep(ctx, delay); err != nil {
			return false, err
		}
	}
	startedAt := g.now()
	g.nextAllowed = startedAt.Add(providerRequestInterval)

	err := collect()
	if delay, rateLimited := rateLimitBackoff(err, g.consecutiveRateLimits+1, g.now(), g.jitter); rateLimited {
		g.consecutiveRateLimits++
		retryAt := g.now().Add(delay)
		if retryAt.After(g.nextAllowed) {
			g.nextAllowed = retryAt
		}
	} else {
		g.consecutiveRateLimits = 0
	}
	return true, err
}
