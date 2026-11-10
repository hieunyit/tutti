package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/controlplane"
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

// UserRepository is the persistence contract for managed console users.
type UserRepository interface {
	Get(ctx context.Context, username string) (ManagedUser, error)
	List(ctx context.Context) ([]ManagedUser, error)
	Create(ctx context.Context, u ManagedUser) error
	Put(ctx context.Context, u ManagedUser) error
	Delete(ctx context.Context, username string) error
	TouchLogin(ctx context.Context, username string, at time.Time) error
}

// UserStore persists managed console users in Redis or PostgreSQL (with Redis fallback on PG read failure).
type UserStore struct {
	rdb redis.Cmdable
	cp  controlplane.Store
}

var _ UserRepository = (*UserStore)(nil)

func NewUserStore(rdb redis.Cmdable) *UserStore { return &UserStore{rdb: rdb} }

// WithControlPlane attaches a PostgreSQL control-plane store.
func (s *UserStore) WithControlPlane(cp controlplane.Store) *UserStore {
	s.cp = cp
	return s
}

func toCPUser(u ManagedUser) controlplane.ConsoleUser {
	return controlplane.ConsoleUser{
		Username:           u.Username,
		Role:               u.Role,
		PasswordHash:       u.PasswordHash,
		Disabled:           u.Disabled,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
		LastLoginAt:        u.LastLoginAt,
		PasswordChangedAt:  u.PasswordChangedAt,
		MustChangePassword: u.MustChangePassword,
	}
}

func fromCPUser(u controlplane.ConsoleUser) ManagedUser {
	return ManagedUser{
		Username:           u.Username,
		Role:               u.Role,
		PasswordHash:       u.PasswordHash,
		Disabled:           u.Disabled,
		CreatedAt:          u.CreatedAt,
		UpdatedAt:          u.UpdatedAt,
		LastLoginAt:        u.LastLoginAt,
		PasswordChangedAt:  u.PasswordChangedAt,
		MustChangePassword: u.MustChangePassword,
	}
}

