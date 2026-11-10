package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

func newManagedSvc(t *testing.T, conf Config) (*Service, *miniredis.Miniredis, redis.Cmdable) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	svc, err := NewService(conf, rc, WithManagedUsers(rc))
	require.NoError(t, err)
	svc.bcryptCost = bcryptMinCost
	return svc, mr, rc
}

const bcryptMinCost = 4

const goodPw = "correct-horse-battery"

func TestPasswordPolicy(t *testing.T) {
	require.ErrorIs(t, ValidatePassword("u", "short"), dom.ErrInvalidArg)
	require.ErrorIs(t, ValidatePassword("u", strings.Repeat("a", 73)), dom.ErrInvalidArg)
	require.ErrorIs(t, ValidatePassword("abcdefghijklm", "abcdefghijklm"), dom.ErrInvalidArg)
	require.NoError(t, ValidatePassword("u", "123456789012"))
	pw, err := GeneratePassword()
	require.NoError(t, err)
	require.NoError(t, ValidatePassword("u", pw))
	pw2, _ := GeneratePassword()
	require.NotEqual(t, pw, pw2)
}

func TestConsoleUsers_CreateListAndLogin(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newManagedSvc(t, testConf(t, true))

	_, _, err := svc.CreateUser(ctx, "ops", RoleViewer, "short", false)
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, _, err = svc.CreateUser(ctx, "admin", RoleViewer, goodPw, false)
	require.ErrorIs(t, err, dom.ErrAlreadyExists, "config names are reserved")
	_, _, err = svc.CreateUser(ctx, "bad name", RoleViewer, goodPw, false)
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, _, err = svc.CreateUser(ctx, "ops", "root", goodPw, false)
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, _, err = svc.CreateUser(ctx, "ops", RoleViewer, goodPw, true)
	require.ErrorIs(t, err, dom.ErrInvalidArg, "password and generate are exclusive")

	info, generated, err := svc.CreateUser(ctx, "ops", RoleViewer, "", true)
	require.NoError(t, err)
	require.NotEmpty(t, generated)
	require.True(t, info.MustChangePassword)
	_, _, err = svc.CreateUser(ctx, "ops", RoleViewer, goodPw, false)
	require.ErrorIs(t, err, dom.ErrAlreadyExists)

	list, err := svc.ListUsers(ctx)
	require.NoError(t, err)
	require.Len(t, list, 3)
	bySource := map[string]string{}
	for _, u := range list {
		bySource[u.Username] = u.Source
	}
	require.Equal(t, map[string]string{"admin": "config", "view": "config", "ops": "managed"}, bySource)

	res, err := svc.Login(ctx, "ops", generated, "1.1.1.1")
	require.NoError(t, err)
	require.Equal(t, RoleViewer, res.Role)
	sess, err := svc.Authenticate(ctx, res.Token)
	require.NoError(t, err)
	require.Equal(t, SourceManaged, sess.Source)
	require.True(t, sess.MustChangePassword)

	list, _ = svc.ListUsers(ctx)
	for _, u := range list {
		if u.Username == "ops" {
			require.False(t, u.LastLoginAt.IsZero(), "last login recorded")
		}
	}
	_, err = svc.Login(ctx, "ops", "wrong-password-xx", "1.1.1.1")
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestConsoleUsers_ThrottlingStillApplies(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newManagedSvc(t, testConf(t, true))
	_, _, err := svc.CreateUser(ctx, "ops", RoleViewer, goodPw, false)
	require.NoError(t, err)
	for range maxFailures {
		_, err = svc.Login(ctx, "ops", "wrong-password-xx", "9.9.9.9")
		require.ErrorIs(t, err, ErrInvalidCredentials)
	}
	_, err = svc.Login(ctx, "ops", goodPw, "9.9.9.9")
	require.ErrorIs(t, err, ErrTooManyAttempts)
}

func TestConsoleUsers_DisableAndDeleteRevokeSessions(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newManagedSvc(t, testConf(t, true))
	_, _, err := svc.CreateUser(ctx, "ops", RoleViewer, goodPw, false)
	require.NoError(t, err)
	res, err := svc.Login(ctx, "ops", goodPw, "ip")
	require.NoError(t, err)
	_, err = svc.Authenticate(ctx, res.Token)
	require.NoError(t, err)

	yes := true
	require.NoError(t, svc.UpdateUser(ctx, "admin", "ops", "", &yes))
	_, err = svc.Authenticate(ctx, res.Token)
	require.ErrorIs(t, err, ErrInvalidSession, "disabled user's session stops working immediately")
	_, err = svc.Login(ctx, "ops", goodPw, "ip")
	require.ErrorIs(t, err, ErrInvalidCredentials, "disabled user cannot log in")

	no := false
	require.NoError(t, svc.UpdateUser(ctx, "admin", "ops", "", &no))
	_, err = svc.Authenticate(ctx, res.Token)
	require.NoError(t, err, "re-enabled user's old session works again")

	// role change applies to existing sessions
	require.NoError(t, svc.UpdateUser(ctx, "admin", "ops", RoleAdmin, nil))
	sess, err := svc.Authenticate(ctx, res.Token)
	require.NoError(t, err)
	require.Equal(t, RoleAdmin, sess.Role)

	require.NoError(t, svc.DeleteUser(ctx, "admin", "ops"))
	_, err = svc.Authenticate(ctx, res.Token)
	require.ErrorIs(t, err, ErrInvalidSession, "deleted user's session stops working")
	require.ErrorIs(t, svc.DeleteUser(ctx, "admin", "ops"), dom.ErrNotFound)
}

func TestConsoleUsers_PasswordChangeRevokesOtherSessions(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newManagedSvc(t, testConf(t, true))
	_, _, err := svc.CreateUser(ctx, "ops", RoleViewer, goodPw, false)
	require.NoError(t, err)
	a, err := svc.Login(ctx, "ops", goodPw, "ip")
	require.NoError(t, err)
	b, err := svc.Login(ctx, "ops", goodPw, "ip")
	require.NoError(t, err)

	require.ErrorIs(t, svc.ChangeOwnPassword(ctx, a.Token, "wrong-current-pw", "another-good-password", "ip"), ErrInvalidCredentials)
	require.ErrorIs(t, svc.ChangeOwnPassword(ctx, a.Token, goodPw, "short", "ip"), dom.ErrInvalidArg)
	require.ErrorIs(t, svc.ChangeOwnPassword(ctx, a.Token, goodPw, goodPw, "ip"), dom.ErrInvalidArg)
	require.NoError(t, svc.ChangeOwnPassword(ctx, a.Token, goodPw, "another-good-password", "ip"))

	_, err = svc.Authenticate(ctx, a.Token)
	require.NoError(t, err, "current session survives")
	_, err = svc.Authenticate(ctx, b.Token)
	require.ErrorIs(t, err, ErrInvalidSession, "other session revoked")
	_, err = svc.Login(ctx, "ops", goodPw, "ip")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	_, err = svc.Login(ctx, "ops", "another-good-password", "ip")
	require.NoError(t, err)

	// config users cannot change their password here
	adm, err := svc.Login(ctx, "admin", "adminpw", "ip")
	require.NoError(t, err)
	require.ErrorIs(t, svc.ChangeOwnPassword(ctx, adm.Token, "adminpw", "another-good-password", "ip"), dom.ErrFailedPrecondition)
}

func TestConsoleUsers_ResetPassword(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newManagedSvc(t, testConf(t, true))
	_, _, err := svc.CreateUser(ctx, "ops", RoleViewer, goodPw, false)
	require.NoError(t, err)
	old, err := svc.Login(ctx, "ops", goodPw, "ip")
	require.NoError(t, err)

	generated, err := svc.ResetPassword(ctx, "ops", "", true)
	require.NoError(t, err)
	require.NotEmpty(t, generated)
	_, err = svc.Authenticate(ctx, old.Token)
	require.ErrorIs(t, err, ErrInvalidSession, "reset revokes all sessions")
	res, err := svc.Login(ctx, "ops", generated, "ip")
	require.NoError(t, err)
	sess, _ := svc.Authenticate(ctx, res.Token)
	require.True(t, sess.MustChangePassword)

	_, err = svc.ResetPassword(ctx, "ops", "short", false)
	require.ErrorIs(t, err, dom.ErrInvalidArg)
	_, err = svc.ResetPassword(ctx, "admin", goodPw, false)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition, "config user")
	_, err = svc.ResetPassword(ctx, "nobody", goodPw, false)
	require.ErrorIs(t, err, dom.ErrNotFound)
}

