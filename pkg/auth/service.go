package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
)

const (
	sessionPrefix = "tutti:session:"
	failPrefix    = "tutti:authfail:"

	maxFailures = 10
	// maxUserFailures is the per-username threshold across all client
	// addresses. Beyond it, (user, IP) pairs that never logged in successfully
	// are slowed down with a progressive delay; pairs that did are unaffected.
	maxUserFailures = 100
	// maxUserHardFailures is the hard per-username ceiling: unknown client
	// addresses are refused beyond it. Known (user, IP) pairs are never refused
	// by the username-wide budget, so an attacker spraying from many addresses
	// cannot lock the real user out.
	maxUserHardFailures = 1000
	failWindow          = 15 * time.Minute

	refreshPrefix = "auth:refresh:"
	refreshLimit  = 6
	refreshWindow = time.Minute

	knownPrefix = "tutti:authknown:"
	// knownTTL is how long a successful (user, IP) login is remembered.
	knownTTL = 30 * 24 * time.Hour

	// Progressive delay applied to unknown IPs over the username-wide budget:
	// minDelay doubled every delayStep failures, capped at maxDelay.
	minDelay  = time.Second
	maxDelay  = 30 * time.Second
	delayStep = 10
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrTooManyAttempts    = errors.New("too many failed login attempts")
	ErrInvalidSession     = errors.New("invalid or expired session")
	// ErrRefreshRateLimited is returned by Refresh when a token refreshes more than refreshLimit times per minute.
	ErrRefreshRateLimited = errors.New("too many session refreshes")
)

// InvalidCredentialsError wraps the generic credential error while carrying
// the retry budget for this username and client IP. It is identical for known
// and unknown users.
type InvalidCredentialsError struct {
	RemainingAttempts int
}

func (e *InvalidCredentialsError) Error() string { return ErrInvalidCredentials.Error() }
func (e *InvalidCredentialsError) Unwrap() error { return ErrInvalidCredentials }

// Session is the data stored in Redis for a login token.
type Session struct {
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	// StartedAt is the original login time and the only anchor of the absolute cap; restamp keeps it
	// while CreatedAt moves. Sessions stored before it existed use CreatedAt.
	StartedAt time.Time `json:"startedAt,omitempty"`
	// LastActiveAt is the last login or refresh. Sessions stored before it existed use CreatedAt.
	LastActiveAt time.Time `json:"lastActiveAt,omitempty"`

	// Resolved on every request from the current users, never stored.
	Source             string `json:"-"`
	MustChangePassword bool   `json:"-"`
}

// LoginResult is returned from a successful login.
type LoginResult struct {
	Token string `json:"token"`
	// ExpiresAt is the idle expiry now; MaxExpiresAt is the absolute cap.
	ExpiresAt    time.Time `json:"expiresAt"`
	MaxExpiresAt time.Time `json:"maxExpiresAt"`
	ServerTime   time.Time `json:"serverTime"`
	Username     string    `json:"username"`
	Role         string    `json:"role"`
}

// SessionTimes are the session deadlines reported to clients (all UTC).
type SessionTimes struct {
	ExpiresAt, MaxExpiresAt, ServerTime time.Time
}

// Times returns the current idle expiry and absolute cap of the session at now.
func (s *Service) Times(sess *Session, now time.Time) SessionTimes {
	last := sess.LastActiveAt
	if last.Before(sess.CreatedAt) {
		last = sess.CreatedAt
	}
	maxAt := sess.capAnchor().Add(s.conf.MaxLifetime())
	exp := last.Add(s.conf.IdleTimeout())
	if exp.After(maxAt) {
		exp = maxAt
	}
	return SessionTimes{ExpiresAt: exp.UTC(), MaxExpiresAt: maxAt.UTC(), ServerTime: now.UTC()}
}

// capAnchor is the time the absolute cap counts from.
func (sess *Session) capAnchor() time.Time {
	if sess.StartedAt.IsZero() {
		return sess.CreatedAt
	}
	return sess.StartedAt
}

