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
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	sessionPrefix = "tutti:session:"
	failPrefix    = "tutti:authfail:"

	maxFailures = 10
	failWindow  = 15 * time.Minute
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
}

func NewService(conf Config, rdb redis.Cmdable) (*Service, error) {
	if err := conf.Validate(); err != nil {
		return nil, err
	}
	s := &Service{conf: conf, rdb: rdb, users: make(map[string]User, len(conf.Users)), now: time.Now}
	for _, u := range conf.Users {
		s.users[u.Username] = u
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

func failKey(username, ip string) string {
	sum := sha256.Sum256([]byte(username + "\x00" + ip))
	return failPrefix + hex.EncodeToString(sum[:])
}

// Login verifies credentials and creates a session. ip is used only for
// the failed-attempt counter.
func (s *Service) Login(ctx context.Context, username, password, ip string) (*LoginResult, error) {
	fk := failKey(username, ip)
	if n, err := s.rdb.Get(ctx, fk).Int(); err == nil && n >= maxFailures {
		return nil, ErrTooManyAttempts
	} else if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("auth: rate limit lookup: %w", err)
	}

	u, found := s.users[username]
	hash := []byte(u.PasswordHash)
	if !found {
		hash = s.dummyHash
	}
	cmpErr := bcrypt.CompareHashAndPassword(hash, []byte(password))
	if !found || cmpErr != nil {
		pipe := s.rdb.TxPipeline()
		pipe.Incr(ctx, fk)
		pipe.Expire(ctx, fk, failWindow)
		if _, err := pipe.Exec(ctx); err != nil {
			return nil, fmt.Errorf("auth: record failure: %w", err)
		}
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
	_ = s.rdb.Del(ctx, fk).Err()
	return &LoginResult{Token: token, ExpiresAt: now.Add(ttl), Username: u.Username, Role: u.Role}, nil
}

// Authenticate resolves a bearer token to its session.
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
	return &sess, nil
}

// Logout deletes the session of the token.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.rdb.Del(ctx, sessionPrefix+hashToken(token)).Err()
}