func TestConsoleUsers_SafetyRules(t *testing.T) {
	ctx := context.Background()
	// only managed admins: config has a single viewer
	conf := Config{Enabled: true, Users: []User{{Username: "view", PasswordHash: hash(t, "viewpw"), Role: RoleViewer}}}
	svc, _, _ := newManagedSvc(t, conf)
	_, _, err := svc.CreateUser(ctx, "root1", RoleAdmin, goodPw, false)
	require.NoError(t, err)

	yes := true
	// last enabled admin
	require.ErrorIs(t, svc.UpdateUser(ctx, "other", "root1", RoleViewer, nil), dom.ErrFailedPrecondition)
	require.ErrorIs(t, svc.UpdateUser(ctx, "other", "root1", "", &yes), dom.ErrFailedPrecondition)
	require.ErrorIs(t, svc.DeleteUser(ctx, "other", "root1"), dom.ErrFailedPrecondition)

	_, _, err = svc.CreateUser(ctx, "root2", RoleAdmin, goodPw, false)
	require.NoError(t, err)
	// self protection
	require.ErrorIs(t, svc.UpdateUser(ctx, "root1", "root1", "", &yes), dom.ErrFailedPrecondition)
	require.ErrorIs(t, svc.UpdateUser(ctx, "root1", "root1", RoleViewer, nil), dom.ErrFailedPrecondition)
	require.ErrorIs(t, svc.DeleteUser(ctx, "root1", "root1"), dom.ErrFailedPrecondition)
	// two admins: one can be disabled, then the other is the last
	require.NoError(t, svc.UpdateUser(ctx, "root1", "root2", "", &yes))
	require.ErrorIs(t, svc.DeleteUser(ctx, "other", "root1"), dom.ErrFailedPrecondition)
	// disabled admins do not count, but deleting a disabled one is fine
	require.NoError(t, svc.DeleteUser(ctx, "root1", "root2"))

	// config users are read-only
	require.ErrorIs(t, svc.UpdateUser(ctx, "root1", "view", RoleAdmin, nil), dom.ErrFailedPrecondition)
	require.ErrorIs(t, svc.DeleteUser(ctx, "root1", "view"), dom.ErrFailedPrecondition)
}

