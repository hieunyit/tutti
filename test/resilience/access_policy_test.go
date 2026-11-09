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
package resilience

import (
	"bytes"
	"net/http"
	"testing"

	mclient "github.com/minio/minio-go/v7"
	mcredentials "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/service/proxy"
)

// accessClient is a proxy client authenticating with a custom access key.
func (e *testEnv) accessClient(t testing.TB, key string) *mclient.Client {
	t.Helper()
	c, err := mclient.New(e.ProxyAddr, &mclient.Options{
		Creds:  mcredentials.NewStaticV4(key, key+"-secret-1234", ""),
		Region: "us-east-1",
	})
	require.NoError(t, err)
	return c
}

func TestAccessPolicies(t *testing.T) {
	// custom accounts "reader" and "scoped" act on the storages with the main storage credentials
	for _, u := range []string{"reader", "scoped"} {
		for _, st := range storages {
			st.Credentials[u] = mainCreds
		}
	}
	t.Cleanup(func() {
		for _, u := range []string{"reader", "scoped"} {
			for _, st := range storages {
				delete(st.Credentials, u)
			}
		}
	})
	e := startEnv(t, envOpts{proxyHook: func(c *proxy.Config) {
		c.Auth.Custom = map[string]s3.CredentialsV4{
			"reader": {AccessKeyID: "reader-key", SecretAccessKey: "reader-key-secret-1234"},
			"scoped": {AccessKeyID: "scoped-key", SecretAccessKey: "scoped-key-secret-1234"},
		}
	}})
	ctx := t.Context()

	open := bucketName("ap-open")
	photos := "photos-" + bucketName("x")
	secret := bucketName("ap-secret")
	for _, b := range []string{open, photos, secret} {
		require.NoError(t, direct[mainKey].MakeBucket(ctx, b, mclient.MakeBucketOptions{}))
		_, err := direct[mainKey].PutObject(ctx, b, "obj", bytes.NewReader([]byte("data")), 4, mclient.PutObjectOptions{})
		require.NoError(t, err)
	}

	// policies are set before the first request: proxies cache them for a few seconds
	e.restJSON(t, http.MethodPost, "/tutti/access-policies", map[string]any{"user": "reader", "permission": "read-only"})
	e.restJSON(t, http.MethodPost, "/tutti/access-policies", map[string]any{
		"user": "scoped", "permission": "read-write", "buckets": []string{"photos-*"}})
	list := e.restJSON(t, http.MethodGet, "/tutti/access-policies", nil)
	require.Len(t, list["policies"], 2)

	denied := func(t *testing.T, err error) {
		t.Helper()
		require.Error(t, err)
		require.Equal(t, "AccessDenied", mclient.ToErrorResponse(err).Code)
		require.Equal(t, http.StatusForbidden, mclient.ToErrorResponse(err).StatusCode)
	}

	t.Run("read-only", func(t *testing.T) {
		c := e.accessClient(t, "reader-key")
		got, err := readObject(t, c, open, "obj")
		require.NoError(t, err)
		require.Equal(t, "data", got)
		_, err = c.PutObject(ctx, open, "new", bytes.NewReader([]byte("x")), 1, mclient.PutObjectOptions{})
		denied(t, err)
		denied(t, c.RemoveObject(ctx, open, "obj", mclient.RemoveObjectOptions{}))
		denied(t, c.MakeBucket(ctx, bucketName("ap-new"), mclient.MakeBucketOptions{}))
		require.False(t, existsDirect(t, mainKey, open, "new"))
		require.True(t, existsDirect(t, mainKey, open, "obj"))
		buckets, err := c.ListBuckets(ctx)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(buckets), 3)
	})

	t.Run("bucket-restricted", func(t *testing.T) {
		c := e.accessClient(t, "scoped-key")
		_, err := c.PutObject(ctx, photos, "new", bytes.NewReader([]byte("x")), 1, mclient.PutObjectOptions{})
		require.NoError(t, err)
		got, err := readObject(t, c, photos, "new")
		require.NoError(t, err)
		require.Equal(t, "x", got)
		_, err = readObject(t, c, secret, "obj")
		denied(t, err)
		_, err = c.PutObject(ctx, secret, "new", bytes.NewReader([]byte("x")), 1, mclient.PutObjectOptions{})
		denied(t, err)
		// copy from a disallowed source bucket
		_, err = c.CopyObject(ctx, mclient.CopyDestOptions{Bucket: photos, Object: "copy"}, mclient.CopySrcOptions{Bucket: secret, Object: "obj"})
		denied(t, err)
		require.False(t, existsDirect(t, mainKey, photos, "copy"))
		// bucket list is filtered
		buckets, err := c.ListBuckets(ctx)
		require.NoError(t, err)
		var names []string
		for _, b := range buckets {
			names = append(names, b.Name)
		}
		require.Contains(t, names, photos)
		require.NotContains(t, names, secret)
		require.NotContains(t, names, open)
	})

	t.Run("accounts-api", func(t *testing.T) {
		require.Eventually(t, func() bool {
			res := e.restJSON(t, http.MethodGet, "/tutti/proxy-accounts", nil)
			accs, _ := res["accounts"].([]any)
			found := 0
			for _, a := range accs {
				m := a.(map[string]any)
				if m["user"] == "reader" || m["user"] == "scoped" {
					found++
					require.Equal(t, true, m["hasPolicy"])
				}
				require.NotContains(t, m, "secretAccessKey")
			}
			return found == 2
		}, e.WaitLong, e.RetryLong)
	})
}
