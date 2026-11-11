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

package api

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/cors"
	"github.com/tmc/grpc-websocket-proxy/wsproxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hieunyit/tutti/pkg/auth"
)

type RegisterHandlersFunc func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error

func GRPCGateway(ctx context.Context, conf *Config, register RegisterHandlersFunc) (start func(context.Context) error, stop func(context.Context) error, err error) {
	mux := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(auth.GatewayHeaderMatcher),
		runtime.WithMarshalerOption(runtime.MIMEWildcard, &runtime.JSONPb{
			MarshalOptions:   protojson.MarshalOptions{EmitUnpopulated: true},
			UnmarshalOptions: protojson.UnmarshalOptions{DiscardUnknown: true},
		}),
	)
	var opts []grpc.DialOption

	if conf.Secure {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: false})))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	serverAddress := ":" + strconv.Itoa(conf.GrpcPort)

	// Register all handlers
	err = register(ctx, mux, serverAddress, opts)
	if err != nil {
		return nil, nil, err
	}

	withCors := cors.New(cors.Options{
		AllowOriginFunc:  func(origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"ACCEPT", "Authorization", "Content-Type", "X-CSRF-Token", auth.WebhookTokenHeader},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}).Handler(mux)

	handler := wsproxy.WebsocketProxy(withCors)
	srv := &http.Server{
		Addr:              fmt.Sprintf("0.0.0.0:%d", conf.HttpPort),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	srv.Handler = ConcurrencyLimiter(handler, conf.MaxConcurrentRequests)

	start = func(_ context.Context) error {
		return srv.ListenAndServe()
	}
	stop = func(ctx context.Context) error {
		if err := srv.Shutdown(ctx); err != nil {
			_ = srv.Close()
			return err
		}
		return nil
	}
	return
}

const defaultMaxConcurrentRequests = 100

// ConcurrencyLimiter wraps an http.Handler with an in-flight concurrency limiter.
// If maxConcurrent is <= 0, it defaults to defaultMaxConcurrentRequests (100).
// When saturated, it immediately returns HTTP 503 (Service Unavailable) with Retry-After: 1.
func ConcurrencyLimiter(h http.Handler, maxConcurrent int) http.Handler {
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrentRequests
	}
	sem := make(chan struct{}, maxConcurrent)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			h.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "server busy: too many concurrent requests", http.StatusServiceUnavailable)
		}
	})
}
