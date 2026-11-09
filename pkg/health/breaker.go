// Package health keeps per-storage circuit breakers fed by active probes and real traffic.
package health

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"sync"
	"syscall"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"
)

const (
	DefaultFailureThreshold = 3
	DefaultOpenDuration     = 30 * time.Second
)

// Config configures the circuit breakers.
type Config struct {
	// FailureThreshold is the number of consecutive availability failures that opens a breaker.
	FailureThreshold int `yaml:"failureThreshold"`
	// OpenDuration is how long an open breaker rejects requests before one trial is allowed.
	OpenDuration time.Duration `yaml:"openDuration"`
}

// WithDefaults replaces zero values with defaults.
func (c Config) WithDefaults() Config {
	if c.FailureThreshold == 0 {
		c.FailureThreshold = DefaultFailureThreshold
	}
	if c.OpenDuration == 0 {
		c.OpenDuration = DefaultOpenDuration
	}
	return c
}

func (c Config) Validate() error {
	if c.FailureThreshold < 0 {
		return fmt.Errorf("health config: failureThreshold must not be negative")
	}
	if c.OpenDuration < 0 {
		return fmt.Errorf("health config: openDuration must not be negative")
	}
	return nil
}

// Outcome is the availability meaning of an error.
type Outcome int

const (
	// Success: the storage answered (including definitive S3 errors such as NoSuchKey or AccessDenied).
	Success Outcome = iota
	// Failure: the storage looks unavailable (network error, timeout, 5xx).
	Failure
	// Neutral: tells nothing about the storage (client cancellation, unrelated local error).
	Neutral
)

// Classify maps an error to its availability meaning.
func Classify(err error) Outcome {
	if err == nil {
		return Success
	}
	var s3Err mclient.ErrorResponse
	if errors.As(err, &s3Err) && s3Err.StatusCode != 0 {
		return classifyStatus(s3Err.StatusCode)
	}
	var withStatus interface{ HTTPStatusCode() int }
	if errors.As(err, &withStatus) && withStatus.HTTPStatusCode() != 0 {
		return classifyStatus(withStatus.HTTPStatusCode())
	}
	var ne net.Error
	switch {
	case errors.Is(err, context.Canceled):
		return Neutral
	case errors.Is(err, context.DeadlineExceeded),
		errors.As(err, &ne),
		errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF),
		errors.Is(err, syscall.ECONNREFUSED), errors.Is(err, syscall.ECONNRESET),
		errors.Is(err, syscall.EPIPE), errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return Failure
	}
	return Neutral
}

func classifyStatus(code int) Outcome {
	if code >= 500 && code != http.StatusNotImplemented {
		return Failure
	}
	return Success
}

// UnavailableError is the S3 error returned instead of waiting for an unavailable storage.
func UnavailableError(storage string) error {
	return mclient.ErrorResponse{
		Code:       "ServiceUnavailable",
		Message:    fmt.Sprintf("storage %s is temporarily unavailable", storage),
		StatusCode: http.StatusServiceUnavailable,
	}
}

// Status is a snapshot of one breaker.
type Status struct {
	LastChange time.Time
	Name       string
	LastError  string
	// Available is true while the breaker is closed.
	Available           bool
	ConsecutiveFailures int
}

type breaker struct {
	lastChange time.Time
	openUntil  time.Time
	trialAt    time.Time
	lastErr    string
	failures   int
	open       bool
	trial      bool
}

// Registry is a concurrency-safe set of per-storage circuit breakers.
type Registry struct {
	now      func() time.Time
	onChange func(storage string, available bool)
	breakers map[string]*breaker
	logger   zerolog.Logger
	cfg      Config
	mu       sync.Mutex
}

type Option func(*Registry)

// WithLogger sets the logger used for breaker transitions.
func WithLogger(l zerolog.Logger) Option { return func(r *Registry) { r.logger = l } }

// WithOnChange sets a callback invoked on every open/close transition (not under lock).
func WithOnChange(f func(storage string, available bool)) Option {
	return func(r *Registry) { r.onChange = f }
}

func withClock(f func() time.Time) Option { return func(r *Registry) { r.now = f } }

func NewRegistry(cfg Config, opts ...Option) *Registry {
	r := &Registry{
		cfg:      cfg.WithDefaults(),
		now:      time.Now,
		breakers: map[string]*breaker{},
		logger:   zlog.Logger,
	}
	for _, o := range opts {
		o(r)
	}
	return r
}

// Register makes storages visible in Snapshot before any traffic.
func (r *Registry) Register(storages ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range storages {
		r.get(s)
	}
}

func (r *Registry) get(storage string) *breaker {
	b, ok := r.breakers[storage]
	if !ok {
		b = &breaker{lastChange: r.now()}
		r.breakers[storage] = b
	}
	return b
}

// trialPossible reports whether an open breaker admits a trial now. Caller holds the lock.
func (r *Registry) trialPossible(b *breaker, now time.Time) bool {
	if now.Before(b.openUntil) {
		return false
	}
	// a trial whose outcome was never observed does not block forever
	return !b.trial || now.Sub(b.trialAt) >= r.cfg.OpenDuration
}

// Available reports whether a request would be admitted, without taking the half-open trial.
func (r *Registry) Available(storage string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.breakers[storage]
	if !ok || !b.open {
		return true
	}
	return r.trialPossible(b, r.now())
}

// Allow reports whether a request may be sent now. For an open breaker whose open period elapsed
// it admits exactly one trial request; the caller must report its outcome with Observe.
func (r *Registry) Allow(storage string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.breakers[storage]
	if !ok || !b.open {
		return true
	}
	now := r.now()
	if !r.trialPossible(b, now) {
		return false
	}
	b.trial, b.trialAt = true, now
	return true
}

// Observe records the outcome of a request or probe to the storage.
func (r *Registry) Observe(storage string, err error) {
	outcome := Classify(err)
	var changed, avail bool
	r.mu.Lock()
	b := r.get(storage)
	now := r.now()
	switch outcome {
	case Success:
		b.failures, b.trial, b.lastErr = 0, false, ""
		if b.open {
			b.open = false
			b.lastChange = now
			changed, avail = true, true
		}
	case Neutral:
		b.trial = false
	case Failure:
		b.failures++
		b.lastErr = err.Error()
		switch {
		case !b.open:
			if b.failures >= r.cfg.FailureThreshold {
				b.open, b.openUntil, b.trial = true, now.Add(r.cfg.OpenDuration), false
				b.lastChange = now
				changed = true
			}
		case !now.Before(b.openUntil):
			// failed half-open trial: stay open for another period
			b.openUntil, b.trial = now.Add(r.cfg.OpenDuration), false
		}
	}
	failures, lastErr := b.failures, b.lastErr
	r.mu.Unlock()

	if !changed {
		return
	}
	if avail {
		r.logger.Info().Str("storage", storage).Msg("storage circuit breaker closed: storage is available again")
	} else {
		r.logger.Warn().Str("storage", storage).Int("consecutive_failures", failures).
			Dur("open_for", r.cfg.OpenDuration).Str("last_error", lastErr).
			Msg("storage circuit breaker opened: requests fail fast until the storage recovers")
	}
	if r.onChange != nil {
		r.onChange(storage, avail)
	}
}

// Snapshot returns the status of every known storage sorted by name.
func (r *Registry) Snapshot() []Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := make([]Status, 0, len(r.breakers))
	for name, b := range r.breakers {
		res = append(res, Status{
			Name: name, Available: !b.open, ConsecutiveFailures: b.failures,
			LastError: b.lastErr, LastChange: b.lastChange,
		})
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
	return res
}
