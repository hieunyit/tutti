package auth

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hieunyit/tutti/pkg/controlplane"
	xctx "github.com/hieunyit/tutti/pkg/ctx"
)

type ctxKey struct{}

// ContextWithSession returns a context that carries the signed-in session.
func ContextWithSession(ctx context.Context, sess *Session) context.Context {
	return context.WithValue(ctx, ctxKey{}, sess)
}

// FromContext returns the authenticated session placed by the interceptor.
func FromContext(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(ctxKey{}).(*Session)
	return s, ok
}

// publicMethodPrefixes are reachable without a session.
//
//   - /chorus.Webhook/: called by storage systems (S3 bucket notifications,
//     swift access-log shippers) that cannot perform a login. These endpoints
//     are protected by network placement and the webhook port split
//     (api.webhook.grpcPort/httpPort) instead.
var publicMethodPrefixes = []string{
	"/chorus.Webhook/",
}

// publicMethods are individual RPCs reachable without a session.
var publicMethods = map[string]struct{}{
	// version info, equivalent of a public /version endpoint.
	"/chorus.Chorus/GetAppVersion": {},
}

// viewerMethods is the explicit allow-list of read-only RPCs a viewer may
// call. Any method not listed here (and not public) is admin-only, so newly
// added RPCs fail closed.
//
// GetProxyCredentials is deliberately absent: it returns secret keys.
var viewerMethods = map[string]struct{}{
	"/chorus.ProtectionGroups/List":               {},
	"/chorus.ProtectionGroups/Get":                {},
	"/chorus.ProtectionGroups/GetAutomaticFreeze": {},
	"/chorus.ProtectionGroups/ListWriters":        {},
	"/chorus.Resilience/ListRuntimeStorages":      {},
	"/chorus.Chorus/GetStorages":                  {},
	"/chorus.Diff/List":                           {},
	"/chorus.Diff/GetReport":                      {},
	"/chorus.Diff/GetReportEntries":               {},
	"/chorus.Policy/AvailableBuckets":             {},
	"/chorus.Policy/GetReplication":               {},
	"/chorus.Policy/ListReplications":             {},
	"/chorus.Policy/StreamReplication":            {},
	"/chorus.Policy/GetSwitchStatus":              {},
	"/chorus.Policy/ListRoutings":                 {},
	"/chorus.Resilience/GetProxyStatus":           {},
	"/chorus.Resilience/GetStatsHistory":          {},
	"/chorus.Resilience/ListFailoverPolicies":     {},
	"/chorus.Resilience/ListFailoverEvents":       {},
	"/chorus.Resilience/ListAccessPolicies":       {},
	"/chorus.Resilience/ListProxyAccounts":        {},
	"/chorus.Resilience/GetProxyFleetConfig":      {},
	"/chorus.Resilience/ListAuditLog":             {},
	// ListS3Users returns metadata only (no secrets). Console user RPCs and all mutations are admin-only.
	"/chorus.Resilience/ListS3Users": {},
}

func isPublic(method string) bool {
	if _, ok := publicMethods[method]; ok {
		return true
	}
	for _, p := range publicMethodPrefixes {
		if strings.HasPrefix(method, p) {
			return true
		}
	}
	return false
}

// Allowed reports whether role may call the gRPC full method.
func Allowed(role, method string) bool {
	if isPublic(method) {
		return true
	}
	switch role {
	case RoleAdmin:
		return true
	case RoleViewer:
		_, ok := viewerMethods[method]
		return ok
	}
	return false
}

// Stable machine-readable markers of the "password change required" rejection.
const (
	PasswordChangeRequiredMessage = "password change required"
	PasswordChangeRequiredReason  = "PASSWORD_CHANGE_REQUIRED"
	PasswordChangeRequiredDomain  = "tutti.auth"
)

// PasswordChangeRequiredError is the PermissionDenied status returned to users that must change their password first.
// Through grpc-gateway it renders as HTTP 403 {"code":7,"message":"password change required","details":[{"@type":
// ".../google.rpc.ErrorInfo","reason":"PASSWORD_CHANGE_REQUIRED","domain":"tutti.auth"}]}.
func PasswordChangeRequiredError() error {
	st := status.New(codes.PermissionDenied, PasswordChangeRequiredMessage)
	if d, err := st.WithDetails(&errdetails.ErrorInfo{Reason: PasswordChangeRequiredReason, Domain: PasswordChangeRequiredDomain}); err == nil {
		st = d
	}
	return st.Err()
}

func bearerFromMD(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	for _, v := range md.Get("authorization") {
		if len(v) > 7 && strings.EqualFold(v[:7], "bearer ") {
			return strings.TrimSpace(v[7:])
		}
	}
	return ""
}

func (s *Service) check(ctx context.Context, method string) (context.Context, error) {
	if !s.Enabled() || isPublic(method) {
		if cur := controlplane.ActorFrom(ctx); cur.Name == "system" {
			ctx = controlplane.WithActor(ctx, controlplane.Actor{Name: "api", RequestID: xctx.GetTraceID(ctx)})
		}
		return ctx, nil
	}
	sess, err := s.Authenticate(ctx, bearerFromMD(ctx))
	if err != nil {
		if errors.Is(err, ErrInvalidSession) {
			return ctx, status.Error(codes.Unauthenticated, "authentication required: missing, invalid or expired token")
		}
		return ctx, status.Error(codes.Unavailable, "unable to verify session")
	}
	if sess.MustChangePassword {
		// Only /auth/me, /auth/password and /auth/logout (plain HTTP handlers, not gRPC) are usable until the password is changed.
		return ctx, PasswordChangeRequiredError()
	}
	if !Allowed(sess.Role, method) {
		return ctx, status.Errorf(codes.PermissionDenied, "role %q is not allowed to call %s", sess.Role, method)
	}
	ctx = ContextWithSession(ctx, sess)
	ctx = controlplane.WithActor(ctx, controlplane.Actor{Name: sess.Username, RequestID: xctx.GetTraceID(ctx)})
	return ctx, nil
}

// UnaryInterceptor enforces authentication and roles on unary RPCs.
func (s *Service) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, err := s.check(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

// StreamInterceptor enforces authentication and roles on streaming RPCs.
func (s *Service) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, err := s.check(ss.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
	}
}
