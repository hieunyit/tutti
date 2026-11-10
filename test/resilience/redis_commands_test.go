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
	"io"
	"testing"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/service/proxy"
)

func TestRedisCommandsPerRequest(t *testing.T) {
	for _, disabled := range []bool{true, false} {
		name := "snapshot"
		if disabled {
			name = "no-snapshot"
		}
		t.Run(name, func(t *testing.T) {
			fr := newFailingRedis(t)
			e := startEnv(t, envOpts{redis: fr, proxyHook: func(c *proxy.Config) { c.Snapshot.Disabled = disabled }})
			bucket := bucketName("cmds")
			ctx := t.Context()
			c := e.proxyClient(t)
			require.NoError(t, direct[mainKey].MakeBucket(ctx, bucket, mclient.MakeBucketOptions{}))
			_, err := c.PutObject(ctx, bucket, "o", bytes.NewReader([]byte("hello")), 5, mclient.PutObjectOptions{})
			require.NoError(t, err)
			time.Sleep(2 * time.Second)
			const n = 200
			run := func(f func()) float64 {
				// idle baseline for the same wall time
				start := time.Now()
				b0 := fr.CommandCount()
				for i := 0; i < n; i++ {
					f()
				}
				used := fr.CommandCount() - b0
				el := time.Since(start)
				i0 := fr.CommandCount()
				time.Sleep(el)
				idle := fr.CommandCount() - i0
				return float64(used-idle) / n
			}
			get := run(func() {
				o, err := c.GetObject(ctx, bucket, "o", mclient.GetObjectOptions{})
				require.NoError(t, err)
				_, _ = io.Copy(io.Discard, o)
				o.Close()
			})
			head := run(func() { _, err := c.StatObject(ctx, bucket, "o", mclient.StatObjectOptions{}); require.NoError(t, err) })
			list := run(func() {
				for range c.ListObjects(ctx, bucket, mclient.ListObjectsOptions{}) {
				}
			})
			put := run(func() {
				_, err := c.PutObject(ctx, bucket, "p", bytes.NewReader([]byte("x")), 1, mclient.PutObjectOptions{})
				require.NoError(t, err)
			})
			t.Logf("full-proxy redis commands per request, %s (idle-corrected, no replication on the bucket): GET=%.2f HEAD=%.2f LIST=%.2f PUT=%.2f", name, get, head, list, put)
			if !disabled {
				// PERF-3: steady-state reads need no Redis command for routing/policy, writes one version read
				require.Less(t, get, 1.5)
				require.Less(t, head, 1.5)
				require.Less(t, list, 1.5)
				require.Less(t, put, 2.5)
			} else {
				require.Greater(t, get, 8.0)
			}
		})
	}
}
