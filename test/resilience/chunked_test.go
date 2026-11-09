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
	"crypto/rand"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	mclient "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/signer"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/service/proxy"
)

const (
	streamingSigned          = "STREAMING-AWS4-HMAC-SHA256-PAYLOAD"
	streamingSignedTrailer   = "STREAMING-AWS4-HMAC-SHA256-PAYLOAD-TRAILER"
	streamingUnsignedTrailer = "STREAMING-UNSIGNED-PAYLOAD-TRAILER"
)

// spyTransport records the payload mode of every request and optionally rewrites the request body.
type spyTransport struct {
	mu      sync.Mutex
	modes   []string
	status  []int
	tamper  func(body []byte) []byte
	wrapped http.RoundTripper
}

func (s *spyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodPut {
		s.mu.Lock()
		s.modes = append(s.modes, req.Header.Get("X-Amz-Content-Sha256"))
		s.mu.Unlock()
		if s.tamper != nil && req.Body != nil {
			b, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, err
			}
			req = req.Clone(req.Context())
			req.Body = io.NopCloser(bytes.NewReader(s.tamper(b)))
		}
	}
	res, err := s.wrapped.RoundTrip(req)
	if err == nil && req.Method == http.MethodPut {
		s.mu.Lock()
		s.status = append(s.status, res.StatusCode)
		s.mu.Unlock()
	}
	return res, err
}

func (s *spyTransport) lastMode() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.modes) == 0 {
		return ""
	}
	return s.modes[len(s.modes)-1]
}

func (s *spyTransport) statuses() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int(nil), s.status...)
}

func newSpy(tamper func([]byte) []byte) *spyTransport {
	return &spyTransport{tamper: tamper, wrapped: http.DefaultTransport}
}

// flipFirstChunkByte corrupts a payload byte of the first aws-chunked chunk, keeping the length and all signatures.
func flipFirstChunkByte(body []byte) []byte {
	i := bytes.Index(body, []byte("\r\n"))
	out := append([]byte(nil), body...)
	out[i+2+100] ^= 0xff
	return out
}

