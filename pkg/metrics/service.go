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

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
)

var countRequests = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "storage_requests_total",
		Help: "Number of api calls to storage.",
	},
	[]string{"flow", "storage", "method"},
)

var bytesUpload = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "storage_bucket_bytes_upload",
		Help: "Number of bytes uploaded to storage.",
	},
	[]string{"flow", "storage", "bucket"},
)

var bytesDownload = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "storage_bucket_bytes_download",
		Help: "Number of bytes downloaded from storage.",
	},
	[]string{"flow", "storage", "bucket"},
)

var copyInProgressBytes = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "copy_in_progress_obj_bytes",
		Help: "Size of files currently copied by worker.",
	},
	[]string{"flow", "user", "bucket"},
)

var storageAvailable = promauto.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "storage_available",
		Help: "1 if the storage circuit breaker is closed, 0 if requests to the storage fail fast.",
	},
	[]string{"storage"},
)

var breakerTransitions = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "storage_breaker_transitions_total",
		Help: "Number of storage circuit breaker transitions.",
	},
	[]string{"storage", "to"},
)

var failovers = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "failover_total",
		Help: "Number of automatic routing failovers.",
	},
	[]string{"from", "to"},
)

// CountFailover counts one automatic failover of a routing scope from one storage to another.
func CountFailover(from, to string) {
	failovers.WithLabelValues(from, to).Inc()
}

// SetStorageAvailable sets the storage_available gauge.
func SetStorageAvailable(storage string, available bool) {
	v := 0.0
	if available {
		v = 1
	}
	storageAvailable.WithLabelValues(storage).Set(v)
}

// CountBreakerTransition counts a circuit breaker transition to "open" or "closed".
func CountBreakerTransition(storage string, available bool) {
	to := "open"
	if available {
		to = "closed"
	}
	breakerTransitions.WithLabelValues(storage, to).Inc()
}

type Service interface {
	Count(flow xctx.Flow, storage string, method string)
	Upload(flow xctx.Flow, storage, bucket string, bytes int)
	Download(flow xctx.Flow, storage, bucket string, bytes int)
}

type WorkerService interface {
	WorkerInProgressBytesInc(ctx context.Context, bytes int64)
	WorkerInProgressBytesDec(ctx context.Context, bytes int64)
}

func NewS3Service(enabled bool) *svc {
	return &svc{enabled: enabled}
}

var _ Service = &svc{}
var _ WorkerService = &svc{}

type svc struct {
	enabled bool
}

func (s svc) WorkerInProgressBytesInc(ctx context.Context, bytes int64) {
	if !s.enabled {
		return
	}
	copyInProgressBytes.WithLabelValues(
		string(xctx.GetFlow(ctx)),
		xctx.GetUser(ctx),
		xctx.GetBucket(ctx),
	).Add(float64(bytes))
}

func (s svc) WorkerInProgressBytesDec(ctx context.Context, bytes int64) {
	if !s.enabled {
		return
	}
	copyInProgressBytes.WithLabelValues(
		string(xctx.GetFlow(ctx)),
		xctx.GetUser(ctx),
		xctx.GetBucket(ctx),
	).Sub(float64(bytes))
}

func (s svc) Count(flow xctx.Flow, storage string, method string) {
	if !s.enabled {
		return
	}
	countRequests.WithLabelValues(string(flow), storage, method).Inc()
}

func (s svc) Upload(flow xctx.Flow, storage, bucket string, bytes int) {
	if !s.enabled || bytes <= 0 {
		return
	}
	bytesUpload.WithLabelValues(string(flow), storage, bucket).Add(float64(bytes))
}

func (s svc) Download(flow xctx.Flow, storage, bucket string, bytes int) {
	if !s.enabled || bytes <= 0 {
		return
	}
	bytesDownload.WithLabelValues(string(flow), storage, bucket).Add(float64(bytes))
}
