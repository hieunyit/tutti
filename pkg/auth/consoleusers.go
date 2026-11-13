package auth

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/hieunyit/tutti/pkg/dom"
)

const (
	// MinPasswordLength is the password policy minimum.
	MinPasswordLength = 12
	// maxPasswordLength is the bcrypt limit: longer passwords would be silently truncated.
	maxPasswordLength = 72

	generatedPasswordLength = 20
	passwordAlphabet        = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789-_.!"
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@-]{0,63}$`)

// UserInfo describes a console user. It never contains the password hash.
type UserInfo struct {
	Username           string
	Role               string
	Source             string
	Disabled           bool
	MustChangePassword bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
	LastLoginAt        time.Time
	PasswordChangedAt  time.Time
}

func infoFromManaged(m ManagedUser) UserInfo {
	return UserInfo{
		Username: m.Username, Role: m.Role, Source: SourceManaged, Disabled: m.Disabled,
		MustChangePassword: m.MustChangePassword, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		LastLoginAt: m.LastLoginAt, PasswordChangedAt: m.PasswordChangedAt,
	}
}

// ValidatePassword enforces the password policy.
func ValidatePassword(username, password string) error {
	switch {
	case len(password) < MinPasswordLength:
		return fmt.Errorf("%w: password must be at least %d characters", dom.ErrInvalidArg, MinPasswordLength)
	case len(password) > maxPasswordLength:
		return fmt.Errorf("%w: password must be at most %d bytes", dom.ErrInvalidArg, maxPasswordLength)
	case password == username:
		return fmt.Errorf("%w: password must not equal the username", dom.ErrInvalidArg)
	}
	return nil
}

// GeneratePassword returns a random password that satisfies the policy.
func GeneratePassword() (string, error) {
	out := make([]byte, generatedPasswordLength)
	max := big.NewInt(int64(len(passwordAlphabet)))
	for i := range out {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		out[i] = passwordAlphabet[n.Int64()]
	}
	return string(out), nil
}

func validRole(role string) bool { return role == RoleAdmin || role == RoleViewer }

// ready fails unless console user management is possible.
func (s *Service) ready() error {
	if !s.Enabled() {
		return fmt.Errorf("%w: sign-in is disabled (api.auth.enabled=false), console users cannot be managed", dom.ErrFailedPrecondition)
	}
	if s.managed == nil {
		return fmt.Errorf("%w: managed console users are not available on this server", dom.ErrFailedPrecondition)
	}
	return nil
}

func (s *Service) hashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), s.cost())
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// ListUsers returns database managed and config users sorted by name.
func (s *Service) ListUsers(ctx context.Context) ([]UserInfo, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	if s.conf.AllowManagedUsers {
		managed, err := s.managed.List(ctx)
		if err != nil {
			return nil, err
		}
		res := make([]UserInfo, len(managed))
		for i, m := range managed {
			res[i] = infoFromManaged(m)
		}
		sort.Slice(res, func(i, j int) bool { return res[i].Username < res[j].Username })
		return res, nil
	}
	res := make([]UserInfo, 0, len(s.users))
	for _, u := range s.users {
		res = append(res, UserInfo{Username: u.Username, Role: u.Role, Source: SourceConfig})
	}
	managed, err := s.managed.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, m := range managed {
		if _, dup := s.users[m.Username]; dup {
			continue // config wins
		}
		res = append(res, infoFromManaged(m))
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Username < res[j].Username })
	return res, nil
}

// CreateUser creates a managed user. If generate is set, the password is generated and returned.
func (s *Service) CreateUser(ctx context.Context, username, role, password string, generate bool) (UserInfo, string, error) {
	if err := s.ready(); err != nil {
		return UserInfo{}, "", err
	}
	if !usernameRe.MatchString(username) {
		return UserInfo{}, "", fmt.Errorf("%w: username must be 1-64 characters: letters, digits, '.', '_', '@', '-', starting with a letter or digit", dom.ErrInvalidArg)
	}
	if !validRole(role) {
		return UserInfo{}, "", fmt.Errorf("%w: role must be %q or %q", dom.ErrInvalidArg, RoleAdmin, RoleViewer)
	}
	if s.managed != nil {
		if _, err := s.managed.Get(ctx, username); err == nil {
			return UserInfo{}, "", fmt.Errorf("%w: console user %q already exists", dom.ErrAlreadyExists, username)
		}
	}
	if !s.conf.AllowManagedUsers {
		if _, ok := s.users[username]; ok {
			return UserInfo{}, "", fmt.Errorf("%w: console user %q is defined in config", dom.ErrAlreadyExists, username)
		}
	}
	password, generated, err := s.passwordOrGenerate(username, password, generate)
	if err != nil {
		return UserInfo{}, "", err
	}
	hash, err := s.hashPassword(password)
	if err != nil {
		return UserInfo{}, "", err
	}
	now := s.now().UTC()
	m := ManagedUser{
		Username: username, Role: role, PasswordHash: hash, CreatedAt: now, UpdatedAt: now, PasswordChangedAt: now,
		// a generated password is known to the admin: the user must replace it
		MustChangePassword: generated != "",
	}
	if err := s.managed.Create(ctx, m); err != nil {
		return UserInfo{}, "", err
	}
	return infoFromManaged(m), generated, nil
}

func (s *Service) passwordOrGenerate(username, password string, generate bool) (pw, generated string, err error) {
	switch {
	case generate && password != "":
		return "", "", fmt.Errorf("%w: set either password or generatePassword, not both", dom.ErrInvalidArg)
	case generate:
		pw, err = GeneratePassword()
		return pw, pw, err
	}
	if err := ValidatePassword(username, password); err != nil {
		return "", "", err
	}
	return password, "", nil
}

// mutateManaged rejects pure config users and includes them in the atomic last-admin guard.
func (s *Service) mutateManaged(ctx context.Context, username string,
	fn func(*ManagedUser) (remove, checkOtherAdmin bool, err error)) error {
	isManaged := false
	if s.managed != nil {
		if _, err := s.managed.Get(ctx, username); err == nil {
			isManaged = true
		}
	}
	if !isManaged && !s.conf.AllowManagedUsers {
		if _, ok := s.users[username]; ok {
			return fmt.Errorf("%w: console user %q is defined in config and cannot be modified", dom.ErrFailedPrecondition, username)
		}
	}
	configAdmins := 0
	configNames := make([]string, 0, len(s.users))
	for name, u := range s.users {
		if (s.conf.AllowManagedUsers || isManaged) && name == username {
			continue
		}
		configNames = append(configNames, name)
		if u.Role == RoleAdmin {
			configAdmins++
		}
	}
	return s.managed.mutate(ctx, username, configAdmins, configNames, fn)
}

// UpdateUser changes the role (empty keeps) and disabled flag (nil keeps) of a managed user. actor is the caller.
func (s *Service) UpdateUser(ctx context.Context, actor, username, role string, disabled *bool) error {
	if err := s.ready(); err != nil {
		return err
	}
	if role != "" && !validRole(role) {
		return fmt.Errorf("%w: role must be %q or %q", dom.ErrInvalidArg, RoleAdmin, RoleViewer)
	}
	return s.mutateManaged(ctx, username, func(m *ManagedUser) (bool, bool, error) {
		newRole, newDisabled := m.Role, m.Disabled
		if role != "" {
			newRole = role
		}
		if disabled != nil {
			newDisabled = *disabled
		}
		if username == actor {
			if newDisabled && !m.Disabled {
				return false, false, fmt.Errorf("%w: you cannot disable yourself", dom.ErrFailedPrecondition)
			}
			if newRole != m.Role && m.Role == RoleAdmin {
				return false, false, fmt.Errorf("%w: you cannot change your own role", dom.ErrFailedPrecondition)
			}
		}
		loseAdmin := m.Role == RoleAdmin && !m.Disabled && (newRole != RoleAdmin || newDisabled)
		m.Role, m.Disabled, m.UpdatedAt = newRole, newDisabled, s.now().UTC()
		return false, loseAdmin, nil
	})
}

// ResetPassword sets or generates a password of a managed user, forces a change at next login and revokes the
// sessions of the user (all sessions created before now stop working).
func (s *Service) ResetPassword(ctx context.Context, username, password string, generate bool) (string, error) {
	if err := s.ready(); err != nil {
		return "", err
	}
	pw, generated, err := s.passwordOrGenerate(username, password, generate)
	if err != nil {
		return "", err
	}
	hash, err := s.hashPassword(pw)
	if err != nil {
		return "", err
	}
	now := s.now().UTC()
	if err := s.mutateManaged(ctx, username, func(m *ManagedUser) (bool, bool, error) {
		m.PasswordHash, m.PasswordChangedAt, m.UpdatedAt, m.MustChangePassword = hash, now, now, true
		return false, false, nil
	}); err != nil {
		return "", err
	}
	return generated, nil
}

// DeleteUser removes a managed user. actor is the caller.
func (s *Service) DeleteUser(ctx context.Context, actor, username string) error {
	if err := s.ready(); err != nil {
		return err
	}
	if username == actor {
		return fmt.Errorf("%w: you cannot delete yourself", dom.ErrFailedPrecondition)
	}
	return s.mutateManaged(ctx, username, func(m *ManagedUser) (bool, bool, error) {
		return true, m.Role == RoleAdmin && !m.Disabled, nil
	})
}

// ChangeOwnPassword verifies the current password, sets the new one and revokes all other sessions of the user.
// token is the caller's session: it stays valid. ip is used for login throttling of the current password check.
func (s *Service) ChangeOwnPassword(ctx context.Context, token, current, newPassword, ip string) error {
	if err := s.ready(); err != nil {
		return err
	}
	sess, err := s.Authenticate(ctx, token)
	if err != nil {
		return err
	}
	if sess.Source != SourceManaged {
		return fmt.Errorf("%w: console user %q is defined in config: change its password hash in the worker config", dom.ErrFailedPrecondition, sess.Username)
	}
	if err := s.throttled(ctx, sess.Username, ip); err != nil {
		return err
	}
	m, err := s.managed.Get(ctx, sess.Username)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(m.PasswordHash), []byte(current)) != nil {
		if err := s.recordFailure(ctx, sess.Username, ip); err != nil {
			return err
		}
		return ErrInvalidCredentials
	}
	if err := ValidatePassword(sess.Username, newPassword); err != nil {
		return err
	}
	if newPassword == current {
		return fmt.Errorf("%w: the new password must differ from the current one", dom.ErrInvalidArg)
	}
	hash, err := s.hashPassword(newPassword)
	if err != nil {
		return err
	}
	now := s.now().UTC()
	if err := s.mutateManaged(ctx, sess.Username, func(latest *ManagedUser) (bool, bool, error) {
		if latest.PasswordHash != m.PasswordHash {
			return false, false, ErrInvalidCredentials
		}
		latest.PasswordHash, latest.PasswordChangedAt, latest.UpdatedAt, latest.MustChangePassword = hash, now, now, false
		return false, false, nil
	}); err != nil {
		return err
	}
	// Keep the caller signed in: re-stamp its session so it is not older than the password change.
	// StartedAt is kept (set from the old CreatedAt for legacy sessions) so the absolute cap does not restart.
	sess.StartedAt = sess.capAnchor()
	sess.CreatedAt = now
	return s.restamp(ctx, token, sess)
}

func (s *Service) cost() int {
	if s.bcryptCost > 0 {
		return s.bcryptCost
	}
	return BcryptCost
}