func randomBytes(t testing.TB, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

func TestChunked_ValidUploads(t *testing.T) {
	e := startEnv(t, envOpts{})
	bucket := bucketName("chunked-ok")
	require.NoError(t, direct[mainKey].MakeBucket(t.Context(), bucket, mclient.MakeBucketOptions{}))

	cases := []struct {
		name     string
		wantMode string
		clientOp func(o *mclient.Options)
		putOpts  mclient.PutObjectOptions
		size     int
	}{
		// multi-chunk body (minio-go chunk size is 64KiB) signed chunk by chunk
		{name: "signed-streaming", wantMode: streamingSigned, size: 200 * 1024},
		{name: "signed-streaming-empty", wantMode: streamingSigned, size: 0},
		{
			name: "signed-streaming-trailer", wantMode: streamingSignedTrailer, size: 150 * 1024,
			clientOp: func(o *mclient.Options) { o.TrailingHeaders = true },
			putOpts:  mclient.PutObjectOptions{Checksum: mclient.ChecksumCRC32C},
		},
		{
			name: "unsigned-streaming-trailer", wantMode: streamingUnsignedTrailer, size: 150 * 1024,
			clientOp: func(o *mclient.Options) { o.TrailingHeaders = true },
			putOpts:  mclient.PutObjectOptions{Checksum: mclient.ChecksumCRC32C, DisableContentSha256: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			spy := newSpy(nil)
			c := e.proxyClient(t, func(o *mclient.Options) {
				o.Transport = spy
				if tc.clientOp != nil {
					tc.clientOp(o)
				}
			})
			data := randomBytes(t, tc.size)
			_, err := c.PutObject(t.Context(), bucket, tc.name, bytes.NewReader(data), int64(len(data)), tc.putOpts)
			r.NoError(err)
			r.Equal(tc.wantMode, spy.lastMode(), "request must really use this payload mode")

			// the storage holds the decoded payload, not the aws-chunked framing
			got, err := readObject(t, direct[mainKey], bucket, tc.name)
			r.NoError(err)
			r.Equal(string(data), got)
			// and the proxy serves it back
			got, err = readObject(t, c, bucket, tc.name)
			r.NoError(err)
			r.Equal(string(data), got)
		})
	}
}

func TestChunked_TamperedChunkRejected(t *testing.T) {
	e := startEnv(t, envOpts{})
	bucket := bucketName("chunked-bad")
	require.NoError(t, direct[mainKey].MakeBucket(t.Context(), bucket, mclient.MakeBucketOptions{}))

	t.Run("signed-chunk", func(t *testing.T) {
		r := require.New(t)
		spy := newSpy(flipFirstChunkByte)
		c := e.proxyClient(t, func(o *mclient.Options) { o.Transport = spy; o.MaxRetries = 1 })
		data := randomBytes(t, 200*1024)
		_, err := c.PutObject(t.Context(), bucket, "tampered", bytes.NewReader(data), int64(len(data)), mclient.PutObjectOptions{})
		r.Error(err)
		r.Equal(streamingSigned, spy.lastMode())
		r.False(existsDirect(t, mainKey, bucket, "tampered"), "tampered upload must not create the object on the backend")
		// the proxy still serves valid uploads afterwards
		good := e.proxyClient(t)
		_, err = good.PutObject(t.Context(), bucket, "good", bytes.NewReader(data), int64(len(data)), mclient.PutObjectOptions{})
		r.NoError(err)
	})

	t.Run("unsigned-trailer-checksum-mismatch", func(t *testing.T) {
		r := require.New(t)
		spy := newSpy(func(b []byte) []byte { out := append([]byte(nil), b...); out[100] ^= 0xff; return out })
		c := e.proxyClient(t, func(o *mclient.Options) { o.Transport = spy; o.MaxRetries = 1; o.TrailingHeaders = true })
		data := randomBytes(t, 150*1024)
		_, err := c.PutObject(t.Context(), bucket, "bad-checksum", bytes.NewReader(data), int64(len(data)),
			mclient.PutObjectOptions{Checksum: mclient.ChecksumCRC32C, DisableContentSha256: true})
		r.Error(err)
		r.Equal(streamingUnsignedTrailer, spy.lastMode())
		r.False(existsDirect(t, mainKey, bucket, "bad-checksum"), "upload with a wrong trailing checksum must not create the object")
	})
}

func TestChunked_SignedContentType(t *testing.T) {
	// minio-go signs content-type, so hand-sign the request without it and add the header afterwards
	put := func(t *testing.T, e *testEnv, bucket, key string) (int, string) {
		data := []byte(strings.Repeat("x", 4096))
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPut, "http://"+e.ProxyAddr+"/"+bucket+"/"+key, bytes.NewReader(data))
		require.NoError(t, err)
		req.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")
		req = signer.SignV4(*req, mainCreds.AccessKeyID, mainCreds.SecretAccessKey, "", "us-east-1")
		req.Header.Set("Content-Type", "text/plain")
		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()
		b, err := io.ReadAll(res.Body)
		require.NoError(t, err)
		return res.StatusCode, string(b)
	}

	t.Run("default-accepts-unsigned", func(t *testing.T) {
		e := startEnv(t, envOpts{proxyHook: func(c *proxy.Config) { c.Auth.RequireSignedContentType = false }})
		bucket := bucketName("ctype-ok")
		require.NoError(t, direct[mainKey].MakeBucket(t.Context(), bucket, mclient.MakeBucketOptions{}))
		code, body := put(t, e, bucket, "obj")
		require.Equal(t, http.StatusOK, code, body)
		require.True(t, existsDirect(t, mainKey, bucket, "obj"))
	})

	t.Run("required-rejects-unsigned", func(t *testing.T) {
		e := startEnv(t, envOpts{proxyHook: func(c *proxy.Config) { c.Auth.RequireSignedContentType = true }})
		bucket := bucketName("ctype-bad")
		require.NoError(t, direct[mainKey].MakeBucket(t.Context(), bucket, mclient.MakeBucketOptions{}))
		code, body := put(t, e, bucket, "obj")
		require.Equal(t, http.StatusForbidden, code, body)
		require.False(t, existsDirect(t, mainKey, bucket, "obj"))
	})
}
