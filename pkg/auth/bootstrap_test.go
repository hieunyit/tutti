package auth

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

func newBootstrapSvc(t *testing.T, conf Config) *Service {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	svc, err := NewService(conf, rc, WithManagedUsers(rc))
	require.NoError(t, err)
	svc.bcryptCost = bcryptMinCost
	return svc
}

func captureOutput(t *testing.T) (logs, banner *bytes.Buffer) {
	t.Helper()
	logs, banner = &bytes.Buffer{}, &bytes.Buffer{}
	oldLog, oldOut := log.Logger, bannerOut
	log.Logger = zerolog.New(logs)
	bannerOut = banner
	t.Cleanup(func() { log.Logger, bannerOut = oldLog, oldOut })
	return logs, banner
}

func managedConf() Config {
	return Config{Enabled: true, SessionTTL: 12 * time.Hour, AllowManagedUsers: true}
}

func TestEnsureAdmin_GeneratedPasswordNotInLogs(t *testing.T) {
	t.Setenv("TUTTI_BOOTSTRAP_ADMIN_PASSWORD", "")
	logs, banner := captureOutput(t)
	svc := newBootstrapSvc(t, managedConf())
	require.NoError(t, svc.EnsureAdmin(context.Background()))

	out := banner.String()
	require.Contains(t, out, "INITIAL ADMIN PASSWORD")
	require.Contains(t, out, "username: admin")
	require.Contains(t, out, "admin reset-password")
	var pw string
	for _, l := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(l, "password: "); ok {
			pw = v
		}
	}
	require.NotEmpty(t, pw)
	require.Equal(t, 1, strings.Count(out, pw), "password printed exactly once")
	require.NotContains(t, logs.String(), pw, "secret must never reach the structured log")
	require.Contains(t, logs.String(), "initial admin password generated; see the banner on stderr")

	// the printed password works and a change is forced
	login, err := svc.Login(context.Background(), "admin", pw, "ip")
	require.NoError(t, err)
	sess, err := svc.Authenticate(context.Background(), login.Token)
	require.NoError(t, err)
	require.True(t, sess.MustChangePassword)
}

func TestEnsureAdmin_ProvidedPassword(t *testing.T) {
	ctx := context.Background()
	t.Run("empty env behaves as unset", func(t *testing.T) {
		t.Setenv("TUTTI_BOOTSTRAP_ADMIN_PASSWORD", "   ")
		_, banner := captureOutput(t)
		require.NoError(t, newBootstrapSvc(t, managedConf()).EnsureAdmin(ctx))
		require.Contains(t, banner.String(), "INITIAL ADMIN PASSWORD")
	})
	t.Run("too short fails and names the variable", func(t *testing.T) {
		t.Setenv("TUTTI_BOOTSTRAP_ADMIN_PASSWORD", "short")
		_, banner := captureOutput(t)
		err := newBootstrapSvc(t, managedConf()).EnsureAdmin(ctx)
		require.ErrorIs(t, err, dom.ErrInvalidArg)
		require.Contains(t, err.Error(), "TUTTI_BOOTSTRAP_ADMIN_PASSWORD")
		require.Contains(t, err.Error(), "12")
		require.Empty(t, banner.String())
	})
	t.Run("config value too short", func(t *testing.T) {
		t.Setenv("TUTTI_BOOTSTRAP_ADMIN_PASSWORD", "")
		conf := managedConf()
		conf.BootstrapAdminPassword = "short"
		err := newBootstrapSvc(t, conf).EnsureAdmin(ctx)
		require.ErrorIs(t, err, dom.ErrInvalidArg)
		require.Contains(t, err.Error(), "bootstrapAdminPassword")
	})
	t.Run("valid password prints nothing", func(t *testing.T) {
		t.Setenv("TUTTI_BOOTSTRAP_ADMIN_PASSWORD", "a-long-enough-pw")
		logs, banner := captureOutput(t)
		require.NoError(t, newBootstrapSvc(t, managedConf()).EnsureAdmin(ctx))
		require.Empty(t, banner.String())
		require.NotContains(t, logs.String(), "a-long-enough-pw")
	})
}

func TestResetUserPassword(t *testing.T) {
	ctx := context.Background()
	t.Setenv("TUTTI_BOOTSTRAP_ADMIN_FORCE_CHANGE", "")

	t.Run("existing user, sessions invalidated, role kept", func(t *testing.T) {
		svc := newBootstrapSvc(t, managedConf())
		_, _, err := svc.CreateUser(ctx, "boss", RoleAdmin, goodPw, false)
		require.NoError(t, err)
		login, err := svc.Login(ctx, "boss", goodPw, "ip")
		require.NoError(t, err)
		_, err = svc.Authenticate(ctx, login.Token)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)
		generated, err := svc.ResetUserPassword(ctx, "boss", "")
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(generated), MinPasswordLength)

		_, err = svc.Authenticate(ctx, login.Token)
		require.Error(t, err, "old session must be revoked")
		_, err = svc.Login(ctx, "boss", goodPw, "ip")
		require.Error(t, err, "old password must stop working")
		l2, err := svc.Login(ctx, "boss", generated, "ip")
		require.NoError(t, err)
		sess, err := svc.Authenticate(ctx, l2.Token)
		require.NoError(t, err)
		require.True(t, sess.MustChangePassword)
		users, err := svc.ListUsers(ctx)
		require.NoError(t, err)
		require.Equal(t, RoleAdmin, users[0].Role)
	})
	t.Run("missing user is created as admin", func(t *testing.T) {
		svc := newBootstrapSvc(t, managedConf())
		generated, err := svc.ResetUserPassword(ctx, "admin", "")
		require.NoError(t, err)
		users, err := svc.ListUsers(ctx)
		require.NoError(t, err)
		require.Len(t, users, 1)
		require.Equal(t, RoleAdmin, users[0].Role)
		require.True(t, users[0].MustChangePassword)
		_, err = svc.Login(ctx, "admin", generated, "ip")
		require.NoError(t, err)
	})
	t.Run("explicit password min length", func(t *testing.T) {
		svc := newBootstrapSvc(t, managedConf())
		_, err := svc.ResetUserPassword(ctx, "admin", "short")
		require.ErrorIs(t, err, dom.ErrInvalidArg)
		generated, err := svc.ResetUserPassword(ctx, "admin", "my-new-password-1")
		require.NoError(t, err)
		require.Empty(t, generated, "nothing generated when a password is supplied")
		_, err = svc.Login(ctx, "admin", "my-new-password-1", "ip")
		require.NoError(t, err)
	})
	t.Run("force change can be disabled for a supplied password", func(t *testing.T) {
		t.Setenv("TUTTI_BOOTSTRAP_ADMIN_FORCE_CHANGE", "false")
		svc := newBootstrapSvc(t, managedConf())
		_, err := svc.ResetUserPassword(ctx, "admin", "my-new-password-1")
		require.NoError(t, err)
		users, _ := svc.ListUsers(ctx)
		require.False(t, users[0].MustChangePassword)
	})
	t.Run("invalid username", func(t *testing.T) {
		svc := newBootstrapSvc(t, managedConf())
		_, err := svc.ResetUserPassword(ctx, "bad name", "")
		require.ErrorIs(t, err, dom.ErrInvalidArg)
	})
}
