package auth

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

func hash(t *testing.T, pw string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	require.NoError(t, err)
	return string(h)
}

func testConf(t *testing.T, enabled bool) Config {
	return Config{
		Enabled:    enabled,
		SessionTTL: time.Hour,
		Users: []User{
			{Username: "admin", PasswordHash: hash(t, "adminpw"), Role: RoleAdmin},
			{Username: "view", PasswordHash: hash(t, "viewpw"), Role: RoleViewer},
		},
	}
}

func TestConfigValidate(t *testing.T) {
	h := hash(t, "x")
	tests := []struct {
		name    string
		conf    Config
		wantErr string
	}{
		{"disabled ignores everything", Config{Enabled: false, Users: []User{{Username: ""}}}, ""},
		{"enabled without users", Config{Enabled: true}, "at least one user"},
		{"valid", Config{Enabled: true, Users: []User{{"a", h, "admin"}, {"b", h, "viewer"}}}, ""},
		{"duplicate", Config{Enabled: true, Users: []User{{"a", h, "admin"}, {"a", h, "viewer"}}}, "duplicate"},
		{"bad role", Config{Enabled: true, Users: []User{{"a", h, "root"}}}, "role"},
		{"plaintext hash", Config{Enabled: true, Users: []User{{"a", "password", "admin"}}}, "bcrypt"},
		{"empty username", Config{Enabled: true, Users: []User{{"", h, "admin"}}}, "username"},
		{"$2y$ accepted", Config{Enabled: true, Users: []User{{"a", "$2y$10$abc", "admin"}}}, ""},
		{"negative ttl", Config{Enabled: true, SessionTTL: -1, Users: []User{{"a", h, "admin"}}}, "sessionTTL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.conf.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

type env struct {
	mr   *miniredis.Miniredis
	svc  *Service
	conn *grpc.ClientConn
	addr string
}

func setup(t *testing.T, enabled bool) *env {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	svc, err := NewService(testConf(t, enabled), rdb)
	require.NoError(t, err)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.UnaryInterceptor(svc.UnaryInterceptor()), grpc.StreamInterceptor(svc.StreamInterceptor()))
	pb.RegisterTuttiServer(srv, pb.UnimplementedTuttiServer{})
	pb.RegisterPolicyServer(srv, pb.UnimplementedPolicyServer{})
	pb.RegisterWebhookServer(srv, pb.UnimplementedWebhookServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return &env{mr: mr, svc: svc, conn: conn, addr: lis.Addr().String()}
}

func (e *env) login(t *testing.T, user, pw string) string {
	t.Helper()
	res, err := e.svc.Login(context.Background(), user, pw, "1.2.3.4")
	require.NoError(t, err)
	return res.Token
}

func withToken(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

// Unimplemented from the stub server means the interceptor let the call through.
func requireCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	require.Equal(t, want, status.Code(err), "err: %v", err)
}

func TestInterceptorDisabled(t *testing.T) {
	e := setup(t, false)
	_, err := pb.NewPolicyClient(e.conn).AddReplication(context.Background(), &pb.AddReplicationRequest{})
	requireCode(t, err, codes.Unimplemented)
}

func TestInterceptorEnabled(t *testing.T) {
	e := setup(t, true)
	policy := pb.NewPolicyClient(e.conn)
	empty := &emptypb.Empty{}

	t.Run("missing token", func(t *testing.T) {
		_, err := pb.NewTuttiClient(e.conn).GetStorages(context.Background(), empty)
		requireCode(t, err, codes.Unauthenticated)
	})
	t.Run("invalid token", func(t *testing.T) {
		_, err := pb.NewTuttiClient(e.conn).GetStorages(withToken("bogus"), empty)
		requireCode(t, err, codes.Unauthenticated)
	})
	t.Run("expired token", func(t *testing.T) {
		tok := e.login(t, "admin", "adminpw")
		e.mr.FastForward(2 * time.Hour)
		_, err := pb.NewTuttiClient(e.conn).GetStorages(withToken(tok), empty)
		requireCode(t, err, codes.Unauthenticated)
	})
	t.Run("viewer read ok, mutation denied", func(t *testing.T) {
		tok := e.login(t, "view", "viewpw")
		_, err := pb.NewTuttiClient(e.conn).GetStorages(withToken(tok), empty)
		requireCode(t, err, codes.Unimplemented)
		_, err = policy.ListReplications(withToken(tok), &pb.ListReplicationsRequest{})
		requireCode(t, err, codes.Unimplemented)
		_, err = policy.AddReplication(withToken(tok), &pb.AddReplicationRequest{})
		requireCode(t, err, codes.PermissionDenied)
		_, err = pb.NewTuttiClient(e.conn).GetProxyCredentials(withToken(tok), empty)
		requireCode(t, err, codes.PermissionDenied)
		stream, err := policy.StreamReplication(withToken(tok), &pb.ReplicationID{})
		require.NoError(t, err)
		_, err = stream.Recv()
		requireCode(t, err, codes.Unimplemented)
	})
	t.Run("admin ok", func(t *testing.T) {
		tok := e.login(t, "admin", "adminpw")
		_, err := policy.AddReplication(withToken(tok), &pb.AddReplicationRequest{})
		requireCode(t, err, codes.Unimplemented)
	})
	t.Run("unknown method is admin only", func(t *testing.T) {
		require.True(t, Allowed(RoleAdmin, "/tutti.New/Thing"))
		require.False(t, Allowed(RoleViewer, "/tutti.New/Thing"))
	})
	t.Run("webhook and version public", func(t *testing.T) {
		_, err := pb.NewWebhookClient(e.conn).SwiftEvents(context.Background(), &pb.SwiftEventsRequest{})
		requireCode(t, err, codes.Unimplemented)
		_, err = pb.NewTuttiClient(e.conn).GetAppVersion(context.Background(), empty)
		requireCode(t, err, codes.Unimplemented)
	})
}

func newHTTP(t *testing.T, enabled bool) (*env, http.Handler) {
	e := setup(t, enabled)
	mux := runtime.NewServeMux()
	require.NoError(t, e.svc.RegisterHTTP(mux))
	require.NoError(t, pb.RegisterTuttiHandlerFromEndpoint(context.Background(), mux, e.addr,
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}))
	return e, mux
}

func do(h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "10.0.0.1:5555"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHTTPLogin(t *testing.T) {
	_, h := newHTTP(t, true)

	rec := do(h, "POST", "/auth/login", `{"username":"admin","password":"adminpw"}`, "")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	var res map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	require.NotEmpty(t, res["token"])
	require.Equal(t, "admin", res["username"])
	require.Equal(t, "admin", res["role"])
	_, err := time.Parse(time.RFC3339, res["expiresAt"])
	require.NoError(t, err)

	knownFailure := do(h, "POST", "/auth/login", `{"username":"admin","password":"nope"}`, "")
	unknownFailure := do(h, "POST", "/auth/login", `{"username":"ghost","password":"nope"}`, "")
	require.Equal(t, 401, knownFailure.Code)
	require.Equal(t, 401, unknownFailure.Code)
	var knownErr, unknownErr struct {
		Code              int    `json:"code"`
		Message           string `json:"message"`
		RemainingAttempts int    `json:"remainingAttempts"`
	}
	require.NoError(t, json.Unmarshal(knownFailure.Body.Bytes(), &knownErr))
	require.NoError(t, json.Unmarshal(unknownFailure.Body.Bytes(), &unknownErr))
	require.Equal(t, knownErr, unknownErr, "attempt feedback must not reveal whether the username exists")
	require.Equal(t, maxFailures-1, knownErr.RemainingAttempts)
	require.Equal(t, 400, do(h, "POST", "/auth/login", `{}`, "").Code)
}

func TestHTTPRateLimit(t *testing.T) {
	_, h := newHTTP(t, true)
	for i := 0; i < maxFailures; i++ {
		rec := do(h, "POST", "/auth/login", `{"username":"admin","password":"bad"}`, "")
		require.Equal(t, 401, rec.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, float64(maxFailures-i-1), body["remainingAttempts"])
	}
	// even the right password is now rejected
	require.Equal(t, 429, do(h, "POST", "/auth/login", `{"username":"admin","password":"adminpw"}`, "").Code)
	// other username from same IP is unaffected
	require.Equal(t, 200, do(h, "POST", "/auth/login", `{"username":"view","password":"viewpw"}`, "").Code)
}

func TestHTTPMeAndLogout(t *testing.T) {
	e, h := newHTTP(t, true)
	require.Equal(t, 401, do(h, "GET", "/auth/me", "", "").Code)

	tok := e.login(t, "view", "viewpw")
	rec := do(h, "GET", "/auth/me", "", tok)
	require.Equal(t, 200, rec.Code)
	var me meResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &me))
	require.NotEmpty(t, me.ExpiresAt)
	require.NotEmpty(t, me.MaxExpiresAt)
	require.NotEmpty(t, me.ServerTime)
	me.ExpiresAt, me.MaxExpiresAt, me.ServerTime = "", "", ""
	require.Equal(t, meResponse{Username: "view", Role: "viewer", AuthEnabled: true, Source: SourceConfig}, me)

	// token is forwarded through grpc-gateway to the interceptor
	require.Equal(t, 501, do(h, "GET", "/storage", "", tok).Code) // stub: Unimplemented
	require.Equal(t, 401, do(h, "GET", "/storage", "", "").Code)

	require.Equal(t, 401, do(h, "POST", "/auth/logout", "", "").Code)
	require.Equal(t, 204, do(h, "POST", "/auth/logout", "", tok).Code)
	require.Equal(t, 401, do(h, "GET", "/auth/me", "", tok).Code)
	require.Equal(t, 401, do(h, "GET", "/storage", "", tok).Code)
}

func TestHTTPMeDisabled(t *testing.T) {
	_, h := newHTTP(t, false)
	rec := do(h, "GET", "/auth/me", "", "")
	require.Equal(t, 200, rec.Code)
	var me meResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &me))
	require.Equal(t, meResponse{Role: "admin", AuthEnabled: false}, me)
	require.Equal(t, 404, do(h, "POST", "/auth/login", `{"username":"admin","password":"adminpw"}`, "").Code)
}

