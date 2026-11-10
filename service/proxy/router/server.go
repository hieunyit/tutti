/*
 * Copyright © 2023 Clyso GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package router

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/replication"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/trafficstats"
	"github.com/hieunyit/tutti/pkg/util"
)

type Config struct {
	Storages          map[dom.StorageType]StorageProxy
	LogMiddleware     func(next http.Handler) http.Handler
	TraceMiddleware   func(next http.Handler) http.Handler
	MetricsMiddleware func(next http.Handler) http.Handler
	PolicyMiddleware  func(next http.Handler) http.Handler
	// AccessMiddleware enforces per-account access policies. Optional, applied to S3 only
	// (after authentication and request parsing).
	AccessMiddleware func(next http.Handler) http.Handler
	// CorsMiddleware must be set only when CORS is enabled: it also makes the proxy drop backend CORS headers.
	CorsMiddleware func(next http.Handler) http.Handler
	// Replication defines how failures to record replication events are handled.
	Replication ReplicationOptions
	// Sync configures optional synchronous replication on the write path.
	Sync SyncOptions
}

const (
	// OnEventErrorFail responds with 503 if the replication event could not be recorded.
	OnEventErrorFail = "fail"
	// OnEventErrorLog only logs the error and forwards the storage response.
	OnEventErrorLog = "log"

	DefaultEventRetryBudget = 2 * time.Second
	eventRetryInitialDelay  = 50 * time.Millisecond
)

// ReplicationOptions controls handling of replication event recording errors.
type ReplicationOptions struct {
	// OnEventError is "fail" (default) or "log".
	OnEventError string
	// EventRetryBudget is the total time spent retrying Replicate. Default 2s.
	EventRetryBudget time.Duration
}

func (o ReplicationOptions) withDefaults() ReplicationOptions {
	if o.OnEventError == "" {
		o.OnEventError = OnEventErrorFail
	}
	if o.EventRetryBudget <= 0 {
		o.EventRetryBudget = DefaultEventRetryBudget
	}
	return o
}

// replicateWithRetry retries Replicate with exponential backoff until deadline.
// The deadline is shared by all tasks of a request, so a request producing many tasks
// (e.g. DeleteObjects with 1000 keys) is still bounded by a single retry budget.
func replicateWithRetry(ctx context.Context, svc replication.Service, storage string, task tasks.ReplicationTask, deadline time.Time) error {
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	delay := eventRetryInitialDelay
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := svc.Replicate(ctx, storage, task)
		if err == nil {
			return nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return err
		}
		if delay > remaining {
			delay = remaining
		}
		zerolog.Ctx(ctx).Warn().Err(err).Dur("retry_in", delay).Msg("unable to record replication event, retrying")
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("%w (retry aborted: %w)", err, ctx.Err())
		case <-timer.C:
		}
		delay *= 2
	}
}

// opClass maps the S3 method of the request to a trafficstats operation class.
func opClass(ctx context.Context) string {
	switch accesspolicy.Classify(xctx.GetMethod(ctx)) {
	case accesspolicy.ClassRead:
		return trafficstats.ClassRead
	case accesspolicy.ClassWrite:
		return trafficstats.ClassWrite
	case accesspolicy.ClassExists:
		return trafficstats.ClassExists
	case accesspolicy.ClassManage:
		return trafficstats.ClassManage
	}
	return trafficstats.ClassOther
}

func writeEventRecordError(ctx context.Context, w http.ResponseWriter) {
	trafficstats.Default.EventRecordFailed()
	util.WriteError(ctx, w, minio.ErrorResponse{
		Code:       "ServiceUnavailable",
		Message:    "The change was applied but could not be recorded for replication. Please retry the request.",
		BucketName: xctx.GetBucket(ctx),
		Key:        xctx.GetObject(ctx),
		StatusCode: http.StatusServiceUnavailable,
	})
}

type StorageProxy struct {
	Router             Router
	Replicator         replication.Service
	AuthMiddleware     func(next http.Handler) http.Handler
	ReqParseMiddleware func(next http.Handler) http.Handler
}

func (c *Config) Validate() error {
	if len(c.Storages) == 0 {
		return fmt.Errorf("%w: at least one storage proxy must be configured", dom.ErrInternal)
	}
	for storType, storProxy := range c.Storages {
		if storProxy.Router == nil {
			return fmt.Errorf("%w: router is not configured for storage type %s", dom.ErrInternal, storType)
		}
		if storProxy.Replicator == nil {
			return fmt.Errorf("%w: replicator service is not configured for storage type %s", dom.ErrInternal, storType)
		}
		if storProxy.ReqParseMiddleware == nil {
			return fmt.Errorf("%w: request parse middleware is not configured for storage type %s", dom.ErrInternal, storType)
		}
	}
	if c.LogMiddleware == nil {
		return fmt.Errorf("%w: log middleware is not configured", dom.ErrInternal)
	}
	if c.PolicyMiddleware == nil {
		return fmt.Errorf("%w: policy middleware is not configured", dom.ErrInternal)
	}
	return nil
}

func New(config Config) (http.Handler, error) {
	// build proxy handlers. Middleware wraps request handler, so it is applied in reverse order
	if err := config.Validate(); err != nil {
		return nil, err
	}
	handlers := make(map[dom.StorageType]http.Handler, len(config.Storages))
	for storType, storProxy := range config.Storages {
		// 9. main request handler. Forward request to storage backend and emit replication tasks
		handler := serve(storProxy.Router, storProxy.Replicator, config.CorsMiddleware != nil, config.Replication, config.Sync)
		if config.TraceMiddleware != nil {
			// 8. tracing
			handler = config.TraceMiddleware(handler)
		}
		if config.MetricsMiddleware != nil {
			// 7. metrics collection
			handler = config.MetricsMiddleware(handler)
		}
		if storType == dom.S3 && config.AccessMiddleware != nil {
			// 6.5. per-account access policy (user from auth, method/bucket from request parse, routed storage from
			// the routing policy: a user on a shared connection account is judged on the storage it is routed to)
			handler = config.AccessMiddleware(handler)
		}
		// 6. set routing & replication policies to context
		handler = config.PolicyMiddleware(handler)
		// 5. parse storage-specific request method/user/bucket/object and set to context
		handler = storProxy.ReqParseMiddleware(handler)
		if storProxy.AuthMiddleware != nil {
			// 4. storage-specific auth check
			handler = storProxy.AuthMiddleware(handler)
		}
		handlers[storType] = handler
	}
	var result http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 3. detect storage type from request
		storType := objstore.StorageTypeFromRequest(r)
		ctx := log.WithStorType(r.Context(), storType)
		handler, ok := handlers[storType]
		if !ok {
			util.WriteError(ctx, w, fmt.Errorf("%w: proxy handler is not configured for storage type %s", dom.ErrInternal, storType))
			return
		}
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
	if config.CorsMiddleware != nil {
		// 2. CORS: answer preflight requests before auth and set CORS headers for actual requests
		result = config.CorsMiddleware(result)
	}

	// 1. init request scoped logger
	result = config.LogMiddleware(result)
	// 0. built-in traffic statistics: outermost, so that every response, byte and the full duration is counted
	result = trafficstats.Default.Middleware(result)
	return result, nil

}

func serve(router Router, replSvc replication.Service, proxyCors bool, replOpts ReplicationOptions, syncOpts SyncOptions) http.Handler {
	replOpts = replOpts.withDefaults()
	// Use a custom handler function instead of ServeMux to avoid automatic redirects
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer("").Start(r.Context(), "Route")
		defer span.End()
		r = r.WithContext(ctx)
		logger := zerolog.Ctx(r.Context())
		logger.Debug().Msg("proxy: new request received")
		trafficstats.SetRoute(ctx, opClass(ctx))

		resp, taskList, storage, isApiErr, err := router.Route(r)
		if err != nil {
			util.WriteError(r.Context(), w, err)
			return
		}
		defer func() {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
		}()
		ctx = log.WithStorage(ctx, storage)
		trafficstats.SetStorage(ctx, storage)
		var syncRes syncResult
		var synced bool
		// TODO: is it reachable? This branch is active only if err == nil
		if isApiErr {
			zerolog.Ctx(ctx).Info().Err(err).Msg("s3 api error returned")
			// create replication tasks according to replication rules
		} else {
			replCtx, cancel := log.StartNew(ctx)
			defer cancel()
			// detached ctx keeps values but ignores client cancel; stop retrying when client is gone
			retryCtx, stopRetry := withCancelFrom(replCtx, r.Context())
			defer stopRetry()
			deadline := time.Now().Add(replOpts.EventRetryBudget)
			var recordErr error
			for _, task := range taskList {
				replErr := replicateWithRetry(retryCtx, replSvc, storage, task, deadline)
				if replErr != nil {
					logger.Err(replErr).
						Str("bucket", xctx.GetBucket(ctx)).
						Str("object", xctx.GetObject(ctx)).
						Str("on_event_error", replOpts.OnEventError).
						Msg("unable to handle replication")
					recordErr = replErr
					if retryCtx.Err() != nil || !time.Now().Before(deadline) {
						break
					}
				}
			}
			if recordErr != nil && replOpts.OnEventError == OnEventErrorFail {
				// resp body is closed by defer
				writeEventRecordError(ctx, w)
				return
			}
			if recordErr == nil && syncOpts.active() {
				syncRes, synced = syncOpts.syncDestinations(replCtx, taskList)
				switch {
				case !synced:
					trafficstats.Default.SyncOutcome(trafficstats.SyncSkipped)
				case syncRes.copies < syncRes.quorum:
					trafficstats.Default.SyncOutcome(trafficstats.SyncBelowQuorum)
				default:
					trafficstats.Default.SyncOutcome(trafficstats.SyncQuorum)
				}
				if synced && syncRes.copies < syncRes.quorum {
					onFail := syncOpts.OnQuorumFailure
					if onFail == "" {
						onFail = OnQuorumFailureFail
					}
					syncRes.logUnconfirmed(logger, onFail)
					if onFail == OnQuorumFailureFail {
						writeQuorumError(ctx, w)
						return
					}
				} else if synced && (len(syncRes.failed) != 0 || len(syncRes.pending) != 0) {
					syncRes.logUnconfirmed(logger, syncOpts.OnQuorumFailure)
				}
			}
		}
		// Forward response to original client
		copyResponseHeaders(w.Header(), resp.Header, proxyCors)
		if synced {
			w.Header().Set(ReplicasHeader, syncRes.header())
		}
		w.WriteHeader(resp.StatusCode)
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			logger.Err(err).Msg("unable to copy response body")
			// The status line is already sent: abort the response so that the client sees a broken
			// connection instead of a silently truncated successful response.
			panic(http.ErrAbortHandler)
		}
	})
}

// hopByHopHeaders are connection-scoped headers that a proxy must not forward.
var hopByHopHeaders = []string{
	"Connection", "Keep-Alive", "Proxy-Connection", "Proxy-Authenticate", "Proxy-Authorization",
	"Transfer-Encoding", "Upgrade", "Te", "Trailer",
}

// copyResponseHeaders copies all values of the backend response headers to dst, except hop-by-hop
// headers (including the ones listed in Connection). With proxyCors the CORS headers set by the proxy
// CORS middleware are kept: backend Access-Control-* are dropped and backend Vary values are added.
func copyResponseHeaders(dst, src http.Header, proxyCors bool) {
	skip := make(map[string]struct{}, len(hopByHopHeaders))
	for _, h := range hopByHopHeaders {
		skip[http.CanonicalHeaderKey(h)] = struct{}{}
	}
	for _, v := range src["Connection"] {
		for _, tok := range strings.Split(v, ",") {
			if tok = strings.TrimSpace(tok); tok != "" {
				skip[http.CanonicalHeaderKey(tok)] = struct{}{}
			}
		}
	}
	for k, v := range src {
		k = http.CanonicalHeaderKey(k)
		if _, ok := skip[k]; ok {
			continue
		}
		if proxyCors {
			if strings.HasPrefix(k, "Access-Control-") {
				continue
			}
			if k == "Vary" {
				for _, val := range v {
					dst.Add(k, val)
				}
				continue
			}
		}
		dst[k] = append([]string(nil), v...)
	}
}

// withCancelFrom returns a ctx with the values of base that is also canceled when
// cancelSrc is done. The returned func releases it and must be called.
func withCancelFrom(base, cancelSrc context.Context) (context.Context, func()) {
	ctx, cancel := context.WithCancel(base)
	stop := context.AfterFunc(cancelSrc, cancel)
	return ctx, func() {
		stop()
		cancel()
	}
}