// Get returns the user or dom.ErrNotFound.
func (s *UserStore) Get(ctx context.Context, username string) (ManagedUser, error) {
	if s.cp != nil {
		v, err := s.cp.Repos().ConsoleUsers.Get(ctx, username)
		if err == nil {
			return fromCPUser(v.Value), nil
		}
		if !controlplane.IsUnavailable(err) || s.rdb == nil {
			return ManagedUser{}, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Str("username", username).Msg("console users: PostgreSQL get failed, falling back to Redis")
	}
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
	if s.cp != nil {
		vs, err := s.cp.Repos().ConsoleUsers.List(ctx)
		if err == nil {
			res := make([]ManagedUser, len(vs))
			for i, v := range vs {
				res[i] = fromCPUser(v.Value)
			}
			return res, nil
		}
		if !controlplane.IsUnavailable(err) || s.rdb == nil {
			return nil, err
		}
		zerolog.Ctx(ctx).Warn().Err(err).Msg("console users: PostgreSQL list failed, falling back to Redis")
	}
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
	if s.cp != nil {
		_, err := s.cp.Repos().ConsoleUsers.Create(ctx, toCPUser(u))
		return controlplane.WrapWriteError(err)
	}
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
	if s.cp != nil {
		_, err := s.cp.Repos().ConsoleUsers.Put(ctx, toCPUser(u))
		return controlplane.WrapWriteError(err)
	}
	raw, err := json.Marshal(u)
	if err != nil {
		return err
	}
	if err := s.rdb.HSet(ctx, ConsoleUsersKey, u.Username, raw).Err(); err != nil {
		return fmt.Errorf("console users: put: %w", err)
	}
	return nil
}

// consoleMutation atomically checks the old record, optionally checks that another
// enabled admin remains, and replaces or deletes the target. A hash-wide check
// prevents two concurrent demotions from both removing the last administrator.
var consoleMutation = redis.NewScript(`
if redis.call('hget', KEYS[1], ARGV[1]) ~= ARGV[2] then return 0 end
if ARGV[4] == '1' then
  local admins = tonumber(ARGV[5])
  local shadowed = {}
  for i = 6, #ARGV do shadowed[ARGV[i]] = true end
  local users = redis.call('hgetall', KEYS[1])
  for i = 1, #users, 2 do
    if users[i] ~= ARGV[1] and not shadowed[users[i]] then
      local ok, u = pcall(cjson.decode, users[i+1])
      if ok and u.role == 'admin' and not u.disabled then admins = admins + 1 end
    end
  end
  if admins == 0 then return -1 end
end
if ARGV[3] == '' then
  redis.call('hdel', KEYS[1], ARGV[1])
  redis.call('hdel', KEYS[2], ARGV[1])
else
  redis.call('hset', KEYS[1], ARGV[1], ARGV[3])
end
return 1
`)

// mutate retries an update against the latest record. fn is re-evaluated after a
// concurrent change, so unrelated updates to the same user are preserved.
func (s *UserStore) mutate(ctx context.Context, username string, configAdmins int, configNames []string,
	fn func(*ManagedUser) (remove, checkOtherAdmin bool, err error)) error {
	if s.cp != nil {
		err := s.cp.Repos().ConsoleUsers.MutateGuarded(ctx, username, controlplane.ConsoleUserGuard{
			ConfigAdmins: configAdmins,
			ConfigNames:  configNames,
		}, func(cu *controlplane.ConsoleUser) (bool, bool, error) {
			u := fromCPUser(*cu)
			remove, checkAdmin, err := fn(&u)
			if err != nil {
				return false, false, err
			}
			*cu = toCPUser(u)
			return remove, checkAdmin, nil
		})
		return controlplane.WrapWriteError(err)
	}
	for range 20 {
		old, err := s.rdb.HGet(ctx, ConsoleUsersKey, username).Result()
		if errors.Is(err, redis.Nil) {
			return fmt.Errorf("%w: console user %q", dom.ErrNotFound, username)
		}
		if err != nil {
			return fmt.Errorf("console users: get: %w", err)
		}
		var u ManagedUser
		if err := json.Unmarshal([]byte(old), &u); err != nil {
			return fmt.Errorf("console users: decode %q: %w", username, err)
		}
		u.Username = username
		remove, checkAdmin, err := fn(&u)
		if err != nil {
			return err
		}
		var next string
		if !remove {
			raw, err := json.Marshal(u)
			if err != nil {
				return err
			}
			next = string(raw)
		}
		guard := 0
		if checkAdmin {
			guard = 1
		}
		args := []any{username, old, next, guard, configAdmins}
		for _, name := range configNames {
			args = append(args, name)
		}
		result, err := consoleMutation.Run(ctx, s.rdb, []string{ConsoleUsersKey, consoleLoginKey}, args...).Int()
		if err != nil {
			return fmt.Errorf("console users: update: %w", err)
		}
		switch result {
		case 1:
			return nil
		case -1:
			return fmt.Errorf("%w: cannot remove the last enabled admin", dom.ErrFailedPrecondition)
		}
	}
	return fmt.Errorf("%w: console user %q: too many concurrent changes", dom.ErrFailedPrecondition, username)
}

// Delete removes the user. Returns dom.ErrNotFound if missing.
func (s *UserStore) Delete(ctx context.Context, username string) error {
	if s.cp != nil {
		return controlplane.WrapWriteError(s.cp.Repos().ConsoleUsers.Delete(ctx, username))
	}
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
	if s.cp != nil {
		return s.cp.Repos().ConsoleUsers.TouchLogin(ctx, username, at)
	}
	return s.rdb.HSet(ctx, consoleLoginKey, username, at.UTC().Format(time.RFC3339Nano)).Err()
}