func req(remote string, hdr map[string]string) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = remote
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	return r
}

func TestResolveClientIP(t *testing.T) {
	trusted, err := parseTrustedProxies([]string{"10.0.0.0/8", "192.168.1.5", "fd00::/8"})
	require.NoError(t, err)
	tests := []struct {
		name string
		r    *http.Request
		want string
	}{
		{"untrusted peer spoofed xff", req("203.0.113.9:1", map[string]string{"X-Forwarded-For": "1.1.1.1", "X-Real-IP": "2.2.2.2"}), "203.0.113.9"},
		{"trusted peer", req("10.0.0.1:1", map[string]string{"X-Forwarded-For": "198.51.100.7"}), "198.51.100.7"},
		{"client-spoofed prefix ignored", req("10.0.0.1:1", map[string]string{"X-Forwarded-For": "6.6.6.6, 198.51.100.7"}), "198.51.100.7"},
		{"chained proxies", req("10.0.0.1:1", map[string]string{"X-Forwarded-For": "6.6.6.6, 198.51.100.7, 10.1.1.1, 192.168.1.5"}), "198.51.100.7"},
		{"x-real-ip fallback", req("10.0.0.1:1", map[string]string{"X-Real-IP": "198.51.100.8"}), "198.51.100.8"},
		{"trusted no headers", req("10.0.0.1:1", nil), "10.0.0.1"},
		{"all xff trusted falls to real-ip", req("10.0.0.1:1", map[string]string{"X-Forwarded-For": "10.2.2.2", "X-Real-IP": "198.51.100.9"}), "198.51.100.9"},
		{"ipv6 client", req("10.0.0.1:1", map[string]string{"X-Forwarded-For": "2001:db8::1"}), "2001:db8::1"},
		{"ipv6 trusted peer", req("[fd00::1]:1", map[string]string{"X-Forwarded-For": "2001:db8::2, fd00::5"}), "2001:db8::2"},
		{"ipv6 untrusted peer", req("[2001:db8::99]:1", map[string]string{"X-Forwarded-For": "1.1.1.1"}), "2001:db8::99"},
		{"garbage xff ignored", req("10.0.0.1:1", map[string]string{"X-Forwarded-For": "not-an-ip"}), "10.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, resolveClientIP(tt.r, trusted))
		})
	}
	// no trusted proxies configured: headers never used
	require.Equal(t, "10.0.0.1", resolveClientIP(req("10.0.0.1:1", map[string]string{"X-Forwarded-For": "1.1.1.1"}), nil))
}

