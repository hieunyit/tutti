package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// fakeClock drives both the service clock and miniredis key expiry.
type fakeClock struct {
	t  time.Time
	mr *miniredis.Miniredis
}

func (c *fakeClock) Now() time.Time { return c.t }
func (c *fakeClock) Advance(d time.Duration) {
	c.t = c.t.Add(d)
	c.mr.FastForward(d)
}

func newClockSvc(t *testing.T, conf Config) (*Service, *fakeClock, *miniredis.Miniredis) {
	t.Helper()
	svc, mr, _ := newManagedSvc(t, conf)
	clk := &fakeClock{t: time.Date(2026, 11, 13, 8, 0, 0, 0, time.UTC), mr: mr}
	svc.now = clk.Now
	return svc, clk, mr
}

func sessConf(t *testing.T) Config {
	c := testConf(t, true)
	c.SessionTTL = 0
	return c
}

func sessionTTL(t *testing.T, mr *miniredis.Miniredis, token string) time.Duration {
	t.Helper()
	return mr.TTL(sessionPrefix + hashToken(token))
}

func TestAuthenticateDoesNotExtendSession(t *testing.T) {
	ctx := context.Background()
	svc, clk, _ := newClockSvc(t, sessConf(t))
	res, err := svc.Login(ctx, "admin", "adminpw", "1.2.3.4")
	require.NoError(t, err)
	var last error
	for i := 0; i < 100; i++ {
		clk.Advance(time.Duration(31*60/100+1) * time.Second)
		_, last = svc.Authenticate(ctx, res.Token)
		if last != nil {
			break
		}
	}
	require.ErrorIs(t, last, ErrInvalidSession)
}

func TestRefreshExtendsToIdle(t *testing.T) {
	ctx := context.Background()
	svc, clk, mr := newClockSvc(t, sessConf(t))
	res, err := svc.Login(ctx, "admin", "adminpw", "1.2.3.4")
	require.NoError(t, err)
	require.Equal(t, clk.Now().Add(30*time.Minute), res.ExpiresAt)
	require.Equal(t, clk.Now().Add(24*time.Hour), res.MaxExpiresAt)
	require.Equal(t, clk.Now(), res.ServerTime)

	clk.Advance(20 * time.Minute)
	require.Equal(t, 10*time.Minute, sessionTTL(t, mr, res.Token))
	st, err := svc.Refresh(ctx, res.Token)
	require.NoError(t, err)
	require.Equal(t, 30*time.Minute, sessionTTL(t, mr, res.Token))
	require.Equal(t, clk.Now().Add(30*time.Minute), st.ExpiresAt)
	require.Equal(t, res.MaxExpiresAt, st.MaxExpiresAt)
	require.Equal(t, clk.Now(), st.ServerTime)

	clk.Advance(29 * time.Minute)
	_, err = svc.Authenticate(ctx, res.Token)
	require.NoError(t, err)
}

func TestRefreshCappedAtMax(t *testing.T) {
	ctx := context.Background()
	svc, clk, mr := newClockSvc(t, sessConf(t))
	created := clk.Now()
	res, err := svc.Login(ctx, "admin", "adminpw", "1.2.3.4")
	require.NoError(t, err)

	// Keep the session alive with a refresh every 25 minutes up to 23h59m.
	for clk.Now().Sub(created) < 23*time.Hour+30*time.Minute {
		clk.Advance(25 * time.Minute)
		_, err = svc.Refresh(ctx, res.Token)
		require.NoError(t, err)
	}
	clk.Advance(created.Add(23*time.Hour + 59*time.Minute).Sub(clk.Now()))
	st, err := svc.Refresh(ctx, res.Token)
	require.NoError(t, err)
	require.Equal(t, created.Add(24*time.Hour), st.ExpiresAt)
	require.Equal(t, time.Minute, sessionTTL(t, mr, res.Token))

	clk.Advance(time.Minute)
	_, err = svc.Refresh(ctx, res.Token)
	require.ErrorIs(t, err, ErrInvalidSession)
}

func TestRefreshAtCapWithKeyStillPresent(t *testing.T) {
	ctx := context.Background()
	svc, clk, mr := newClockSvc(t, sessConf(t))
	res, err := svc.Login(ctx, "admin", "adminpw", "1.2.3.4")
	require.NoError(t, err)
	// Clock moves without Redis expiry (e.g. skew): the cap alone must reject.
	clk.t = clk.t.Add(24 * time.Hour)
	_, err = svc.Refresh(ctx, res.Token)
	require.ErrorIs(t, err, ErrInvalidSession)
	require.False(t, mr.Exists(sessionPrefix+hashToken(res.Token)))
}