// sessionTTL is min(idle, absolute remaining) for a session created at createdAt.
func (s *Service) sessionTTL(createdAt, now time.Time) time.Duration {
	ttl := s.conf.IdleTimeout()
	if rem := s.conf.MaxLifetime() - now.Sub(createdAt); rem < ttl {
		ttl = rem
	}
	return ttl
}

// Service authenticates users and manages sessions.
type Service struct {
	conf  Config
	rdb   redis.Cmdable
	users map[string]User
	// dummyHash is compared against when the user is unknown, to keep
	// response timing independent of user existence.
	dummyHash []byte
	now       func() time.Time
	trusted   []*net.IPNet
	// managed stores console users created at runtime. Nil if not configured.
	managed *UserStore
	// bcryptCost overrides BcryptCost (tests).
	bcryptCost int
	// sleep waits for d or until ctx is done (replaced in tests).
	sleep func(ctx context.Context, d time.Duration) error
	// warnings are static operator warnings reported by GET /auth/me.
	warnings []string
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// userDelay is the progressive delay for an unknown IP when the username-wide failure count is n.
func userDelay(n int) time.Duration {
	if n < maxUserFailures {
		return 0
	}
	step := (n - maxUserFailures) / delayStep
	if step >= 5 { // 1s<<5 = 32s > maxDelay
		return maxDelay
	}
	if d := minDelay << step; d < maxDelay {
		return d
	}
	return maxDelay
}

// Option customizes a Service.
type Option func(*Service)

// WithManagedUsers enables console users stored in Redis (normally the config DB). Config users stay read-only
// and win on name conflicts.
func WithManagedUsers(rdb redis.Cmdable) Option {
	return func(s *Service) { s.managed = NewUserStore(rdb) }
}

// WithControlPlane attaches a PostgreSQL control-plane store to the managed console user store.
func WithControlPlane(cp controlplane.Store) Option {
	return func(s *Service) {
		if s.managed != nil {
			s.managed.WithControlPlane(cp)
		} else {
			s.managed = NewUserStore(nil).WithControlPlane(cp)
		}
	}
}

// WithWarnings sets static warnings reported by GET /auth/me (e.g. WarningWebhookUnauthenticated).
func WithWarnings(w ...string) Option {
	return func(s *Service) { s.warnings = w }
}

func NewService(conf Config, rdb redis.Cmdable, opts ...Option) (*Service, error) {
	if err := conf.Validate(); err != nil {
		return nil, err
	}
	trusted, err := parseTrustedProxies(conf.TrustedProxies)
	if err != nil {
		return nil, err
	}
	s := &Service{trusted: trusted, conf: conf, rdb: rdb, users: make(map[string]User, len(conf.Users)), now: time.Now, sleep: sleepCtx}
	for _, u := range conf.Users {
		s.users[u.Username] = u
	}
	for _, o := range opts {
		o(s)
	}
	dummy, err := bcrypt.GenerateFromPassword([]byte("tutti-dummy-password"), 10)
	if err != nil {
		return nil, err
	}
	s.dummyHash = dummy
	return s, nil
}

// Enabled reports whether authentication is enforced.
func (s *Service) Enabled() bool { return s != nil && s.conf.Enabled }

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func userFailKey(username string) string {
	sum := sha256.Sum256([]byte(username))
	return failPrefix + "user:" + hex.EncodeToString(sum[:])
}

func knownKey(username, ip string) string {
	sum := sha256.Sum256([]byte(username + "\x00" + ip))
	return knownPrefix + hex.EncodeToString(sum[:])
}

func failKey(username, ip string) string {
	sum := sha256.Sum256([]byte(username + "\x00" + ip))
	return failPrefix + hex.EncodeToString(sum[:])
}

// account is a console user resolved from config or the managed store.
type account struct {
	Username           string
	Hash               string
	Role               string
	Source             string
	Disabled           bool
	MustChangePassword bool
	PasswordChangedAt  time.Time
}

const (
	SourceConfig  = "config"
	SourceManaged = "managed"
)

// lookup finds the user in the database (managed store) first if AllowManagedUsers is set, or config first.
func (s *Service) lookup(ctx context.Context, username string) (account, bool, error) {
	if !s.conf.AllowManagedUsers {
		if u, ok := s.users[username]; ok {
			return account{Username: u.Username, Hash: u.PasswordHash, Role: u.Role, Source: SourceConfig}, true, nil
		}
	}
	if s.managed != nil {
		m, err := s.managed.Get(ctx, username)
		if err == nil {
			return account{
				Username: m.Username, Hash: m.PasswordHash, Role: m.Role, Source: SourceManaged, Disabled: m.Disabled,
				MustChangePassword: m.MustChangePassword, PasswordChangedAt: m.PasswordChangedAt,
			}, true, nil
		}
		if !errors.Is(err, dom.ErrNotFound) {
			return account{}, false, err
		}
	}
	if u, ok := s.users[username]; ok {
		return account{Username: u.Username, Hash: u.PasswordHash, Role: u.Role, Source: SourceConfig}, true, nil
	}
	return account{}, false, nil
}

// EnsureAdmin ensures at least one active administrator exists in the database (managed store).
// If the database has no admin, it imports from config or seeds the bootstrap admin user.
// The bootstrapped admin account always requires a password change upon first login unless
// TUTTI_BOOTSTRAP_ADMIN_FORCE_CHANGE=false is explicitly set.
func (s *Service) EnsureAdmin(ctx context.Context) error {
	if s.managed == nil {
		return nil
	}
	list, err := s.managed.List(ctx)
	if err != nil && !errors.Is(err, dom.ErrNotFound) {
		return err
	}
	for _, u := range list {
		if u.Role == RoleAdmin && !u.Disabled {
			return nil
		}
	}
	if len(list) > 0 {
		// Users already exist in the database; do not silently re-seed default admin.
		return nil
	}
	now := s.now().UTC()
	if len(s.conf.Users) > 0 {
		for _, u := range s.conf.Users {
			_ = s.managed.Create(ctx, ManagedUser{
				Username:          u.Username,
				Role:              u.Role,
				PasswordHash:      u.PasswordHash,
				CreatedAt:         now,
				UpdatedAt:         now,
				PasswordChangedAt: now,
			})
		}
		return nil
	}
	bootstrapPw := strings.TrimSpace(os.Getenv("TUTTI_BOOTSTRAP_ADMIN_PASSWORD"))
	if bootstrapPw == "" {
		bootstrapPw = strings.TrimSpace(s.conf.BootstrapAdminPassword)
	}
	mustChange := true
	if val := os.Getenv("TUTTI_BOOTSTRAP_ADMIN_FORCE_CHANGE"); val == "false" || val == "0" {
		mustChange = false
	}
	if bootstrapPw == "" {
		bootstrapPw = randomBootstrapPassword()
		mustChange = true
		log.Warn().Str("username", "admin").Str("password", bootstrapPw).Msg("no bootstrap admin password configured (TUTTI_BOOTSTRAP_ADMIN_PASSWORD / bootstrapAdminPassword): generated a one-time admin password, shown once; it must be changed at first login")
	}
	cost := s.cost()
	hash, err := bcrypt.GenerateFromPassword([]byte(bootstrapPw), cost)
	if err != nil {
		return err
	}
	return s.managed.Create(ctx, ManagedUser{
		Username:           "admin",
		Role:               RoleAdmin,
		PasswordHash:       string(hash),
		CreatedAt:          now,
		UpdatedAt:          now,
		PasswordChangedAt:  now,
		MustChangePassword: mustChange,
	})
}

// throttled enforces the login budgets:
//   - the (user, IP) budget always applies and refuses with ErrTooManyAttempts;
//   - the username-wide budget never blocks a (user, IP) pair that logged in successfully in the last 30 days;
//   - other addresses are delayed progressively once the username-wide count reaches maxUserFailures and refused
//     beyond maxUserHardFailures.
func (s *Service) throttled(ctx context.Context, username, ip string) error {
	n, err := s.rdb.Get(ctx, failKey(username, ip)).Int()
	if err == nil && n >= maxFailures {
		log.Warn().Str("username", username).Str("client_ip", ip).Msg("login rejected: too many failed attempts")
		return ErrTooManyAttempts
	} else if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("auth: rate limit lookup: %w", err)
	}
	un, err := s.rdb.Get(ctx, userFailKey(username)).Int()
	if errors.Is(err, redis.Nil) || (err == nil && un < maxUserFailures) {
		return nil
	} else if err != nil {
		return fmt.Errorf("auth: rate limit lookup: %w", err)
	}
	known, err := s.rdb.Exists(ctx, knownKey(username, ip)).Result()
	if err != nil {
		return fmt.Errorf("auth: rate limit lookup: %w", err)
	}
	if known > 0 {
		return nil
	}
	if un >= maxUserHardFailures {
		log.Warn().Str("username", username).Str("client_ip", ip).Msg("login rejected: username-wide failure ceiling reached")
		return ErrTooManyAttempts
	}
	d := userDelay(un)
	log.Warn().Str("username", username).Str("client_ip", ip).Dur("delay", d).Msg("login delayed: username-wide failure budget exceeded")
	if err := s.sleep(ctx, d); err != nil {
		return err
	}
	return nil
}

