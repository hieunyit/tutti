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

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/replication"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/hieunyit/tutti/pkg/util"
)

type Config struct {
	Storages          map[dom.StorageType]StorageProxy
	LogMiddleware     func(next http.Handler) http.Handler
	TraceMiddleware   func(next http.Handler) http.Handler
	MetricsMiddleware func(next http.Handler) http.Handler
	PolicyMiddleware  func(next http.Handler) http.Handler
	// CorsMiddleware must be set only when CORS is enabled: it also makes the proxy drop backend CORS headers.
	CorsMiddleware func(next http.Handler) http.Handler
	// Replication defines how failures to record replication events are handled.
	Replication ReplicationOptions
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
	delay := eventRetryInitialDelay
	for {
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

func writeEventRecordError(ctx context.Context, w http.ResponseWriter) {
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
		handler := serve(storProxy.Router, storProxy.Replicator, config.CorsMiddleware != nil, config.Replication)
		if config.TraceMiddleware != nil {
			// 8. tracing
			handler = config.TraceMiddleware(handler)
		}
		if config.MetricsMiddleware != nil {
			// 7. metrics collection
			handler = config.MetricsMiddleware(handler)
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
	return result, nil

}

func serve(router Router, replSvc replication.Service, proxyCors bool, replOpts ReplicationOptions) http.Handler {
	replOpts = replOpts.withDefaults()
	// Use a custom handler function instead of ServeMux to avoid automatic redirects
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer("").Start(r.Context(), "Route")
		defer span.End()
		r = r.WithContext(ctx)
		logger := zerolog.Ctx(r.Context())
		logger.Debug().Msg("proxy: new request received")

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
				}
			}
			if recordErr != nil && replOpts.OnEventError == OnEventErrorFail {
				// resp body is closed by defer
				writeEventRecordError(ctx, w)
				return
			}
		}
		// Forward response to original client
		for k, v := range resp.Header {
			if proxyCors {
				// keep CORS headers set by proxy CORS middleware
				if strings.HasPrefix(k, "Access-Control-") {
					continue
				}
				if k == "Vary" {
					// keep Vary set by proxy CORS middleware
					for _, val := range v {
						w.Header().Add(k, val)
					}
					continue
				}
			}
			w.Header().Set(k, v[0])
		}
		w.WriteHeader(resp.StatusCode)
		_, err = io.Copy(w, resp.Body)
		if err != nil {
			logger.Err(err).Msg("unable to copy response body")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
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
