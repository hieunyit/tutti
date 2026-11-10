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

package access

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/s3"
)

const listXML = `<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Owner><ID>o</ID><DisplayName>o</DisplayName></Owner><Buckets><Bucket><Name>photos-1</Name><CreationDate>2024-01-01T00:00:00.000Z</CreationDate></Bucket><Bucket><Name>secret</Name><CreationDate>2024-01-01T00:00:00.000Z</CreationDate></Bucket><Bucket><Name>logs</Name><CreationDate>2024-01-01T00:00:00.000Z</CreationDate></Bucket></Buckets></ListAllMyBucketsResult>`

func run(t *testing.T, pols map[string]accesspolicy.Policy, user string, m s3.Method, bucket string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	lookup := func(_ context.Context, u string) (accesspolicy.Policy, bool) { p, ok := pols[u]; return p, ok }
	h := Middleware(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m == s3.ListBuckets {
			w.Header().Set("Content-Type", "application/xml")
			w.Header().Set("Content-Length", "1")
			_, _ = w.Write([]byte(listXML))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	ctx := log.WithUser(req.Context(), user)
	ctx = log.WithMethod(ctx, m)
	ctx = log.WithBucket(ctx, bucket)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))
	return rec
}

func TestMiddleware(t *testing.T) {
	pols := map[string]accesspolicy.Policy{
		"ro":     {User: "ro", Permission: accesspolicy.ReadOnly, Buckets: []string{"*"}},
		"wo":     {User: "wo", Permission: accesspolicy.WriteOnly, Buckets: []string{"*"}},
		"scoped": {User: "scoped", Permission: accesspolicy.ReadWrite, Buckets: []string{"photos-*", "logs"}},
	}
	code := func(user string, m s3.Method, bucket string, hdr map[string]string) int {
		return run(t, pols, user, m, bucket, hdr).Code
	}
	require.Equal(t, 403, code("ro", s3.PutObject, "b", nil))
	require.Equal(t, 200, code("ro", s3.GetObject, "b", nil))
	require.Equal(t, 403, code("wo", s3.GetObject, "b", nil))
	require.Equal(t, 200, code("wo", s3.PutObject, "b", nil))
	require.Equal(t, 200, code("scoped", s3.GetObject, "photos-x", nil))
	require.Equal(t, 403, code("scoped", s3.GetObject, "secret", nil))
	require.Equal(t, 200, code("scoped", s3.CopyObject, "logs", map[string]string{s3.AmzCopySource: "/photos-1/a%20b.txt?versionId=3"}))
	require.Equal(t, 200, code("scoped", s3.CopyObject, "logs", map[string]string{s3.AmzCopySource: "photos-1/a"}))
	require.Equal(t, 403, code("scoped", s3.CopyObject, "logs", map[string]string{s3.AmzCopySource: "/secret/a"}))
	require.Equal(t, 403, code("scoped", s3.CopyObject, "logs", nil))
	// no policy: unrestricted, even unknown methods
	require.Equal(t, 200, code("free", s3.PutObject, "any", nil))
	require.Equal(t, 200, code("free", s3.UndefinedMethod, "any", nil))
	// restricted account + unknown method
	require.Equal(t, 403, code("ro", s3.UndefinedMethod, "b", nil))
	require.Equal(t, 403, code("scoped", s3.UndefinedMethod, "logs", nil))

	rec := run(t, pols, "ro", s3.PutObject, "b", nil)
	require.Contains(t, rec.Body.String(), "<Code>AccessDenied</Code>")
}

func TestListBucketsFiltered(t *testing.T) {
	pols := map[string]accesspolicy.Policy{
		"scoped": {User: "scoped", Permission: accesspolicy.ReadOnly, Buckets: []string{"photos-*", "logs"}},
		"ro":     {User: "ro", Permission: accesspolicy.ReadOnly, Buckets: []string{"*"}},
	}
	rec := run(t, pols, "scoped", s3.ListBuckets, "", nil)
	require.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "<Name>photos-1</Name>")
	require.Contains(t, body, "<Name>logs</Name>")
	require.NotContains(t, body, "secret")
	require.Contains(t, body, "<ID>o</ID>")
	require.True(t, strings.Contains(body, "xmlns=\"http://s3.amazonaws.com/doc/2006-03-01/\""), body)
	require.NotEqual(t, "1", rec.Header().Get("Content-Length"))

	// all buckets: untouched
	rec = run(t, pols, "ro", s3.ListBuckets, "", nil)
	require.Contains(t, rec.Body.String(), "secret")
}

// A storage that gzips ListBuckets when asked must not break filtering: Accept-Encoding is dropped before forwarding.
func TestListBucketsFilteredWithGzipUpstream(t *testing.T) {
	pols := map[string]accesspolicy.Policy{"scoped": {User: "scoped", Permission: accesspolicy.ReadOnly, Buckets: []string{"photos-*", "logs"}}}
	lookup := func(_ context.Context, u string) (accesspolicy.Policy, bool) { p, ok := pols[u]; return p, ok }
	var seenAcceptEncoding string
	h := Middleware(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAcceptEncoding = r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Type", "application/xml")
		if strings.Contains(seenAcceptEncoding, "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			zw := gzip.NewWriter(w)
			_, _ = zw.Write([]byte(listXML))
			_ = zw.Close()
			return
		}
		_, _ = w.Write([]byte(listXML))
	}))
	{
		t.Run("header dropped", func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Accept-Encoding", "gzip")
			ctx := log.WithUser(req.Context(), "scoped")
			ctx = log.WithMethod(ctx, s3.ListBuckets)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req.WithContext(ctx))
			require.Equal(t, 200, rec.Code)
			require.Empty(t, seenAcceptEncoding, "Accept-Encoding must not be forwarded when a filter applies")
			require.Empty(t, rec.Header().Get("Content-Encoding"))
			require.Contains(t, rec.Body.String(), "photos-1")
			require.NotContains(t, rec.Body.String(), "secret")
		})
	}

	// defense in depth: a storage that gzips regardless is decoded, never leaked or garbled
	h = Middleware(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		_, _ = zw.Write([]byte(listXML))
		_ = zw.Close()
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := log.WithMethod(log.WithUser(req.Context(), "scoped"), s3.ListBuckets)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), "photos-1")
	require.NotContains(t, rec.Body.String(), "secret")

	// undecodable gzip: fail closed
	h = Middleware(lookup)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write([]byte("not gzip"))
	}))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))
	require.Equal(t, 500, rec.Code)
	require.NotContains(t, rec.Body.String(), "secret")
}

