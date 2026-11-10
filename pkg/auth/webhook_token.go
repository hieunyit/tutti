package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"net/textproto"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// WebhookTokenHeader is the HTTP header (and gRPC metadata key, lower-cased) that may carry the webhook token as an
// alternative to "Authorization: Bearer <token>".
const WebhookTokenHeader = "X-Tutti-Webhook-Token"

const webhookMethodPrefix = "/chorus.Webhook/"

// WarningWebhookUnauthenticated is reported by GET /auth/me when webhooks are served on the main API port without
// api.webhook.token.
const WarningWebhookUnauthenticated = "webhook_unauthenticated"

// GatewayHeaderMatcher forwards X-Tutti-Webhook-Token (dropped by the default matcher) to gRPC metadata, in addition
// to the default set (which already forwards Authorization).
func GatewayHeaderMatcher(key string) (string, bool) {
	if textproto.CanonicalMIMEHeaderKey(key) == WebhookTokenHeader {
		return strings.ToLower(WebhookTokenHeader), true
	}
	return runtime.DefaultHeaderMatcher(key)
}

func webhookTokenFromMD(ctx context.Context) []string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}
	var out []string
	for _, v := range md.Get(strings.ToLower(WebhookTokenHeader)) {
		out = append(out, strings.TrimSpace(v))
	}
	for _, v := range md.Get("authorization") {
		if len(v) > 7 && strings.EqualFold(v[:7], "bearer ") {
			out = append(out, strings.TrimSpace(v[7:]))
		}
	}
	return out
}

func tokenEqual(a, b string) bool {
	ha, hb := sha256.Sum256([]byte(a)), sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(ha[:], hb[:]) == 1
}

// CheckWebhookToken reports whether the incoming call carries the expected webhook token.
func CheckWebhookToken(ctx context.Context, token string) bool {
	ok := false
	for _, c := range webhookTokenFromMD(ctx) {
		if tokenEqual(c, token) {
			ok = true
		}
	}
	return ok
}

func checkWebhook(ctx context.Context, method, token string) error {
	if token == "" || !strings.HasPrefix(method, webhookMethodPrefix) {
		return nil
	}
	if !CheckWebhookToken(ctx, token) {
		return status.Error(codes.Unauthenticated, "webhook token required: send Authorization: Bearer <token> or "+WebhookTokenHeader)
	}
	return nil
}

// WebhookTokenInterceptors enforce token on /chorus.Webhook/* calls. With an empty token they are no-ops.
func WebhookTokenInterceptors(token string) (grpc.UnaryServerInterceptor, grpc.StreamServerInterceptor) {
	u := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		if err := checkWebhook(ctx, info.FullMethod, token); err != nil {
			return nil, err
		}
		return h(ctx, req)
	}
	s := func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, h grpc.StreamHandler) error {
		if err := checkWebhook(ss.Context(), info.FullMethod, token); err != nil {
			return err
		}
		return h(srv, ss)
	}
	return u, s
}
