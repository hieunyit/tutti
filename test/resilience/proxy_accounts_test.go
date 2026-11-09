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
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	mclient "github.com/minio/minio-go/v7"
	mcredentials "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"
)

func (e *testEnv) keyClient(t testing.TB, accessKey, secret string) *mclient.Client {
	t.Helper()
	c, err := mclient.New(e.ProxyAddr, &mclient.Options{
		Creds:  mcredentials.NewStaticV4(accessKey, secret, ""),
		Region: "us-east-1",
	})
	require.NoError(t, err)
	return c
}

func TestDynamicProxyAccounts(t *testing.T) {
	e := startEnv(t, envOpts{dynamicCreds: true})
	ctx := t.Context()
	bucket := bucketName("dyn-acc")
	require.NoError(t, direct[mainKey].MakeBucket(ctx, bucket, mclient.MakeBucketOptions{}))
	_, err := direct[mainKey].PutObject(ctx, bucket, "obj", strings.NewReader("data"), 4, mclient.PutObjectOptions{})
	require.NoError(t, err)

	// proxies pick up account changes within the cache TTL (5s)
	const wait, tick = 15 * time.Second, 200 * time.Millisecond
	works := func(c *mclient.Client) bool {
		_, err := c.StatObject(ctx, bucket, "obj", mclient.StatObjectOptions{})
		return err == nil
	}
	rejected := func(c *mclient.Client) bool {
		_, err := c.StatObject(ctx, bucket, "obj", mclient.StatObjectOptions{})
		return err != nil && mclient.ToErrorResponse(err).StatusCode == http.StatusForbidden
	}

	created := e.restJSON(t, http.MethodPost, "/tutti/proxy-accounts", map[string]any{"user": user, "description": "e2e"})
	keyID, secret1 := created["accessKeyId"].(string), created["secretAccessKey"].(string)
	require.True(t, strings.HasPrefix(keyID, "TUT"))
	require.Len(t, secret1, 40)

	c1 := e.keyClient(t, keyID, secret1)
	require.Eventually(t, func() bool { return works(c1) }, wait, tick, "new account must authenticate")
	// the account acts as the storage user: it can write through the proxy
	_, err = c1.PutObject(ctx, bucket, "written", bytes.NewReader([]byte("x")), 1, mclient.PutObjectOptions{})
	require.NoError(t, err)
	require.True(t, existsDirect(t, mainKey, bucket, "written"))

	// the list shows metadata but no secret
	list := e.restJSON(t, http.MethodGet, "/tutti/proxy-accounts", nil)
	require.NotContains(t, toJSON(t, list), secret1)

	// rotation with grace: both secrets work
	rot := e.restJSON(t, http.MethodPut, "/tutti/proxy-accounts/rotate", map[string]any{"accessKeyId": keyID, "gracePeriod": "3600s"})
	secret2 := rot["secretAccessKey"].(string)
	require.NotEqual(t, secret1, secret2)
	c2 := e.keyClient(t, keyID, secret2)
	require.Eventually(t, func() bool { return works(c2) }, wait, tick, "new secret must work")
	require.True(t, works(c1), "old secret must keep working within the grace period")

	// immediate rotation: previous secrets stop working
	rot = e.restJSON(t, http.MethodPut, "/tutti/proxy-accounts/rotate", map[string]any{"accessKeyId": keyID, "gracePeriod": "0s"})
	secret3 := rot["secretAccessKey"].(string)
	c3 := e.keyClient(t, keyID, secret3)
	require.Eventually(t, func() bool { return works(c3) }, wait, tick)
	require.Eventually(t, func() bool { return rejected(c2) && rejected(c1) }, wait, tick, "old secrets must be rejected after zero grace")

	// disable -> 403
	e.restJSON(t, http.MethodPut, "/tutti/proxy-accounts/status", map[string]any{"accessKeyId": keyID, "status": "disabled"})
	require.Eventually(t, func() bool { return rejected(c3) }, wait, tick, "disabled account must be rejected")
	// enable again
	e.restJSON(t, http.MethodPut, "/tutti/proxy-accounts/status", map[string]any{"accessKeyId": keyID, "status": "active"})
	require.Eventually(t, func() bool { return works(c3) }, wait, tick)

	// delete -> rejected
	e.restJSON(t, http.MethodPut, "/tutti/proxy-accounts/delete", map[string]any{"accessKeyId": keyID})
	require.Eventually(t, func() bool { return rejected(c3) }, wait, tick, "deleted account must be rejected")
	list = e.restJSON(t, http.MethodGet, "/tutti/proxy-accounts", nil)
	require.NotContains(t, toJSON(t, list), keyID)
}

func toJSON(t testing.TB, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}
