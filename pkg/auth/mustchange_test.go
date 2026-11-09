package auth

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

type fakeChorus struct{ pb.UnimplementedChorusServer }

func (fakeChorus) GetStorages(context.Context, *emptypb.Empty) (*pb.GetStoragesResponse, error) {
	return &pb.GetStoragesResponse{}, nil
}

// mustChangeEnv wires auth interceptors, a gRPC server and a grpc-gateway mux the way the worker does.
func mustChangeEnv(t *testing.T) (*Service, func(method, path, token, body string) *httptest.ResponseRecorder, *grpc.ClientConn) {
	t.Helper()
	svc, _, _ := newManagedSvc(t, testConf(t, true))
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.UnaryInterceptor(svc.UnaryInterceptor()), grpc.StreamInterceptor(svc.StreamInterceptor()))
	pb.RegisterChorusServer(srv, fakeChorus{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	ctx := context.Background()
	mux := runtime.NewServeMux()
	require.NoError(t, svc.RegisterHTTP(mux))
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	require.NoError(t, pb.RegisterChorusHandlerFromEndpoint(ctx, mux, lis.Addr().String(), opts))
	conn, err := grpc.NewClient(lis.Addr().String(), opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	do := func(method, path, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	return svc, do, conn
}

func TestMustChangePassword_Enforced(t *testing.T) {
	ctx := context.Background()
	svc, do, conn := mustChangeEnv(t)

	for _, role := range []string{RoleViewer, RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			name := "pw-" + role
			_, pw, err := svc.CreateUser(ctx, name, role, "", true)
			require.NoError(t, err)
			res, err := svc.Login(ctx, name, pw, "1.1.1.1")
			require.NoError(t, err)

			// gRPC: GetStorages is allowed for both roles normally.
			gctx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+res.Token)
			err = conn.Invoke(gctx, "/chorus.Chorus/GetStorages", &emptypb.Empty{}, &pb.GetStoragesResponse{})
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.Contains(t, status.Convert(err).Message(), "password change required")

			// REST through the gateway.
			rec := do(http.MethodGet, "/storage", res.Token, "")
			require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			var body struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Details []struct {
					Reason string `json:"reason"`
					Domain string `json:"domain"`
				} `json:"details"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, 7, body.Code)
			require.Equal(t, "password change required", body.Message)
			require.Len(t, body.Details, 1)
			require.Equal(t, "PASSWORD_CHANGE_REQUIRED", body.Details[0].Reason)

			// Allowed endpoints.
			rec = do(http.MethodGet, "/auth/me", res.Token, "")
			require.Equal(t, 200, rec.Code)
			require.Contains(t, rec.Body.String(), `"mustChangePassword":true`)

			// Change clears the flag and the session keeps working.
			require.Equal(t, 204, do(http.MethodPost, "/auth/password", res.Token, `{"currentPassword":"`+pw+`","newPassword":"new-password-123"}`).Code)
			rec = do(http.MethodGet, "/auth/me", res.Token, "")
			require.Equal(t, 200, rec.Code)
			require.Contains(t, rec.Body.String(), `"mustChangePassword":false`)
			err = conn.Invoke(gctx, "/chorus.Chorus/GetStorages", &emptypb.Empty{}, &pb.GetStoragesResponse{})
			require.NoError(t, err)
			require.Equal(t, 200, do(http.MethodGet, "/storage", res.Token, "").Code)

			// Admin reset sets the flag again; the new login is blocked, logout still works.
			npw, err := svc.ResetPassword(ctx, name, "", true)
			require.NoError(t, err)
			res2, err := svc.Login(ctx, name, npw, "1.1.1.1")
			require.NoError(t, err)
			g2 := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+res2.Token)
			err = conn.Invoke(g2, "/chorus.Chorus/GetStorages", &emptypb.Empty{}, &pb.GetStoragesResponse{})
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.Equal(t, 204, do(http.MethodPost, "/auth/logout", res2.Token, "").Code)
		})
	}
}

func TestMustChangePassword_ConfigUsersNeverFlagged(t *testing.T) {
	ctx := context.Background()
	svc, do, conn := mustChangeEnv(t)
	res, err := svc.Login(ctx, "view", "viewpw", "1.1.1.1")
	require.NoError(t, err)
	sess, err := svc.Authenticate(ctx, res.Token)
	require.NoError(t, err)
	require.False(t, sess.MustChangePassword)
	gctx := metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+res.Token)
	require.NoError(t, conn.Invoke(gctx, "/chorus.Chorus/GetStorages", &emptypb.Empty{}, &pb.GetStoragesResponse{}))
	rec := do(http.MethodGet, "/auth/me", res.Token, "")
	require.Equal(t, 200, rec.Code)
}
