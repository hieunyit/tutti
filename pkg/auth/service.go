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
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/hieunyit/tutti/pkg/dom"
)

const (
	sessionPrefix = "tutti:session:"
	failPrefix    = "tutti:authfail:"

	maxFailures = 10
	// maxUserFailures is the per-username threshold across all client
	// addresses. It is deliberately much higher than maxFailures so that one
	// client cannot lock a user out for everyone.
	maxUserFailures = 100
	failWindow      = 15 * time.Minute
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrTooManyAttempts    = errors.New("too many failed login attempts")
	ErrInvalidSession     = errors.New("invalid or expired session")
)

// Session is the data stored in Redis for a login token.
type Session struct {
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`

	// Resolved on every request from the current users, never stored.
	Source             string `json:"-"`
	MustChangePassword bool   `json:"-"`
}

// LoginResult is returned from a successful login.
type LoginResult struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
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
}

// Option customizes a Service.
type Option func(*Service)

// WithManagedUsers enables console users stored in Redis (normally the config DB). Config users stay read-only
// and win on name conflicts.
func WithManagedUsers(rdb redis.Cmdable) Option {
	return func(s *Service) { s.managed = NewUserStore(rdb) }
}

func NewService(conf Config, rdb redis.Cmdable, opts ...Option) (*Service, error) {
	if err := conf.Validate(); err != nil {
		return nil, err
	}
	trusted, err := parseTrustedProxies(conf.TrustedProxies)
	if err != nil {
		return nil, err
	}
	s := &Service{trusted: trusted, conf: conf, rdb: rdb, users: make(map[string]User, len(conf.Users)), now: time.Now}
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

// lookup finds the user in config first, then in the managed store.
func (s *Service) lookup(ctx context.Context, username string) (account, bool, error) {
	if u, ok := s.users[username]; ok {
		return account{Username: u.Username, Hash: u.PasswordHash, Role: u.Role, Source: SourceConfig}, true, nil
	}
	if s.managed == nil {
		return account{}, false, nil
	}
	m, err := s.managed.Get(ctx, username)
	if errors.Is(err, dom.ErrNotFound) {
		return account{}, false, nil
	}
	if err != nil {
		return account{}, false, err
	}
	return account{
		Username: m.Username, Hash: m.PasswordHash, Role: m.Role, Source: SourceManaged, Disabled: m.Disabled,
		MustChangePassword: m.MustChangePassword, PasswordChangedAt: m.PasswordChangedAt,
	}, true, nil
}

// throttled reports ErrTooManyAttempts if the username/ip pair or the username reached its failure limit.
func (s *Service) throttled(ctx context.Context, username, ip string) error {
	for _, c := range []struct {
		key string
		max int
	}{{failKey(username, ip), maxFailures}, {userFailKey(username), maxUserFailures}} {
		n, err := s.rdb.Get(ctx, c.key).Int()
		if err == nil && n >= c.max {
			log.Warn().Str("username", username).Str("client_ip", ip).Msg("login rejected: too many failed attempts")
			return ErrTooManyAttempts
		} else if err != nil && !errors.Is(err, redis.Nil) {
			return fmt.Errorf("auth: rate limit lookup: %w", err)
		}
	}
	return nil
}

func (s *Service) recordFailure(ctx context.Context, username, ip string) error {
	fk, uk := failKey(username, ip), userFailKey(username)
	pipe := s.rdb.TxPipeline()
	pipe.Incr(ctx, fk)
	pipe.Expire(ctx, fk, failWindow)
	pipe.Incr(ctx, uk)
	pipe.Expire(ctx, uk, failWindow)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("auth: record failure: %w", err)
	}
	return nil
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
		if err := s.recordFailure(ctx, username, ip); err != nil {
			return nil, err
		}
		log.Warn().Str("username", username).Str("client_ip", ip).Bool("known_user", found).Bool("disabled", u.Disabled).Msg("failed login")
		return nil, ErrInvalidCredentials
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, fmt.Errorf("auth: generate token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	now := s.now().UTC()
	ttl := s.conf.ttl()
	data, err := json.Marshal(Session{Username: u.Username, Role: u.Role, CreatedAt: now})
	if err != nil {
		return nil, err
	}
	if err := s.rdb.Set(ctx, sessionPrefix+hashToken(token), data, ttl).Err(); err != nil {
		return nil, fmt.Errorf("auth: store session: %w", err)
	}
	_ = s.rdb.Del(ctx, failKey(username, ip)).Err()
	if u.Source == SourceManaged {
		if err := s.managed.TouchLogin(ctx, u.Username, now); err != nil {
			log.Warn().Err(err).Str("username", u.Username).Msg("unable to record last login")
		}
	}
	return &LoginResult{Token: token, ExpiresAt: now.Add(ttl), Username: u.Username, Role: u.Role}, nil
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
