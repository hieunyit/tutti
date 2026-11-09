/*
 * Copyright © 2026 Clyso GmbH
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

// Package access enforces per-account access policies (pkg/accesspolicy) in the proxy.
package access

import (
	"bytes"
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	mclient "github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/trafficstats"
	"github.com/hieunyit/tutti/pkg/util"
)

// LookupFunc returns the policy of a user, if any.
type LookupFunc func(ctx context.Context, user string) (accesspolicy.Policy, bool)

// Middleware must run after authentication and request parsing (user, method and bucket are read from the context).
func Middleware(lookup LookupFunc) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			user := xctx.GetUser(ctx)
			pol, ok := lookup(ctx, user)
			if !ok {
				zerolog.Ctx(ctx).Debug().Str("user", user).Msg("access policy: none, full access")
				next.ServeHTTP(w, r)
				return
			}
			req := accesspolicy.Request{Method: xctx.GetMethod(ctx), Bucket: xctx.GetBucket(ctx)}
			if req.Method == s3.CopyObject || req.Method == s3.UploadPartCopy {
				req.CopySourceBucket, req.CopySourceInvalid = copySourceBucket(r.Header.Get(s3.AmzCopySource))
			}
			if err := pol.Authorize(req); err != nil {
				trafficstats.Default.AccessDenied()
				zerolog.Ctx(ctx).Info().Err(err).Str("user", user).Str("method", req.Method.String()).Msg("access policy: denied")
				util.WriteError(ctx, w, mclient.ErrorResponse{
					Code:       "AccessDenied",
					Message:    "Access Denied.",
					BucketName: req.Bucket,
					Key:        xctx.GetObject(ctx),
					StatusCode: http.StatusForbidden,
				})
				return
			}
			if req.Method == s3.ListBuckets && !pol.AllBuckets() {
				fw := &filterWriter{ResponseWriter: w, pol: pol, status: http.StatusOK}
				next.ServeHTTP(fw, r)
				fw.finish(ctx)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// copySourceBucket extracts the bucket from x-amz-copy-source ("/bucket/key?versionId=x", url-encoded, leading slash optional).
func copySourceBucket(v string) (bucket string, invalid bool) {
	if v == "" {
		return "", true
	}
	if i := strings.IndexByte(v, '?'); i >= 0 {
		v = v[:i]
	}
	dec, err := url.PathUnescape(v)
	if err != nil {
		return "", true
	}
	dec = strings.TrimPrefix(dec, "/")
	b, _, found := strings.Cut(dec, "/")
	if !found || b == "" {
		return "", true
	}
	return b, false
}

// filterWriter buffers a ListBuckets response and removes buckets that the policy does not allow.
type filterWriter struct {
	http.ResponseWriter
	pol    accesspolicy.Policy
	status int
	buf    bytes.Buffer
}

func (f *filterWriter) WriteHeader(code int) { f.status = code }

func (f *filterWriter) Write(b []byte) (int, error) { return f.buf.Write(b) }

type bucketEntry struct {
	Name         string `xml:"Name"`
	CreationDate string `xml:"CreationDate,omitempty"`
}

type listBucketsResult struct {
	XMLName xml.Name `xml:"ListAllMyBucketsResult"`
	Xmlns   string   `xml:"xmlns,attr,omitempty"`
	Owner   *struct {
		ID          string `xml:"ID"`
		DisplayName string `xml:"DisplayName,omitempty"`
	} `xml:"Owner"`
	Buckets struct {
		Bucket []bucketEntry `xml:"Bucket"`
	} `xml:"Buckets"`
}

func (f *filterWriter) finish(ctx context.Context) {
	body := f.buf.Bytes()
	if f.status == http.StatusOK {
		var res listBucketsResult
		if err := xml.Unmarshal(body, &res); err != nil {
			// cannot filter: never leak the unfiltered list
			zerolog.Ctx(ctx).Error().Err(err).Msg("access policy: unable to parse ListBuckets response")
			f.status = http.StatusInternalServerError
			body = nil
		} else {
			kept := res.Buckets.Bucket[:0]
			for _, b := range res.Buckets.Bucket {
				if f.pol.BucketAllowed(b.Name) {
					kept = append(kept, b)
				}
			}
			res.Buckets.Bucket = kept
			if out, err := xml.Marshal(res); err == nil {
				body = append([]byte(xml.Header), out...)
			} else {
				f.status = http.StatusInternalServerError
				body = nil
			}
		}
	}
	h := f.ResponseWriter.Header()
	h.Del("Content-Encoding")
	h.Del("Etag")
	h.Set("Content-Length", strconv.Itoa(len(body)))
	f.ResponseWriter.WriteHeader(f.status)
	_, _ = f.ResponseWriter.Write(body)
}