func TestRefreshInvalidToken(t *testing.T) {
	svc, _, _ := newClockSvc(t, sessConf(t))
	_, err := svc.Refresh(context.Background(), "nope")
	require.ErrorIs(t, err, ErrInvalidSession)
	_, err = svc.Refresh(context.Background(), "")
	require.ErrorIs(t, err, ErrInvalidSession)
}

func TestRefreshRateLimited(t *testing.T) {
	ctx := context.Background()
	svc, clk, _ := newClockSvc(t, sessConf(t))
	res, err := svc.Login(ctx, "admin", "adminpw", "1.2.3.4")
	require.NoError(t, err)
	for i := 0; i < 6; i++ {
		_, err = svc.Refresh(ctx, res.Token)
		require.NoError(t, err, "call %d", i+1)
	}
	_, err = svc.Refresh(ctx, res.Token)
	require.ErrorIs(t, err, ErrRefreshRateLimited)
	clk.Advance(61 * time.Second)
	_, err = svc.Refresh(ctx, res.Token)
	require.NoError(t, err)
}

func TestPasswordChangeInvalidatesAfterRefresh(t *testing.T) {
	ctx := context.Background()
	svc, clk, _ := newClockSvc(t, sessConf(t))
	_, pw, err := svc.CreateUser(ctx, "carol", RoleAdmin, "", true)
	require.NoError(t, err)
	res, err := svc.Login(ctx, "carol", pw, "1.2.3.4")
	require.NoError(t, err)
	clk.Advance(5 * time.Minute)
	_, err = svc.Refresh(ctx, res.Token)
	require.NoError(t, err)
	clk.Advance(time.Minute)
	_, err = svc.ResetPassword(ctx, "carol", "", true)
	require.NoError(t, err)
	_, err = svc.Refresh(ctx, res.Token)
	require.ErrorIs(t, err, ErrInvalidSession)
	_, err = svc.Authenticate(ctx, res.Token)
	require.ErrorIs(t, err, ErrInvalidSession)
}

func TestLegacySessionWithoutLastActiveAt(t *testing.T) {
	ctx := context.Background()
	svc, clk, mr := newClockSvc(t, sessConf(t))
	data, err := json.Marshal(map[string]any{"username": "admin", "role": RoleAdmin, "createdAt": clk.Now()})
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	require.NoError(t, rdb.Set(ctx, sessionPrefix+hashToken("legacy"), data, 12*time.Hour).Err())

	clk.Advance(time.Hour)
	_, err = svc.Authenticate(ctx, "legacy")
	require.NoError(t, err)
	st, err := svc.Refresh(ctx, "legacy")
	require.NoError(t, err)
	require.Equal(t, clk.Now().Add(30*time.Minute), st.ExpiresAt)
	require.Equal(t, clk.Now().Add(-time.Hour).Add(24*time.Hour), st.MaxExpiresAt)
}

func TestLegacySessionTTLUsedAsMax(t *testing.T) {
	c := Config{SessionTTL: 2 * time.Hour}
	require.Equal(t, 2*time.Hour, c.MaxLifetime())
	require.Equal(t, 30*time.Minute, c.IdleTimeout())
	c = Config{SessionTTL: 10 * time.Minute}
	require.Equal(t, 10*time.Minute, c.MaxLifetime())
	require.Equal(t, 10*time.Minute, c.IdleTimeout())
	c = Config{SessionTTL: 2 * time.Hour, SessionMaxLifetime: 5 * time.Hour, SessionIdleTimeout: time.Minute}
	require.Equal(t, 5*time.Hour, c.MaxLifetime())
	require.Equal(t, time.Minute, c.IdleTimeout())
	require.Equal(t, 24*time.Hour, Config{}.MaxLifetime())
	require.Equal(t, 30*time.Minute, Config{}.IdleTimeout())
}

func TestConfigValidateRejectsBadTimes(t *testing.T) {
	h := hash(t, "x")
	users := []User{{"a", h, "admin"}}
	for name, c := range map[string]Config{
		"negative idle":   {Enabled: true, Users: users, SessionIdleTimeout: -time.Minute},
		"negative max":    {Enabled: true, Users: users, SessionMaxLifetime: -time.Hour},
		"max below idle":  {Enabled: true, Users: users, SessionIdleTimeout: time.Hour, SessionMaxLifetime: time.Minute},
		"max below dflt":  {Enabled: true, Users: users, SessionMaxLifetime: 10 * time.Minute},
		"idle above dflt": {Enabled: true, Users: users, SessionIdleTimeout: 48 * time.Hour},
	} {
		t.Run(name, func(t *testing.T) { require.Error(t, c.Validate()) })
	}
	ok := Config{Enabled: true, Users: users, SessionIdleTimeout: 2 * time.Minute, SessionMaxLifetime: 5 * time.Minute}
	require.NoError(t, ok.Validate())
}