func TestTrustedProxiesValidate(t *testing.T) {
	h := hash(t, "x")
	users := []User{{"a", h, "admin"}}
	require.NoError(t, (&Config{Enabled: true, Users: users, TrustedProxies: []string{"10.0.0.0/8", "127.0.0.1", "::1", "fd00::/8"}}).Validate())
	require.ErrorContains(t, (&Config{Enabled: true, Users: users, TrustedProxies: []string{"nginx"}}).Validate(), "trustedProxies")
	require.ErrorContains(t, (&Config{Enabled: true, Users: users, TrustedProxies: []string{"10.0.0.0/33"}}).Validate(), "trustedProxies")
}

func TestLockoutIsPerClientIP(t *testing.T) {
	e := setup(t, true)
	ctx := context.Background()
	for i := 0; i < maxFailures; i++ {
		_, err := e.svc.Login(ctx, "admin", "bad", "198.51.100.1")
		require.ErrorIs(t, err, ErrInvalidCredentials)
	}
	_, err := e.svc.Login(ctx, "admin", "adminpw", "198.51.100.1")
	require.ErrorIs(t, err, ErrTooManyAttempts)
	// a different client IP can still log in as the same user
	_, err = e.svc.Login(ctx, "admin", "adminpw", "198.51.100.2")
	require.NoError(t, err)
}

