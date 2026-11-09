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
