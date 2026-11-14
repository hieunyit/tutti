package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hieunyit/tutti/pkg/dom"
)

const (
	envBootstrapPassword    = "TUTTI_BOOTSTRAP_ADMIN_PASSWORD"
	envBootstrapForceChange = "TUTTI_BOOTSTRAP_ADMIN_FORCE_CHANGE"

	// ResetPasswordCommand is the recovery command as run inside the worker container image.
	ResetPasswordCommand = "docker compose exec worker /bin/chorus admin reset-password"
)

// bannerOut is where password banners are written. It is stderr, never the structured logger.
var bannerOut io.Writer = os.Stderr

// forceChangeEnabled reports whether a bootstrapped or reset password must be changed at the next login.
func forceChangeEnabled() bool {
	v := strings.TrimSpace(os.Getenv(envBootstrapForceChange))
	return v != "false" && v != "0"
}

// PrintPasswordBanner writes a human readable banner with a generated password to bannerOut.
func PrintPasswordBanner(title, username, password string) {
	const bar = "============================================================"
	_, _ = fmt.Fprintf(bannerOut, "\n%s\n%s\nusername: %s\npassword: %s\n"+
		"You must change it at first login. To reset later run: %s\n%s\n\n",
		bar, title, username, password, ResetPasswordCommand, bar)
}

// ResetUserPassword sets a new password for the console user (created as admin when missing), marks the password as
// to-be-changed (unless TUTTI_BOOTSTRAP_ADMIN_FORCE_CHANGE=false and the password was supplied) and stamps
// PasswordChangedAt so every existing session of the user stops working. When password is empty a random one is
// generated and returned.
func (s *Service) ResetUserPassword(ctx context.Context, username, password string) (generated string, err error) {
	if err := s.ready(); err != nil {
		return "", err
	}
	if !usernameRe.MatchString(username) {
		return "", fmt.Errorf("%w: invalid username %q", dom.ErrInvalidArg, username)
	}
	if password == "" {
		if password, err = GeneratePassword(); err != nil {
			return "", err
		}
		generated = password
	}
	if err := ValidatePassword(username, password); err != nil {
		return "", err
	}
	hash, err := s.hashPassword(password)
	if err != nil {
		return "", err
	}
	mustChange := generated != "" || forceChangeEnabled()
	now := s.now().UTC()
	if _, gerr := s.managed.Get(ctx, username); gerr != nil {
		if !errors.Is(gerr, dom.ErrNotFound) {
			return "", gerr
		}
		if cerr := s.managed.Create(ctx, ManagedUser{
			Username: username, Role: RoleAdmin, PasswordHash: hash, CreatedAt: now, UpdatedAt: now,
			PasswordChangedAt: now, MustChangePassword: mustChange,
		}); cerr != nil {
			return "", cerr
		}
		return generated, nil
	}
	if err := s.mutateManaged(ctx, username, func(m *ManagedUser) (bool, bool, error) {
		m.PasswordHash, m.PasswordChangedAt, m.UpdatedAt, m.MustChangePassword = hash, now, now, mustChange
		return false, false, nil
	}); err != nil {
		return "", err
	}
	return generated, nil
}