func (e *env) fakeSleep() *[]time.Duration {
	var delays []time.Duration
	e.svc.sleep = func(_ context.Context, d time.Duration) error { delays = append(delays, d); return nil }
	return &delays
}

func (e *env) spray(t *testing.T, username string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		ip := net.IPv4(10, byte(1+i/60000), byte(i/250%240), byte(i%250)).String()
		require.NoError(t, e.svc.recordFailure(context.Background(), username, ip))
	}
}

func TestUserBudgetDelaysUnknownIPButNotKnownIP(t *testing.T) {
	e := setup(t, true)
	delays := e.fakeSleep()
	ctx := context.Background()
	_, err := e.svc.Login(ctx, "admin", "adminpw", "203.0.113.7") // remember this pair
	require.NoError(t, err)
	e.spray(t, "admin", maxUserFailures)

	_, err = e.svc.Login(ctx, "admin", "adminpw", "203.0.113.7")
	require.NoError(t, err, "known pair is not blocked or delayed")
	require.Empty(t, *delays)

	_, err = e.svc.Login(ctx, "admin", "adminpw", "203.0.113.50")
	require.NoError(t, err, "unknown IP is delayed, not blocked")
	require.Equal(t, []time.Duration{minDelay}, *delays)
	// other users are unaffected
	_, err = e.svc.Login(ctx, "view", "viewpw", "203.0.113.50")
	require.NoError(t, err)
	require.Len(t, *delays, 1)
	// a successful login does not reset the username-wide counter
	n, err := e.mr.Get(userFailKey("admin"))
	require.NoError(t, err)
	require.Equal(t, "100", n)
}

