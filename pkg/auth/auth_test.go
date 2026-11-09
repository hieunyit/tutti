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

	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
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
	pb.RegisterChorusServer(srv, pb.UnimplementedChorusServer{})
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
		_, err := pb.NewChorusClient(e.conn).GetStorages(context.Background(), empty)
		requireCode(t, err, codes.Unauthenticated)
	})
	t.Run("invalid token", func(t *testing.T) {
		_, err := pb.NewChorusClient(e.conn).GetStorages(withToken("bogus"), empty)
		requireCode(t, err, codes.Unauthenticated)
	})
	t.Run("expired token", func(t *testing.T) {
		tok := e.login(t, "admin", "adminpw")
		e.mr.FastForward(2 * time.Hour)
		_, err := pb.NewChorusClient(e.conn).GetStorages(withToken(tok), empty)
		requireCode(t, err, codes.Unauthenticated)
	})
	t.Run("viewer read ok, mutation denied", func(t *testing.T) {
		tok := e.login(t, "view", "viewpw")
		_, err := pb.NewChorusClient(e.conn).GetStorages(withToken(tok), empty)
		requireCode(t, err, codes.Unimplemented)
		_, err = policy.ListReplications(withToken(tok), &pb.ListReplicationsRequest{})
		requireCode(t, err, codes.Unimplemented)
		_, err = policy.AddReplication(withToken(tok), &pb.AddReplicationRequest{})
		requireCode(t, err, codes.PermissionDenied)
		_, err = pb.NewChorusClient(e.conn).GetProxyCredentials(withToken(tok), empty)
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
		require.True(t, Allowed(RoleAdmin, "/chorus.New/Thing"))
		require.False(t, Allowed(RoleViewer, "/chorus.New/Thing"))
	})
	t.Run("webhook and version public", func(t *testing.T) {
		_, err := pb.NewWebhookClient(e.conn).SwiftEvents(context.Background(), &pb.SwiftEventsRequest{})
		requireCode(t, err, codes.Unimplemented)
		_, err = pb.NewChorusClient(e.conn).GetAppVersion(context.Background(), empty)
		requireCode(t, err, codes.Unimplemented)
	})
}

func newHTTP(t *testing.T, enabled bool) (*env, http.Handler) {
	e := setup(t, enabled)
	mux := runtime.NewServeMux()
	require.NoError(t, e.svc.RegisterHTTP(mux))
	require.NoError(t, pb.RegisterChorusHandlerFromEndpoint(context.Background(), mux, e.addr,
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

	require.Equal(t, 401, do(h, "POST", "/auth/login", `{"username":"admin","password":"nope"}`, "").Code)
	require.Equal(t, 401, do(h, "POST", "/auth/login", `{"username":"ghost","password":"nope"}`, "").Code)
	require.Equal(t, 400, do(h, "POST", "/auth/login", `{}`, "").Code)
}

func TestHTTPRateLimit(t *testing.T) {
	_, h := newHTTP(t, true)
	for i := 0; i < maxFailures; i++ {
		require.Equal(t, 401, do(h, "POST", "/auth/login", `{"username":"admin","password":"bad"}`, "").Code)
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
	require.Equal(t, meResponse{Username: "view", Role: "viewer", AuthEnabled: true}, me)

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
