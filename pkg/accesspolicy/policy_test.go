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

package accesspolicy

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/s3"
)

func TestEveryMethodClassified(t *testing.T) {
	n := 0
	for m := s3.Method(1); ; m++ {
		if strings.HasPrefix(m.String(), "Method(") {
			break
		}
		n++
		require.NotEqual(t, ClassUnknown, Classify(m), "method %s is not classified; add it to the table in policy.go", m)
	}
	require.Greater(t, n, 90)
	require.Equal(t, ClassUnknown, Classify(s3.UndefinedMethod))
}

func TestAuthorize(t *testing.T) {
	ro := Policy{User: "u", Permission: ReadOnly, Buckets: []string{"*"}}
	wo := Policy{User: "u", Permission: WriteOnly, Buckets: []string{"*"}}
	rw := Policy{User: "u", Permission: ReadWrite, Buckets: []string{"photos-*", "logs"}}
	allowed := func(p Policy, m s3.Method, b string) bool {
		return p.Authorize(Request{Method: m, Bucket: b}) == nil
	}
	require.True(t, allowed(ro, s3.GetObject, "b"))
	require.True(t, allowed(ro, s3.ListBuckets, ""))
	require.True(t, allowed(ro, s3.HeadObject, "b"))
	require.False(t, allowed(ro, s3.PutObject, "b"))
	require.False(t, allowed(ro, s3.DeleteObject, "b"))
	require.False(t, allowed(ro, s3.CreateBucket, "b"))
	require.False(t, allowed(ro, s3.UndefinedMethod, "b"))
	require.False(t, allowed(ro, s3.Method(250), "b"))

	require.True(t, allowed(wo, s3.PutObject, "b"))
	require.True(t, allowed(wo, s3.HeadBucket, "b"))
	require.True(t, allowed(wo, s3.UploadPart, "b"))
	require.False(t, allowed(wo, s3.GetObject, "b"))
	require.False(t, allowed(wo, s3.ListObjects, "b"))
	require.False(t, allowed(wo, s3.ListBuckets, ""))
	require.False(t, allowed(wo, s3.PutBucketPolicy, "b"))

	require.True(t, allowed(rw, s3.PutObject, "photos-2024"))
	require.True(t, allowed(rw, s3.GetObject, "logs"))
	require.True(t, allowed(rw, s3.CreateBucket, "logs"))
	require.False(t, allowed(rw, s3.GetObject, "other"))
	require.False(t, allowed(rw, s3.GetObject, "logs2"))
	require.False(t, allowed(rw, s3.GetObject, ""))
	require.True(t, allowed(rw, s3.ListBuckets, ""))

	// copy needs the source bucket as well
	require.NoError(t, rw.Authorize(Request{Method: s3.CopyObject, Bucket: "logs", CopySourceBucket: "photos-1"}))
	require.Error(t, rw.Authorize(Request{Method: s3.CopyObject, Bucket: "logs", CopySourceBucket: "secret"}))
	require.Error(t, rw.Authorize(Request{Method: s3.UploadPartCopy, Bucket: "logs", CopySourceInvalid: true}))
	require.ErrorIs(t, rw.Authorize(Request{Method: s3.CopyObject, Bucket: "logs", CopySourceBucket: "secret"}), dom.ErrAuth)

	// corrupt policy fails closed
	require.Error(t, Policy{User: "u"}.Authorize(Request{Method: s3.GetObject, Bucket: "b"}))
}

func TestValidate(t *testing.T) {
	for name, p := range map[string]Policy{
		"no user":      {Permission: ReadOnly},
		"bad perm":     {User: "u", Permission: "rw"},
		"empty perm":   {User: "u"},
		"bad glob":     {User: "u", Permission: ReadOnly, Buckets: []string{"[a"}},
		"empty bucket": {User: "u", Permission: ReadOnly, Buckets: []string{""}},
		"blank bucket": {User: "u", Permission: ReadOnly, Buckets: []string{"  "}},
		"space bucket": {User: "u", Permission: ReadOnly, Buckets: []string{"my bucket"}},
		"tab bucket":   {User: "u", Permission: ReadOnly, Buckets: []string{"a\tb"}},
	} {
		require.ErrorIs(t, p.Validate(), dom.ErrInvalidArg, name)
	}
	p := Policy{User: "u", Permission: ReadOnly}
	require.NoError(t, p.Validate())
	require.Equal(t, []string{"*"}, p.Buckets)
}

func TestStoreAndCache(t *testing.T) {
	ctx := context.Background()
	mr := miniredis.RunT(t)
	st := NewStore(redis.NewClient(&redis.Options{Addr: mr.Addr()}))

	require.NoError(t, st.Set(ctx, Policy{User: "b", Permission: ReadOnly, Buckets: []string{"x-*"}, Description: "d"}))
	require.NoError(t, st.Set(ctx, Policy{User: "a", Permission: WriteOnly}))
	require.ErrorIs(t, st.Set(ctx, Policy{User: "a", Permission: "bad"}), dom.ErrInvalidArg)
	list, err := st.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "a", list[0].User)
	require.Equal(t, []string{"*"}, list[0].Buckets)
	require.Equal(t, "d", list[1].Description)

	now := time.Now()
	c := NewCache(st, 5*time.Second)
	c.now = func() time.Time { return now }
	p, ok := c.Lookup(ctx, "b")
	require.True(t, ok)
	require.Equal(t, ReadOnly, p.Permission)
	_, ok = c.Lookup(ctx, "nobody")
	require.False(t, ok)

	// change is not visible before TTL, visible after
	require.NoError(t, st.Delete(ctx, "b"))
	_, ok = c.Lookup(ctx, "b")
	require.True(t, ok)
	now = now.Add(6 * time.Second)
	_, ok = c.Lookup(ctx, "b")
	require.False(t, ok)

	// redis down: last known policies are kept
	_, ok = c.Lookup(ctx, "a")
	require.True(t, ok)
	mr.Close()
	now = now.Add(6 * time.Second)
	_, ok = c.Lookup(ctx, "a")
	require.True(t, ok)

	require.ErrorIs(t, NewStore(redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})).Delete(ctx, ""), dom.ErrInvalidArg)
}

func TestCacheNeverLoaded(t *testing.T) {
	st := NewStore(redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 100 * time.Millisecond}))
	c := NewCache(st, time.Second)
	p, ok := c.Lookup(context.Background(), "u")
	require.True(t, ok)
	require.ErrorIs(t, p.Authorize(Request{Method: s3.GetObject, Bucket: "bucket"}), dom.ErrAuth)
}
