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

package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/hieunyit/tutti/pkg/dom"
)

// Handler returns the metrics server mux (/metrics, /health, /ready, /version). It never serves pprof.
func Handler(ctx context.Context, version dom.AppInfo) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if ctx.Err() != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		jsonInfo, err := json.Marshal(version)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(jsonInfo)
	})

	// Profiling is intentionally NOT served here: use the opt-in debug.pprofAddr server.
	return mux
}

func Server(ctx context.Context, port int, version dom.AppInfo) (start func(context.Context) error, stop func(context.Context) error) {
	server := http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           Handler(ctx, version),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	start = func(_ context.Context) error {
		return server.ListenAndServe()
	}
	stop = server.Shutdown

	return
}
