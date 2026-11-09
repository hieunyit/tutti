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

package auth

import (
	"bytes"
	"crypto/sha256"
	"hash"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/minio/minio-go/v7/pkg/signer"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/s3"
)

// sha256Hasher satisfies the hasher argument of signer.StreamingSignV4.
type sha256Hasher struct{ hash.Hash }

func (sha256Hasher) Close() {}

func signedStreamingRequest(t *testing.T, method, path string, data []byte, mutate func(*http.Request)) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(data))
	req.Host = "s3.example.com:9669"
	req.Header.Set("Content-Encoding", "aws-chunked")
	if mutate != nil {
		mutate(req)
	}
	req = signer.StreamingSignV4(req, presignTestAccessKey, presignTestSecretKey, "", "us-east-1",
		int64(len(data)), time.Now().UTC(), sha256Hasher{sha256.New()})
	// the signer leaves the aws-chunked body unbuffered: put it on the wire
	wire, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	req.Body = io.NopCloser(bytes.NewReader(wire))
	req.ContentLength = int64(len(wire))
	req.Header.Set("Content-Length", strconv.Itoa(len(wire)))
	return req
}

func randomData(n int) []byte {
	data := make([]byte, n)
	_, _ = rand.New(rand.NewSource(1)).Read(data) //nolint:gosec
	return data
}

func TestWrapDecodesStreamingSignedPayload(t *testing.T) {
	t.Parallel()
	data := randomData(200*1024 + 17) // several 64 KiB chunks + a partial one

	req := signedStreamingRequest(t, http.MethodPut, "/bucket-test/object.bin", data, func(r *http.Request) {
		r.Header.Set("Content-Encoding", "aws-chunked, gzip")
	})
	require.Equal(t, "STREAMING-AWS4-HMAC-SHA256-PAYLOAD", req.Header.Get(s3.AmzContentSha256))
	require.Greater(t, req.ContentLength, int64(len(data)))

	var (
		called bool
		got    []byte
		fwd    *http.Request
	)
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		fwd = r
		var err error
		got, err = io.ReadAll(r.Body)
		require.NoError(t, err)
	})
	rec := httptest.NewRecorder()
	presignTestMiddleware().Wrap(next).ServeHTTP(rec, req)

	require.True(t, called, "status=%d body=%s", rec.Code, rec.Body.String())
	require.Equal(t, data, got)
	require.Equal(t, int64(len(data)), fwd.ContentLength)
	require.Equal(t, strconv.Itoa(len(data)), fwd.Header.Get("Content-Length"))
	require.Equal(t, "UNSIGNED-PAYLOAD", fwd.Header.Get(s3.AmzContentSha256))
	require.Empty(t, fwd.Header.Values("X-Amz-Decoded-Content-Length"))
	require.Empty(t, fwd.Header.Values("X-Amz-Trailer"))
	require.Equal(t, "gzip", fwd.Header.Get("Content-Encoding"))
}

func TestWrapStreamingDropsEmptyContentEncoding(t *testing.T) {
	t.Parallel()
	req := signedStreamingRequest(t, http.MethodPut, "/bucket-test/object.bin", []byte("hello"), nil)

	var fwd *http.Request
	var got []byte
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		fwd = r
		got, _ = io.ReadAll(r.Body)
	})
	presignTestMiddleware().Wrap(next).ServeHTTP(httptest.NewRecorder(), req)

	require.NotNil(t, fwd)
	require.Equal(t, "hello", string(got))
	require.NotContains(t, fwd.Header, "Content-Encoding")
}

func TestWrapStreamingEmptyPayload(t *testing.T) {
	t.Parallel()
	req := signedStreamingRequest(t, http.MethodPut, "/bucket-test/empty", nil, nil)

	var fwd *http.Request
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { fwd = r })
	rec := httptest.NewRecorder()
	presignTestMiddleware().Wrap(next).ServeHTTP(rec, req)

	require.NotNil(t, fwd, "status=%d", rec.Code)
	require.Equal(t, int64(0), fwd.ContentLength)
}

func TestWrapStreamingTamperedChunkFailsBeforeLastByte(t *testing.T) {
	t.Parallel()
	data := randomData(200 * 1024)
	req := signedStreamingRequest(t, http.MethodPut, "/bucket-test/object.bin", data, nil)

	wire, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	wire[len(wire)/2] ^= 0xff // corrupt data in the middle chunk
	req.Body = io.NopCloser(bytes.NewReader(wire))

	var read int
	var readErr error
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		read, readErr = len(b), err
	})
	presignTestMiddleware().Wrap(next).ServeHTTP(httptest.NewRecorder(), req)

	require.Error(t, readErr)
	require.Contains(t, readErr.Error(), "SignatureDoesNotMatch")
	require.Less(t, read, len(data))
}

func TestWrapStreamingRequiresDecodedLength(t *testing.T) {
	t.Parallel()
	req := signedStreamingRequest(t, http.MethodPut, "/bucket-test/object.bin", []byte("x"), nil)
	// x-amz-decoded-content-length is a signed header: re-point the check at
	// the decoder directly instead of re-signing
	m := presignTestMiddleware()
	req.Header.Del("X-Amz-Decoded-Content-Length")
	err := m.decodeStreamingBody(req, "STREAMING-AWS4-HMAC-SHA256-PAYLOAD", &signingInfo{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "MissingContentLength")
}

func TestWrapStreamingUnsupportedMode(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodPut, "/bucket-test/object.bin", nil)
	err := presignTestMiddleware().decodeStreamingBody(req, "STREAMING-AWS4-ECDSA-P256-SHA256-PAYLOAD", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "NotImplemented")
}