func TestHTTPLoginMeRefresh(t *testing.T) {
	svc, clk, _ := newClockSvc(t, sessConf(t))
	mx := runtime.NewServeMux()
	require.NoError(t, svc.RegisterHTTP(mx))
	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		mx.ServeHTTP(rec, req)
		return rec
	}
	rec := do(http.MethodPost, "/auth/login", "", `{"username":"admin","password":"adminpw"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	var login map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &login))
	require.Equal(t, "2026-11-13T08:30:00Z", login["expiresAt"])
	require.Equal(t, "2026-11-14T08:00:00Z", login["maxExpiresAt"])
	require.Equal(t, "2026-11-13T08:00:00Z", login["serverTime"])
	token := login["token"].(string)

	clk.Advance(10 * time.Minute)
	rec = do(http.MethodGet, "/auth/me", token, "")
	require.Equal(t, http.StatusOK, rec.Code)
	var me map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &me))
	require.Equal(t, "2026-11-13T08:30:00Z", me["expiresAt"], "me must not extend the session")
	require.Equal(t, "2026-11-14T08:00:00Z", me["maxExpiresAt"])
	require.Equal(t, "2026-11-13T08:10:00Z", me["serverTime"])

	rec = do(http.MethodPost, "/auth/refresh", token, "")
	require.Equal(t, http.StatusOK, rec.Code)
	var ref map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &ref))
	require.Equal(t, "2026-11-13T08:40:00Z", ref["expiresAt"])
	require.Equal(t, "2026-11-14T08:00:00Z", ref["maxExpiresAt"])
	require.Equal(t, "2026-11-13T08:10:00Z", ref["serverTime"])

	require.Equal(t, http.StatusUnauthorized, do(http.MethodPost, "/auth/refresh", "bad", "").Code)
	require.Equal(t, http.StatusUnauthorized, do(http.MethodPost, "/auth/refresh", "", "").Code)

	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusOK, do(http.MethodPost, "/auth/refresh", token, "").Code)
	}
	rec = do(http.MethodPost, "/auth/refresh", token, "")
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.NotEmpty(t, rec.Header().Get("Retry-After"))
}

func TestHTTPRefreshAllowedWhenMustChangePassword(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newClockSvc(t, sessConf(t))
	_, pw, err := svc.CreateUser(ctx, "newbie", RoleViewer, "", true)
	require.NoError(t, err)
	res, err := svc.Login(ctx, "newbie", pw, "1.1.1.1")
	require.NoError(t, err)
	mx := runtime.NewServeMux()
	require.NoError(t, svc.RegisterHTTP(mx))
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+res.Token)
	rec := httptest.NewRecorder()
	mx.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestOwnPasswordChangeKeepsOriginalCap(t *testing.T) {
	ctx := context.Background()
	svc, clk, _ := newClockSvc(t, sessConf(t))
	_, _, err := svc.CreateUser(ctx, "dave", RoleViewer, goodPw, false)
	require.NoError(t, err)
	started := clk.Now()
	a, err := svc.Login(ctx, "dave", goodPw, "1.2.3.4")
	require.NoError(t, err)
	b, err := svc.Login(ctx, "dave", goodPw, "1.2.3.4")
	require.NoError(t, err)
	for clk.Now().Sub(started) < 23*time.Hour+40*time.Minute {
		clk.Advance(20 * time.Minute)
		_, err = svc.Refresh(ctx, a.Token)
		require.NoError(t, err)
	}
	require.NoError(t, svc.ChangeOwnPassword(ctx, a.Token, goodPw, "another-good-password", "1.2.3.4"))
	_, err = svc.Authenticate(ctx, b.Token)
	require.ErrorIs(t, err, ErrInvalidSession, "other session revoked")
	_, err = svc.Authenticate(ctx, a.Token)
	require.NoError(t, err, "own session continues")

	st, err := svc.Refresh(ctx, a.Token)
	require.NoError(t, err)
	require.Equal(t, started.Add(24*time.Hour), st.MaxExpiresAt)
	require.Equal(t, started.Add(24*time.Hour), st.ExpiresAt, "cap not restarted by the password change")
}

func TestLegacySessionWithoutStartedAtUsesCreatedAt(t *testing.T) {
	s := &Service{conf: sessConf(t)}
	created := time.Date(2026, 11, 13, 8, 0, 0, 0, time.UTC)
	st := s.Times(&Session{CreatedAt: created}, created.Add(time.Hour))
	require.Equal(t, created.Add(24*time.Hour), st.MaxExpiresAt)
	st = s.Times(&Session{CreatedAt: created.Add(time.Hour), StartedAt: created}, created.Add(2*time.Hour))
	require.Equal(t, created.Add(24*time.Hour), st.MaxExpiresAt)
}
