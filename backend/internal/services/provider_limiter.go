package services

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
	"gorm.io/gorm"

	"wikikeeper-backend/internal/config"
	applogger "wikikeeper-backend/internal/logger"
	"wikikeeper-backend/internal/models"
	"wikikeeper-backend/internal/repository"
)

var providerLimiterLog = applogger.With("component", "provider-limiter")

type ProviderLimitError struct {
	Provider  string
	Attempted bool
	RetryAt   time.Time
	Err       error
}

func (e *ProviderLimitError) Error() string { return e.Err.Error() }
func (e *ProviderLimitError) Unwrap() error { return e.Err }

func asProviderLimitError(err error) (*ProviderLimitError, bool) {
	var limitErr *ProviderLimitError
	return limitErr, errors.As(err, &limitErr)
}

type providerGate struct {
	token                 chan struct{}
	nextAllowed           time.Time
	consecutiveRateLimits int
	loaded                bool
}

func newProviderGate() *providerGate {
	gate := &providerGate{token: make(chan struct{}, 1)}
	gate.token <- struct{}{}
	return gate
}

type ProviderLimiter struct {
	repo            *repository.ProviderRateLimitRepository
	mu              sync.Mutex
	gates           map[string]*providerGate
	defaultInterval time.Duration
	fandomInterval  time.Duration
	now             func() time.Time
	sleep           func(context.Context, time.Duration) error
	jitter          func(time.Duration) time.Duration
}

func NewProviderLimiter(db *gorm.DB, cfg *config.Config) *ProviderLimiter {
	defaultInterval := 500 * time.Millisecond
	fandomInterval := 5 * time.Second
	if cfg != nil {
		if cfg.ProviderRequestInterval > 0 {
			defaultInterval = time.Duration(cfg.ProviderRequestInterval * float64(time.Second))
		}
		if cfg.FandomRequestInterval > 0 {
			fandomInterval = time.Duration(cfg.FandomRequestInterval * float64(time.Second))
		}
	}

	var repo *repository.ProviderRateLimitRepository
	if db != nil {
		repo = repository.NewProviderRateLimitRepository(db)
	}
	return newProviderLimiter(repo, defaultInterval, fandomInterval, time.Now, sleepContext, positiveJitter)
}

func newProviderLimiter(
	repo *repository.ProviderRateLimitRepository,
	defaultInterval time.Duration,
	fandomInterval time.Duration,
	now func() time.Time,
	sleep func(context.Context, time.Duration) error,
	jitter func(time.Duration) time.Duration,
) *ProviderLimiter {
	return &ProviderLimiter{
		repo:            repo,
		gates:           make(map[string]*providerGate),
		defaultInterval: defaultInterval,
		fandomInterval:  fandomInterval,
		now:             now,
		sleep:           sleep,
		jitter:          jitter,
	}
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func requestGroup(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "default"
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "" {
		return "default"
	}
	domain := host
	if registrable, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
		domain = registrable
	}
	switch domain {
	case "fandom.com", "wikia.com", "wikia.org", "gamepedia.com":
		return "fandom.com"
	default:
		return domain
	}
}

func (l *ProviderLimiter) gateFor(rawURL string) (*providerGate, string) {
	provider := requestGroup(rawURL)
	l.mu.Lock()
	defer l.mu.Unlock()
	gate, ok := l.gates[provider]
	if !ok {
		gate = newProviderGate()
		l.gates[provider] = gate
	}
	return gate, provider
}

func (l *ProviderLimiter) intervalFor(provider string) time.Duration {
	if provider == "fandom.com" {
		return l.fandomInterval
	}
	return l.defaultInterval
}

func (l *ProviderLimiter) load(ctx context.Context, gate *providerGate, provider string) {
	if gate.loaded {
		return
	}
	gate.loaded = true
	if l.repo == nil {
		return
	}
	state, err := l.repo.Get(ctx, provider)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if err != nil {
		providerLimiterLog.Error("failed to load provider cooldown", "provider", provider, "error", err)
		return
	}
	gate.nextAllowed = state.RetryAt
	gate.consecutiveRateLimits = state.ConsecutiveRateLimits
}

func (l *ProviderLimiter) persist(ctx context.Context, provider string, gate *providerGate) {
	if l.repo == nil {
		return
	}
	state := &models.ProviderRateLimit{
		Provider:              provider,
		RetryAt:               gate.nextAllowed,
		ConsecutiveRateLimits: gate.consecutiveRateLimits,
		UpdatedAt:             l.now(),
	}
	if err := l.repo.Upsert(ctx, state); err != nil {
		providerLimiterLog.Error("failed to persist provider cooldown", "provider", provider, "error", err)
	}
}

func (l *ProviderLimiter) clear(ctx context.Context, provider string) {
	if l.repo == nil {
		return
	}
	if err := l.repo.Delete(ctx, provider); err != nil {
		providerLimiterLog.Error("failed to clear provider cooldown", "provider", provider, "error", err)
	}
}

func (l *ProviderLimiter) Run(ctx context.Context, rawURL string, collect func() error) error {
	gate, provider := l.gateFor(rawURL)
	select {
	case <-gate.token:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { gate.token <- struct{}{} }()

	l.load(ctx, gate, provider)
	now := l.now()
	if delay := gate.nextAllowed.Sub(now); delay > 0 {
		if gate.consecutiveRateLimits > 0 {
			seconds := (delay + time.Second - 1) / time.Second
			statusErr := &HTTPStatusError{
				StatusCode: http.StatusTooManyRequests,
				RetryAfter: strconv.FormatInt(int64(seconds), 10),
				Body:       "provider backoff active",
			}
			return &ProviderLimitError{Provider: provider, Attempted: false, RetryAt: gate.nextAllowed, Err: statusErr}
		}
		if err := l.sleep(ctx, delay); err != nil {
			return err
		}
	}

	gate.nextAllowed = l.now().Add(l.intervalFor(provider))
	err := collect()
	if delay, rateLimited := rateLimitBackoff(err, gate.consecutiveRateLimits+1, l.now(), l.jitter); rateLimited {
		gate.consecutiveRateLimits++
		retryAt := l.now().Add(delay)
		if retryAt.After(gate.nextAllowed) {
			gate.nextAllowed = retryAt
		}
		l.persist(ctx, provider, gate)
		return &ProviderLimitError{Provider: provider, Attempted: true, RetryAt: gate.nextAllowed, Err: err}
	}

	if gate.consecutiveRateLimits > 0 {
		gate.consecutiveRateLimits = 0
		l.clear(ctx, provider)
	}
	return err
}

func (l *ProviderLimiter) Cooldown(ctx context.Context, rawURL string) (time.Time, bool) {
	gate, provider := l.gateFor(rawURL)
	select {
	case <-gate.token:
	case <-ctx.Done():
		return time.Time{}, false
	}
	defer func() { gate.token <- struct{}{} }()
	l.load(ctx, gate, provider)
	return gate.nextAllowed, gate.consecutiveRateLimits > 0 && gate.nextAllowed.After(l.now())
}