func TestUserBudgetDelayGrowsAndCaps(t *testing.T) {
	require.Equal(t, time.Duration(0), userDelay(maxUserFailures-1))
	require.Equal(t, time.Second, userDelay(maxUserFailures))
	require.Equal(t, 2*time.Second, userDelay(maxUserFailures+delayStep))
	require.Equal(t, 16*time.Second, userDelay(maxUserFailures+4*delayStep))
	require.Equal(t, maxDelay, userDelay(maxUserFailures+5*delayStep))
	require.Equal(t, maxDelay, userDelay(maxUserHardFailures-1))
}

func TestUserBudgetHardCeilingRefusesUnknownOnly(t *testing.T) {
	e := setup(t, true)
	delays := e.fakeSleep()
	ctx := context.Background()
	_, err := e.svc.Login(ctx, "admin", "adminpw", "203.0.113.7")
	require.NoError(t, err)
	e.spray(t, "admin", maxUserHardFailures)

	_, err = e.svc.Login(ctx, "admin", "adminpw", "203.0.113.50")
	require.ErrorIs(t, err, ErrTooManyAttempts)
	_, err = e.svc.Login(ctx, "admin", "adminpw", "203.0.113.7")
	require.NoError(t, err, "attacker spraying from many IPs cannot lock out the known admin address")
	require.Empty(t, *delays)
	_, err = e.svc.Login(ctx, "view", "viewpw", "203.0.113.50")
	require.NoError(t, err)

	// the per-(user, IP) budget still applies to a known pair
	for i := 0; i < maxFailures; i++ {
		_, err = e.svc.Login(ctx, "admin", "bad", "203.0.113.7")
		require.ErrorIs(t, err, ErrInvalidCredentials)
	}
	_, err = e.svc.Login(ctx, "admin", "adminpw", "203.0.113.7")
	require.ErrorIs(t, err, ErrTooManyAttempts)
}

func TestUserBudgetCountersExpire(t *testing.T) {
	e := setup(t, true)
	e.fakeSleep()
	ctx := context.Background()
	e.spray(t, "admin", maxUserHardFailures)
	_, err := e.svc.Login(ctx, "admin", "adminpw", "203.0.113.50")
	require.ErrorIs(t, err, ErrTooManyAttempts)
	e.mr.FastForward(failWindow + time.Second)
	_, err = e.svc.Login(ctx, "admin", "adminpw", "203.0.113.50")
	require.NoError(t, err)

	// known marker expires after 30 days
	require.True(t, e.mr.Exists(knownKey("admin", "203.0.113.50")))
	e.mr.FastForward(knownTTL + time.Second)
	require.False(t, e.mr.Exists(knownKey("admin", "203.0.113.50")))
}

func TestUserBudgetDelayHonoursContext(t *testing.T) {
	e := setup(t, true)
	e.spray(t, "admin", maxUserFailures)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := e.svc.Login(ctx, "admin", "adminpw", "203.0.113.50")
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(start), 900*time.Millisecond)
}

func TestRemainingAttemptsUsesHardCeilingForUnknownIPs(t *testing.T) {
	e := setup(t, true)
	e.fakeSleep()
	ctx := context.Background()
	for _, username := range []string{"admin", "ghost"} {
		e.spray(t, username, maxUserHardFailures-2)
		_, err := e.svc.Login(ctx, username, "bad", "198.51.100.40")
		var credentialErr *InvalidCredentialsError
		require.ErrorAs(t, err, &credentialErr)
		require.Equal(t, 1, credentialErr.RemainingAttempts)

		_, err = e.svc.Login(ctx, username, "bad", "198.51.100.41")
		require.ErrorAs(t, err, &credentialErr)
		require.Equal(t, 0, credentialErr.RemainingAttempts)
	}
}
