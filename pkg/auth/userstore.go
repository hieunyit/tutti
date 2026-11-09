package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/dom"
)

const (
	// ConsoleUsersKey is the Redis hash (config DB) of managed console users: field = username, value = JSON record.
	ConsoleUsersKey = "tutti:console-users"
	// consoleLoginKey holds the last login time per username. It is separate from the records so that a best-effort
	// login update can never overwrite a concurrent change of the record.
	consoleLoginKey = "tutti:console-users:last-login"

	// BcryptCost is the cost used for managed user passwords.
	BcryptCost = 12
)

// ManagedUser is a console user stored in Redis.
type ManagedUser struct {
	Username           string    `json:"username"`
	Role               string    `json:"role"`
	PasswordHash       string    `json:"passwordHash"`
	Disabled           bool      `json:"disabled"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
	LastLoginAt        time.Time `json:"lastLoginAt,omitempty"`
	PasswordChangedAt  time.Time `json:"passwordChangedAt"`
	MustChangePassword bool      `json:"mustChangePassword"`
}

// UserStore persists managed console users.
type UserStore struct {
	rdb redis.Cmdable
}

func NewUserStore(rdb redis.Cmdable) *UserStore { return &UserStore{rdb: rdb} }

// Get returns the user or dom.ErrNotFound.
func (s *UserStore) Get(ctx context.Context, username string) (ManagedUser, error) {
	raw, err := s.rdb.HGet(ctx, ConsoleUsersKey, username).Result()
	if errors.Is(err, redis.Nil) {
		return ManagedUser{}, fmt.Errorf("%w: console user %q", dom.ErrNotFound, username)
	}
	if err != nil {
		return ManagedUser{}, fmt.Errorf("console users: get: %w", err)
	}
	var u ManagedUser
	if err := json.Unmarshal([]byte(raw), &u); err != nil {
		return ManagedUser{}, fmt.Errorf("console users: decode %q: %w", username, err)
	}
	u.Username = username
	return u, nil
}

// List returns all users sorted by name, with the last login time merged in. Malformed records are skipped.
func (s *UserStore) List(ctx context.Context) ([]ManagedUser, error) {
	m, err := s.rdb.HGetAll(ctx, ConsoleUsersKey).Result()
	if err != nil {
		return nil, fmt.Errorf("console users: list: %w", err)
	}
	logins, err := s.rdb.HGetAll(ctx, consoleLoginKey).Result()
	if err != nil {
		return nil, fmt.Errorf("console users: list logins: %w", err)
	}
	res := make([]ManagedUser, 0, len(m))
	for name, raw := range m {
		var u ManagedUser
		if err := json.Unmarshal([]byte(raw), &u); err != nil {
			continue
		}
		u.Username = name
		if v, ok := logins[name]; ok {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				u.LastLoginAt = t
			}
		}
		res = append(res, u)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Username < res[j].Username })
	return res, nil
}

// Create stores a new user; fails with dom.ErrAlreadyExists if the name is taken.
func (s *UserStore) Create(ctx context.Context, u ManagedUser) error {
	raw, err := json.Marshal(u)
	if err != nil {
		return err
	}
	ok, err := s.rdb.HSetNX(ctx, ConsoleUsersKey, u.Username, raw).Result()
	if err != nil {
		return fmt.Errorf("console users: create: %w", err)
	}
	if !ok {
		return fmt.Errorf("%w: console user %q", dom.ErrAlreadyExists, u.Username)
	}
	return nil
}

// Put replaces an existing user record.
func (s *UserStore) Put(ctx context.Context, u ManagedUser) error {
	raw, err := json.Marshal(u)
	if err != nil {
		return err
	}
	if err := s.rdb.HSet(ctx, ConsoleUsersKey, u.Username, raw).Err(); err != nil {
		return fmt.Errorf("console users: put: %w", err)
	}
	return nil
}

// Delete removes the user. Returns dom.ErrNotFound if missing.
func (s *UserStore) Delete(ctx context.Context, username string) error {
	n, err := s.rdb.HDel(ctx, ConsoleUsersKey, username).Result()
	if err != nil {
		return fmt.Errorf("console users: delete: %w", err)
	}
	_ = s.rdb.HDel(ctx, consoleLoginKey, username).Err()
	if n == 0 {
		return fmt.Errorf("%w: console user %q", dom.ErrNotFound, username)
	}
	return nil
}

// TouchLogin records the login time (best effort, never creates records).
func (s *UserStore) TouchLogin(ctx context.Context, username string, at time.Time) error {
	return s.rdb.HSet(ctx, consoleLoginKey, username, at.UTC().Format(time.RFC3339Nano)).Err()
}