func (s *Service) recordFailureCounts(ctx context.Context, username, ip string) (int, int, error) {
	fk, uk := failKey(username, ip), userFailKey(username)
	pipe := s.rdb.TxPipeline()
	pairCount := pipe.Incr(ctx, fk)
	pipe.Expire(ctx, fk, failWindow)
	userCount := pipe.Incr(ctx, uk)
	pipe.Expire(ctx, uk, failWindow)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, 0, fmt.Errorf("auth: record failure: %w", err)
	}
	return int(pairCount.Val()), int(userCount.Val()), nil
}

func (s *Service) recordFailure(ctx context.Context, username, ip string) error {
	_, _, err := s.recordFailureCounts(ctx, username, ip)
	return err
}

// Login verifies credentials and creates a session. ip is used only for
// the failed-attempt counter.
func (s *Service) Login(ctx context.Context, username, password, ip string) (*LoginResult, error) {
	if err := s.throttled(ctx, username, ip); err != nil {
		return nil, err
	}

	u, found, err := s.lookup(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("auth: load user: %w", err)
	}
	hash := []byte(u.Hash)
	if !found {
		hash = s.dummyHash
	}
	cmpErr := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if !found || cmpErr != nil || u.Disabled {
		pairFailures, userFailures, err := s.recordFailureCounts(ctx, username, ip)
		if err != nil {
			return nil, err
		}
		log.Warn().Str("username", username).Str("client_ip", ip).Bool("known_user", found).Bool("disabled", u.Disabled).Msg("failed login")
		remaining := maxFailures - pairFailures
		// The username-wide budget only slows unknown addresses down; it is reported once it is the tighter bound
		// (hard ceiling) for pairs that never logged in successfully.
		if userRemaining := maxUserHardFailures - userFailures; userRemaining < remaining {
			if known, kerr := s.rdb.Exists(ctx, knownKey(username, ip)).Result(); kerr == nil && known == 0 {
				remaining = userRemaining
			}
		}
		if remaining < 0 {
			remaining = 0
		}
		return nil, &InvalidCredentialsError{RemainingAttempts: remaining}
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("auth: generate token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	now := s.now().UTC()
	ttl := s.sessionTTL(now, now)
	data, err := json.Marshal(Session{Username: u.Username, Role: u.Role, CreatedAt: now, StartedAt: now, LastActiveAt: now})
	if err != nil {
		return nil, err
	}
	if err := s.rdb.Set(ctx, sessionPrefix+hashToken(token), data, ttl).Err(); err != nil {
		return nil, fmt.Errorf("auth: store session: %w", err)
	}
	// Clears only this (user, IP) counter: the username-wide counter must keep counting other addresses' failures.
	_ = s.rdb.Del(ctx, failKey(username, ip)).Err()
	if err := s.rdb.Set(ctx, knownKey(username, ip), 1, knownTTL).Err(); err != nil {
		log.Warn().Err(err).Str("username", username).Msg("unable to remember known login address")
	}
	if u.Source == SourceManaged {
		if err := s.managed.TouchLogin(ctx, u.Username, now); err != nil {
			log.Warn().Err(err).Str("username", u.Username).Msg("unable to record last login")
		}
	}
	return &LoginResult{Token: token, ExpiresAt: now.Add(ttl), MaxExpiresAt: now.Add(s.conf.MaxLifetime()), ServerTime: now, Username: u.Username, Role: u.Role}, nil
}

// Refresh extends the session to min(idle timeout, absolute remaining) from now. It is the only way a session is
// extended: Authenticate never does. A session at or past the absolute cap is rejected with ErrInvalidSession; more
// than refreshLimit calls per minute per token fail with ErrRefreshRateLimited.
func (s *Service) Refresh(ctx context.Context, token string) (*SessionTimes, error) {
	sess, err := s.Authenticate(ctx, token)
	if err != nil {
		return nil, err
	}
	hashed := hashToken(token)
	key := refreshPrefix + hashed
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("auth: refresh rate limit: %w", err)
	}
	if n == 1 {
		if err := s.rdb.Expire(ctx, key, refreshWindow).Err(); err != nil {
			return nil, fmt.Errorf("auth: refresh rate limit: %w", err)
		}
	}
	if n > refreshLimit {
		return nil, ErrRefreshRateLimited
	}
	now := s.now().UTC()
	ttl := s.sessionTTL(sess.capAnchor(), now)
	if ttl <= 0 {
		_ = s.rdb.Del(ctx, sessionPrefix+hashed).Err()
		return nil, ErrInvalidSession
	}
	sess.LastActiveAt = now
	data, err := json.Marshal(sess)
	if err != nil {
		return nil, err
	}
	// XX: a session revoked meanwhile (logout) must not be resurrected.
	ok, err := s.rdb.SetArgs(ctx, sessionPrefix+hashed, data, redis.SetArgs{Mode: "XX", TTL: ttl}).Result()
	if errors.Is(err, redis.Nil) || (err == nil && ok != "OK") {
		return nil, ErrInvalidSession
	}
	if err != nil {
		return nil, fmt.Errorf("auth: refresh session: %w", err)
	}
	st := s.Times(sess, now)
	return &st, nil
}

// Authenticate resolves a bearer token to its session. The user and role are resolved from the current config and
// managed users on every call: deleted or disabled users and sessions older than the last password change are rejected.
func (s *Service) Authenticate(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, ErrInvalidSession
	}
	data, err := s.rdb.Get(ctx, sessionPrefix+hashToken(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrInvalidSession
	}
	if err != nil {
		return nil, fmt.Errorf("auth: load session: %w", err)
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, ErrInvalidSession
	}
	u, ok, err := s.lookup(ctx, sess.Username)
	if err != nil {
		return nil, fmt.Errorf("auth: load user: %w", err)
	}
	if !ok || u.Disabled {
		return nil, ErrInvalidSession
	}
	if u.Source == SourceManaged && sess.CreatedAt.Before(u.PasswordChangedAt) {
		return nil, ErrInvalidSession
	}
	// Sessions survive process restarts in Redis, but authorization must use
	// the current users rather than the role recorded at login.
	sess.Role = u.Role
	sess.Source = u.Source
	sess.MustChangePassword = u.MustChangePassword
	return &sess, nil
}

// Logout deletes the session of the token.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.rdb.Del(ctx, sessionPrefix+hashToken(token)).Err()
}

// restamp rewrites the stored session of the token, keeping its remaining lifetime.
func (s *Service) restamp(ctx context.Context, token string, sess *Session) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.rdb.SetArgs(ctx, sessionPrefix+hashToken(token), data, redis.SetArgs{Mode: "XX", KeepTTL: true}).Err()
}

func randomBootstrapPassword() string {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("auth: random bootstrap password: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
