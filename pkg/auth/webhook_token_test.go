package auth

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

const testWebhookToken = "s3cr3t-webhook-token"

func webhookServer(t *testing.T, token string) (*grpc.ClientConn, http.Handler) {
	t.Helper()
	u, s := WebhookTokenInterceptors(token)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(u), grpc.ChainStreamInterceptor(s))
	pb.RegisterWebhookServer(srv, pb.UnimplementedWebhookServer{})
	pb.RegisterChorusServer(srv, pb.UnimplementedChorusServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	conn, err := grpc.NewClient(lis.Addr().String(), opts...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	mux := runtime.NewServeMux(runtime.WithIncomingHeaderMatcher(GatewayHeaderMatcher))
	require.NoError(t, pb.RegisterWebhookHandlerFromEndpoint(context.Background(), mux, lis.Addr().String(), opts))
	return conn, mux
}

func TestWebhookTokenGRPC(t *testing.T) {
	conn, _ := webhookServer(t, testWebhookToken)
	c := pb.NewWebhookClient(conn)
	call := func(ctx context.Context) codes.Code {
		_, err := c.S3Notifications(ctx, &pb.S3NotificationRequest{})
		return codeOf(err)
	}
	require.Equal(t, codes.Unauthenticated, call(context.Background()))
	require.Equal(t, codes.Unauthenticated, call(metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer nope")))
	require.Equal(t, codes.Unauthenticated, call(metadata.AppendToOutgoingContext(context.Background(), "x-tutti-webhook-token", "nope")))
	// Unimplemented means the interceptor let the call through.
	require.Equal(t, codes.Unimplemented, call(metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+testWebhookToken)))
	require.Equal(t, codes.Unimplemented, call(metadata.AppendToOutgoingContext(context.Background(), "x-tutti-webhook-token", testWebhookToken)))
	// other services are not affected by the webhook token
	_, err := pb.NewChorusClient(conn).GetStorages(context.Background(), nil)
	require.NotEqual(t, codes.Unauthenticated, codeOf(err))
}

func TestWebhookTokenUnsetIsNoop(t *testing.T) {
	conn, _ := webhookServer(t, "")
	_, err := pb.NewWebhookClient(conn).S3Notifications(context.Background(), &pb.S3NotificationRequest{})
	require.Equal(t, codes.Unimplemented, codeOf(err))
}

func TestWebhookTokenHTTPGateway(t *testing.T) {
	_, h := webhookServer(t, testWebhookToken)
	post := func(hdr map[string]string) int {
		req := httptest.NewRequest(http.MethodPost, "/webhook/s1/s3-notifications", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	require.Equal(t, http.StatusUnauthorized, post(nil))
	require.Equal(t, http.StatusUnauthorized, post(map[string]string{"Authorization": "Bearer wrong"}))
	require.Equal(t, http.StatusUnauthorized, post(map[string]string{WebhookTokenHeader: "wrong"}))
	// 501 (Unimplemented stub) means the request passed the token check.
	require.Equal(t, http.StatusNotImplemented, post(map[string]string{"Authorization": "Bearer " + testWebhookToken}))
	require.Equal(t, http.StatusNotImplemented, post(map[string]string{WebhookTokenHeader: testWebhookToken}))
}

func TestMeReportsWarnings(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	svc, err := NewService(Config{}, rdb, WithWarnings(WarningWebhookUnauthenticated))
	require.NoError(t, err)
	mux := runtime.NewServeMux()
	require.NoError(t, svc.RegisterHTTP(mux))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/auth/me", nil))
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), `"warnings":["webhook_unauthenticated"]`)
}

func codeOf(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	st, _ := status.FromError(err)
	return st.Code()
}

func webhookServerOpts(t *testing.T, token string, opts ...WebhookOption) *grpc.ClientConn {
	t.Helper()
	u, s := WebhookTokenInterceptors(token, opts...)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(u), grpc.ChainStreamInterceptor(s))
	pb.RegisterWebhookServer(srv, pb.UnimplementedWebhookServer{})
	pb.RegisterChorusServer(srv, pb.UnimplementedChorusServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestWebhookRequireToken(t *testing.T) {
	ctx := context.Background()
	bearer := func(tok string) context.Context {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok)
	}
	call := func(c context.Context, conn *grpc.ClientConn) codes.Code {
		_, err := pb.NewWebhookClient(conn).S3Notifications(c, &pb.S3NotificationRequest{})
		return codeOf(err)
	}

	t.Run("no token configured fails closed", func(t *testing.T) {
		conn := webhookServerOpts(t, "", WebhookRequireToken())
		require.Equal(t, codes.Unauthenticated, call(ctx, conn))
		require.Equal(t, codes.Unauthenticated, call(bearer("anything"), conn))
		// non-webhook services are unaffected
		_, err := pb.NewChorusClient(conn).GetStorages(ctx, nil)
		require.NotEqual(t, codes.Unauthenticated, codeOf(err))
	})

	t.Run("token configured behaves normally", func(t *testing.T) {
		conn := webhookServerOpts(t, testWebhookToken, WebhookRequireToken())
		require.Equal(t, codes.Unauthenticated, call(ctx, conn))
		require.Equal(t, codes.Unauthenticated, call(bearer("nope"), conn))
		require.Equal(t, codes.Unimplemented, call(bearer(testWebhookToken), conn))
	})

	t.Run("without option empty token is a no-op", func(t *testing.T) {
		conn := webhookServerOpts(t, "")
		require.Equal(t, codes.Unimplemented, call(ctx, conn))
	})

	t.Run("stream interceptor fails closed", func(t *testing.T) {
		_, s := WebhookTokenInterceptors("", WebhookRequireToken())
		called := false
		h := func(any, grpc.ServerStream) error { called = true; return nil }
		err := s(nil, fakeCtxStream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: "/chorus.Webhook/S3Notifications"}, h)
		require.Equal(t, codes.Unauthenticated, codeOf(err))
		require.False(t, called)
	})
}

type fakeCtxStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f fakeCtxStream) Context() context.Context { return f.ctx }
