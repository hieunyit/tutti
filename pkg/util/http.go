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

package util

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"net/http"
	"strconv"

	mclient "github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/s3/chunked"
	"github.com/hieunyit/tutti/pkg/trafficstats"
)

func WriteError(ctx context.Context, w http.ResponseWriter, err error) {
	s3Err := mclient.ErrorResponse{}
	logLevel := zerolog.ErrorLevel

	var chunkedErr *chunked.Error
	var rateLimitErr *dom.ErrRateLimitExceeded
	switch {
	case errors.As(err, &rateLimitErr):
		logLevel = zerolog.WarnLevel
		retrySeconds := int(rateLimitErr.RetryIn.Seconds())
		if retrySeconds < 1 {
			retrySeconds = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
		s3Err = mclient.ErrorResponse{
			Code:       "SlowDown",
			Message:    "Please reduce your request rate.",
			BucketName: xctx.GetBucket(ctx),
			Key:        xctx.GetObject(ctx),
			StatusCode: http.StatusServiceUnavailable,
		}
	case errors.As(err, &s3Err):
		logLevel = zerolog.InfoLevel
	case errors.As(err, &chunkedErr):
		// aws-chunked payload decoding / verification failure
		logLevel = zerolog.WarnLevel
		s3Err = mclient.ErrorResponse{
			XMLName:    xml.Name{},
			Code:       chunkedErr.Code,
			Message:    chunkedErr.Message,
			BucketName: xctx.GetBucket(ctx),
			Key:        xctx.GetObject(ctx),
			StatusCode: chunkedErr.StatusCode,
		}
	case errors.Is(err, dom.ErrAuth):
		logLevel = zerolog.WarnLevel
		s3Err = mclient.ErrorResponse{
			XMLName:    xml.Name{},
			Code:       "AccessDenied",
			Message:    err.Error(),
			StatusCode: http.StatusForbidden,
		}
	case errors.Is(err, dom.ErrNotImplemented):
		s3Err = mclient.ErrorResponse{
			Code:       "NotImplemented",
			Message:    err.Error(),
			BucketName: xctx.GetBucket(ctx),
			Key:        xctx.GetObject(ctx),
			StatusCode: http.StatusNotImplemented,
		}
	case errors.Is(err, dom.ErrInvalidArg):
		s3Err = mclient.ErrorResponse{
			Code:       "InvalidArgument",
			Message:    err.Error(),
			BucketName: xctx.GetBucket(ctx),
			Key:        xctx.GetObject(ctx),
			StatusCode: http.StatusBadRequest,
		}
	case errors.Is(err, dom.ErrUnavailable):
		s3Err = mclient.ErrorResponse{
			Code:       "ServiceUnavailable",
			Message:    "Service temporarily unavailable. Please retry.",
			BucketName: xctx.GetBucket(ctx),
			Key:        xctx.GetObject(ctx),
			StatusCode: http.StatusServiceUnavailable,
		}
	case errors.Is(err, dom.ErrRoutingBlock):
		logLevel = zerolog.WarnLevel
		s3Err = mclient.ErrorResponse{
			Code:       "NoSuchBucket",
			Message:    "Bucket already used as replication destination",
			BucketName: xctx.GetBucket(ctx),
			Key:        xctx.GetObject(ctx),
			StatusCode: http.StatusNotFound,
		}
	case errors.Is(err, dom.ErrNoCredentials):
		logLevel = zerolog.WarnLevel
		trafficstats.Default.NoCredentials()
		var named interface{ StorageName() string }
		storage := xctx.GetStorage(ctx)
		if errors.As(err, &named) {
			storage = named.StorageName()
		}
		metrics.CountNoCredentials(storage)
		s3Err = mclient.ErrorResponse{
			Code:       "ServiceUnavailable",
			Message:    err.Error(),
			BucketName: xctx.GetBucket(ctx),
			Key:        xctx.GetObject(ctx),
			StatusCode: http.StatusServiceUnavailable,
		}
	case errors.Is(err, dom.ErrPolicy):
		s3Err = mclient.ErrorResponse{
			Code:       "InternalErrors",
			Message:    err.Error(),
			BucketName: xctx.GetBucket(ctx),
			Key:        xctx.GetObject(ctx),
			StatusCode: http.StatusInternalServerError,
		}
	default:
		s3Err = mclient.ErrorResponse{
			XMLName:    xml.Name{},
			Code:       "InternalError",
			Message:    "An internal error occurred.",
			StatusCode: http.StatusInternalServerError,
		}
	}
	zerolog.Ctx(ctx).WithLevel(logLevel).Err(err).Msg("error returned")
	w.WriteHeader(s3Err.StatusCode)

	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	e := xml.NewEncoder(&buf).Encode(&s3Err)
	if e != nil {
		zerolog.Ctx(ctx).Err(e).Msg("unable to marshall err resp")
		return
	}
	_, e = w.Write(buf.Bytes())
	if e != nil {
		zerolog.Ctx(ctx).Err(e).Msg("unable to write err resp")
		return
	}
}