func runOpts(t *testing.T, pols map[string]accesspolicy.Policy, user string, m s3.Method, bucket string, opts ...Option) *httptest.ResponseRecorder {
	t.Helper()
	lookup := func(_ context.Context, u string) (accesspolicy.Policy, bool) { p, ok := pols[u]; return p, ok }
	reached := false
	h := Middleware(lookup, opts...)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		if m == s3.ListBuckets {
			_, _ = w.Write([]byte(listXML))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ctx := log.WithUser(req.Context(), user)
	ctx = log.WithMethod(ctx, m)
	ctx = log.WithBucket(ctx, bucket)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req.WithContext(ctx))
	if rec.Code == 403 || (m == s3.ListBuckets && !strings.Contains(rec.Body.String(), "<Name>")) {
		// denied requests never reach the storage unless filtered
		if rec.Code == 403 {
			require.False(t, reached)
		}
	}
	return rec
}

func TestDefaultDenyForSharedAccount(t *testing.T) {
	pols := map[string]accesspolicy.Policy{
		"scoped": {User: "scoped", Permission: accesspolicy.ReadWrite, Buckets: []string{"app-*"}},
	}
	shared := WithSharedAccount(func(_ context.Context, u string) bool { return u != "own" })

	// shared user without policy: 403 with a clear message, empty bucket list
	rec := runOpts(t, pols, "nopol", s3.GetObject, "b", shared)
	require.Equal(t, 403, rec.Code)
	require.Contains(t, rec.Body.String(), "<Code>AccessDenied</Code>")
	require.Contains(t, rec.Body.String(), "users on a shared storage account need an explicit policy")
	require.Equal(t, 403, runOpts(t, pols, "nopol", s3.CreateBucket, "b", shared).Code)
	rec = runOpts(t, pols, "nopol", s3.ListBuckets, "", shared)
	require.Equal(t, 200, rec.Code)
	require.Contains(t, rec.Body.String(), "<ListAllMyBucketsResult")
	require.NotContains(t, rec.Body.String(), "<Name>")

	// a user with its own credentials keeps full access (legacy)
	require.Equal(t, 200, runOpts(t, pols, "own", s3.PutObject, "any", shared).Code)
	require.Contains(t, runOpts(t, pols, "own", s3.ListBuckets, "", shared).Body.String(), "secret")
	// without options everything is legacy
	require.Equal(t, 200, runOpts(t, pols, "nopol", s3.PutObject, "any").Code)
	// allow-legacy mode is the default behaviour
	require.Equal(t, 403, runOpts(t, pols, "nopol", s3.GetObject, "b", shared, WithDefaultPolicy(DefaultLegacy)).Code)
	require.Equal(t, 200, runOpts(t, pols, "own", s3.GetObject, "b", shared, WithDefaultPolicy(DefaultLegacy)).Code)
	// deny mode: also users with their own credentials
	require.Equal(t, 403, runOpts(t, pols, "own", s3.GetObject, "b", shared, WithDefaultPolicy(DefaultDeny)).Code)
	require.Equal(t, 403, runOpts(t, pols, "own", s3.GetObject, "b", WithDefaultPolicy(DefaultDeny)).Code)
	require.NotContains(t, runOpts(t, pols, "own", s3.ListBuckets, "", WithDefaultPolicy(DefaultDeny)).Body.String(), "<Name>")
	// a policy always wins, in every mode
	require.Equal(t, 200, runOpts(t, pols, "scoped", s3.GetObject, "app-1", shared, WithDefaultPolicy(DefaultDeny)).Code)
}

// On a shared account the policy globs are the only isolation: bucket creation outside them, access to foreign
// buckets and listing foreign buckets are all refused.
func TestSharedAccountPolicyIsolation(t *testing.T) {
	pols := map[string]accesspolicy.Policy{
		"alice": {User: "alice", Permission: accesspolicy.ReadWrite, Buckets: []string{"alice-*"}},
		"bob":   {User: "bob", Permission: accesspolicy.ReadWrite, Buckets: []string{"bob-*"}},
	}
	shared := WithSharedAccount(func(context.Context, string) bool { return true })
	require.Equal(t, 200, runOpts(t, pols, "alice", s3.CreateBucket, "alice-data", shared).Code)
	require.Equal(t, 403, runOpts(t, pols, "alice", s3.CreateBucket, "bob-data", shared).Code)
	require.Equal(t, 403, runOpts(t, pols, "alice", s3.CreateBucket, "other", shared).Code)
	require.Equal(t, 403, runOpts(t, pols, "alice", s3.GetObject, "bob-data", shared).Code)
	require.Equal(t, 403, runOpts(t, pols, "alice", s3.DeleteBucket, "bob-data", shared).Code)
	rec := runOpts(t, pols, "alice", s3.ListBuckets, "", shared)
	require.Equal(t, 200, rec.Code)
	require.NotContains(t, rec.Body.String(), "secret")
	require.NotContains(t, rec.Body.String(), "bob")
}