func TestConsoleUsers_ConfigAdminCountsAsLastAdminGuard(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newManagedSvc(t, testConf(t, true)) // config admin exists
	_, _, err := svc.CreateUser(ctx, "root1", RoleAdmin, goodPw, false)
	require.NoError(t, err)
	yes := true
	require.NoError(t, svc.UpdateUser(ctx, "admin", "root1", "", &yes), "config admin keeps the system administrable")
}

func TestConsoleUsers_ConcurrentDemotionAndDeletePreserveLastAdmin(t *testing.T) {
	ctx := context.Background()
	conf := Config{Enabled: true, Users: []User{{Username: "view", PasswordHash: hash(t, "viewpw"), Role: RoleViewer}}}
	svc, _, rc := newManagedSvc(t, conf)
	other := *svc // separate service instance sharing Redis, without repeating bcrypt setup
	other.managed = NewUserStore(rc)
	for _, name := range []string{"root1", "root2"} {
		_, _, err := svc.CreateUser(ctx, name, RoleAdmin, goodPw, false)
		require.NoError(t, err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	go func() {
		<-start
		results <- svc.DeleteUser(ctx, "other", "root1")
	}()
	go func() {
		<-start
		results <- other.UpdateUser(ctx, "other", "root2", RoleViewer, nil)
	}()
	close(start)
	a, b := <-results, <-results
	require.True(t, (a == nil && errors.Is(b, dom.ErrFailedPrecondition)) ||
		(b == nil && errors.Is(a, dom.ErrFailedPrecondition)), "one mutation must retain the last admin: %v, %v", a, b)
	list, err := svc.ListUsers(ctx)
	require.NoError(t, err)
	admins := 0
	for _, u := range list {
		if u.Role == RoleAdmin && !u.Disabled {
			admins++
		}
	}
	require.Equal(t, 1, admins)
}

func TestUserStore_ConcurrentMutationsPreserveBothFields(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newManagedSvc(t, testConf(t, true))
	_, _, err := svc.CreateUser(ctx, "ops", RoleViewer, goodPw, false)
	require.NoError(t, err)
	var ready sync.WaitGroup
	ready.Add(2)
	release := make(chan struct{})
	results := make(chan error, 2)
	for _, change := range []func(*ManagedUser){
		func(u *ManagedUser) { u.Role = RoleAdmin },
		func(u *ManagedUser) { u.MustChangePassword = true },
	} {
		go func() {
			var first sync.Once
			results <- svc.managed.mutate(ctx, "ops", 1, []string{"admin", "view"}, func(u *ManagedUser) (bool, bool, error) {
				first.Do(func() { ready.Done(); <-release })
				change(u)
				return false, false, nil
			})
		}()
	}
	ready.Wait()
	close(release)
	require.NoError(t, <-results)
	require.NoError(t, <-results)
	u, err := svc.managed.Get(ctx, "ops")
	require.NoError(t, err)
	require.Equal(t, RoleAdmin, u.Role)
	require.True(t, u.MustChangePassword)
}

func TestConsoleUsers_ConfigWinsOnConflict(t *testing.T) {
	ctx := context.Background()
	svc, _, rc := newManagedSvc(t, testConf(t, true))
	// a stale managed record with the same name as a config user must be ignored
	raw, _ := json.Marshal(ManagedUser{Username: "admin", Role: RoleViewer, PasswordHash: hash(t, "other-password-1")})
	require.NoError(t, rc.HSet(ctx, ConsoleUsersKey, "admin", raw).Err())
	_, err := svc.Login(ctx, "admin", "other-password-1", "ip")
	require.ErrorIs(t, err, ErrInvalidCredentials)
	res, err := svc.Login(ctx, "admin", "adminpw", "ip")
	require.NoError(t, err)
	require.Equal(t, RoleAdmin, res.Role)
	list, err := svc.ListUsers(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
}

func TestConsoleUsers_DisabledAuth(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newManagedSvc(t, testConf(t, false))
	_, err := svc.ListUsers(ctx)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	require.Contains(t, err.Error(), "sign-in is disabled")
	_, _, err = svc.CreateUser(ctx, "ops", RoleViewer, goodPw, false)
	require.ErrorIs(t, err, dom.ErrFailedPrecondition)
}

func TestConsoleUsers_ListNeverContainsHash(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newManagedSvc(t, testConf(t, true))
	_, _, err := svc.CreateUser(ctx, "ops", RoleViewer, goodPw, false)
	require.NoError(t, err)
	list, _ := svc.ListUsers(ctx)
	raw, _ := json.Marshal(list)
	require.NotContains(t, string(raw), "$2")
	require.NotContains(t, strings.ToLower(string(raw)), "hash")
}

func TestHTTP_ChangePasswordAndMe(t *testing.T) {
	svc, _, _ := newManagedSvc(t, testConf(t, true))
	ctx := context.Background()
	_, _, err := svc.CreateUser(ctx, "ops", RoleViewer, goodPw, false)
	require.NoError(t, err)
	mux := runtime.NewServeMux()
	require.NoError(t, svc.RegisterHTTP(mux))
	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	rec := do(http.MethodPost, "/auth/login", "", `{"username":"ops","password":"`+goodPw+`"}`)
	require.Equal(t, 200, rec.Code)
	var lr struct{ Token string }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &lr))

	rec = do(http.MethodGet, "/auth/me", lr.Token, "")
	require.Contains(t, rec.Body.String(), `"source":"managed"`)
	require.Contains(t, rec.Body.String(), `"mustChangePassword":false`)

	require.Equal(t, 401, do(http.MethodPost, "/auth/password", "", `{"currentPassword":"a","newPassword":"b"}`).Code)
	require.Equal(t, 403, do(http.MethodPost, "/auth/password", lr.Token, `{"currentPassword":"nope-nope-nope","newPassword":"new-password-123"}`).Code)
	require.Equal(t, 400, do(http.MethodPost, "/auth/password", lr.Token, `{"currentPassword":"`+goodPw+`","newPassword":"short"}`).Code)
	require.Equal(t, 204, do(http.MethodPost, "/auth/password", lr.Token, `{"currentPassword":"`+goodPw+`","newPassword":"new-password-123"}`).Code)
	require.Equal(t, 200, do(http.MethodGet, "/auth/me", lr.Token, "").Code)
}

func TestViewerAllowList_Users(t *testing.T) {
	require.True(t, Allowed(RoleViewer, "/chorus.Resilience/ListS3Users"))
	for _, m := range []string{"CreateS3User", "UpdateS3UserCredentials", "DeleteS3User", "DeleteS3UserCredentials", "TestS3UserCredentials",
		"ListConsoleUsers", "CreateConsoleUser", "UpdateConsoleUser", "ResetConsoleUserPassword", "DeleteConsoleUser"} {
		require.False(t, Allowed(RoleViewer, "/chorus.Resilience/"+m), m)
		require.True(t, Allowed(RoleAdmin, "/chorus.Resilience/"+m), m)
	}
}
