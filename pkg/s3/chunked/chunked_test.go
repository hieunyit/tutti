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

package chunked

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// Example from the AWS documentation "Signature Calculations for the
// Authorization Header: Transferring Payload in Multiple Chunks".
const (
	docSecret = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	docDate   = "20130524T000000Z"
	docScope  = "20130524/us-east-1/s3/aws4_request"
	docSeed   = "4f232c4386841ef735655705268965c44a0e4690baa4adea153f7db9fa80a0a9"
)

var docSigs = [3]string{
	"ad80c730a21e5b8d04586a2213dd63b9a0e99e0e2307b0ade35a65485a288648",
	"0055627c9e194cb4542bae2aa5492e3c1575bbb81b612b7d234b86a503ef5497",
	"b6c6ea8a5354eaf15b3cb7646744f4275b71ea724fed81ceb9323e279d449df9",
}

func hm(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

func docSigning() *Signing {
	k := hm([]byte("AWS4"+docSecret), "20130524")
	k = hm(k, "us-east-1")
	k = hm(k, "s3")
	k = hm(k, "aws4_request")
	return &Signing{Key: k, AmzDate: docDate, Scope: docScope, Signature: docSeed}
}

func docBody(sigs [3]string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "10000;chunk-signature=%s\r\n", sigs[0])
	b.Write(bytes.Repeat([]byte("a"), 65536))
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "400;chunk-signature=%s\r\n", sigs[1])
	b.Write(bytes.Repeat([]byte("a"), 1024))
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "0;chunk-signature=%s\r\n\r\n", sigs[2])
	return b.Bytes()
}

func hdr(kv ...string) http.Header {
	h := http.Header{}
	for i := 0; i < len(kv); i += 2 {
		h.Set(kv[i], kv[i+1])
	}
	return h
}

type chunkReader struct {
	data []byte
	n    int
}

func (c *chunkReader) Read(p []byte) (int, error) {
	if len(c.data) == 0 {
		return 0, io.EOF
	}
	k := min(c.n, len(c.data), len(p))
	copy(p, c.data[:k])
	c.data = c.data[k:]
	return k, nil
}

func newReader(t *testing.T, body io.Reader, sha string, h http.Header, sign *Signing, opts Options) (*Reader, error) {
	t.Helper()
	mode, err := ParseMode(sha)
	if err != nil {
		return nil, err
	}
	return New(body, mode, h, sign, opts)
}

func decode(t *testing.T, body io.Reader, sha string, h http.Header, sign *Signing, opts Options) ([]byte, error) {
	r, err := newReader(t, body, sha, h, sign, opts)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(r)
}

func requireCode(t *testing.T, err error, code string, status int) {
	t.Helper()
	var ce *Error
	require.True(t, errors.As(err, &ce), "err=%v", err)
	require.Equal(t, code, ce.Code)
	require.Equal(t, status, ce.StatusCode)
}

const (
	signedSha   = "STREAMING-AWS4-HMAC-SHA256-PAYLOAD"
	unsignedTrl = "STREAMING-UNSIGNED-PAYLOAD-TRAILER"
)

func TestSignedChunksAWSDocExample(t *testing.T) {
	h := hdr("X-Amz-Decoded-Content-Length", "66560")
	for _, split := range []int{1, 7, 4096, 1 << 20} {
		out, err := decode(t, &chunkReader{docBody(docSigs), split}, signedSha, h, docSigning(), Options{})
		require.NoError(t, err)
		require.Len(t, out, 66560)
		require.Equal(t, bytes.Repeat([]byte("a"), 66560), out)
	}
}

func TestSignedChunksRejectBadSignatureBeforeLastByte(t *testing.T) {
	sigs := docSigs
	sigs[1] = "0000000000000000000000000000000000000000000000000000000000000000"
	r, err := newReader(t, &chunkReader{docBody(sigs), 4096}, signedSha, hdr(), docSigning(), Options{})
	require.NoError(t, err)
	out, err := io.ReadAll(r)
	requireCode(t, err, "SignatureDoesNotMatch", 403)
	require.Less(t, len(out), 66560)
	// the error is sticky
	_, err = r.Read(make([]byte, 10))
	requireCode(t, err, "SignatureDoesNotMatch", 403)
}

func TestSignedChunksBadFinalSignatureWithholdsLastPiece(t *testing.T) {
	sigs := docSigs
	sigs[2] = "0000000000000000000000000000000000000000000000000000000000000000"
	out, err := decode(t, &chunkReader{docBody(sigs), 4096}, signedSha, hdr(), docSigning(), Options{})
	requireCode(t, err, "SignatureDoesNotMatch", 403)
	require.Less(t, len(out), 66560)
}

func TestSignedChunksRequireSigning(t *testing.T) {
	_, err := newReader(t, bytes.NewReader(nil), signedSha, hdr(), nil, Options{})
	requireCode(t, err, "AccessDenied", 403)
}

func TestSignedTrailer(t *testing.T) {
	sign := docSigning()
	data := []byte("hello signed trailer")
	crc := crc32.ChecksumIEEE(data)
	b64 := base64.StdEncoding.EncodeToString([]byte{byte(crc >> 24), byte(crc >> 16), byte(crc >> 8), byte(crc)})
	build := func(badTrailerSig bool) []byte {
		sum := sha256.Sum256(data)
		sts := func(prev, h string) string {
			return "AWS4-HMAC-SHA256-PAYLOAD\n" + docDate + "\n" + docScope + "\n" + prev + "\n" + emptySHA256 + "\n" + h
		}
		s1 := hex.EncodeToString(hm(sign.Key, sts(docSeed, hex.EncodeToString(sum[:]))))
		s2 := hex.EncodeToString(hm(sign.Key, sts(s1, emptySHA256)))
		tr := "x-amz-checksum-crc32:" + b64 + "\n"
		th := sha256.Sum256([]byte(tr))
		ts := hex.EncodeToString(hm(sign.Key, "AWS4-HMAC-SHA256-TRAILER\n"+docDate+"\n"+docScope+"\n"+s2+"\n"+hex.EncodeToString(th[:])))
		if badTrailerSig {
			ts = s1
		}
		return []byte(fmt.Sprintf("%x;chunk-signature=%s\r\n%s\r\n0;chunk-signature=%s\r\nx-amz-checksum-crc32:%s\r\nx-amz-trailer-signature:%s\r\n\r\n", len(data), s1, data, s2, b64, ts))
	}
	h := hdr("X-Amz-Trailer", "x-amz-checksum-crc32", "X-Amz-Decoded-Content-Length", fmt.Sprint(len(data)))
	sha := "STREAMING-AWS4-HMAC-SHA256-PAYLOAD-TRAILER"
	out, err := decode(t, &chunkReader{build(false), 5}, sha, h, sign, Options{})
	require.NoError(t, err)
	require.Equal(t, data, out)
	out, err = decode(t, &chunkReader{build(true), 5}, sha, h, sign, Options{})
	requireCode(t, err, "SignatureDoesNotMatch", 403)
	require.Less(t, len(out), len(data))
}

func crc32b64(data []byte) string {
	c := crc32.ChecksumIEEE(data)
	return base64.StdEncoding.EncodeToString([]byte{byte(c >> 24), byte(c >> 16), byte(c >> 8), byte(c)})
}

func unsignedTrailerBody(data []byte, trailer string) []byte {
	var b bytes.Buffer
	for len(data) > 0 {
		n := min(1000, len(data))
		fmt.Fprintf(&b, "%x\r\n", n)
		b.Write(data[:n])
		b.WriteString("\r\n")
		data = data[n:]
	}
	fmt.Fprintf(&b, "0\r\n%s\r\n\r\n", trailer)
	return b.Bytes()
}

func TestUnsignedTrailerValidChecksum(t *testing.T) {
	data := make([]byte, 5000)
	for i := range data {
		data[i] = byte(i % 251)
	}
	h := hdr("X-Amz-Trailer", "x-amz-checksum-crc32", "X-Amz-Decoded-Content-Length", "5000")
	body := unsignedTrailerBody(data, "x-amz-checksum-crc32:"+crc32b64(data))
	for _, split := range []int{1, 13, 100000} {
		out, err := decode(t, &chunkReader{body, split}, unsignedTrl, h, nil, Options{})
		require.NoError(t, err)
		require.Equal(t, data, out)
	}
}

func TestUnsignedTrailerWrongChecksum(t *testing.T) {
	h := hdr("X-Amz-Trailer", "x-amz-checksum-crc32")
	body := unsignedTrailerBody([]byte("hello world"), "x-amz-checksum-crc32:"+crc32b64([]byte("other")))
	out, err := decode(t, bytes.NewReader(body), unsignedTrl, h, nil, Options{})
	requireCode(t, err, "XAmzContentChecksumMismatch", 400)
	require.Less(t, len(out), len("hello world"))
}

func TestMissingDeclaredTrailer(t *testing.T) {
	h := hdr("X-Amz-Trailer", "x-amz-checksum-crc32")
	_, err := decode(t, bytes.NewReader([]byte("5\r\nhello\r\n0\r\n\r\n")), unsignedTrl, h, nil, Options{})
	requireCode(t, err, "InvalidRequest", 400)
}

func TestDecodedLengthMismatch(t *testing.T) {
	for _, l := range []string{"6", "4"} {
		h := hdr("X-Amz-Decoded-Content-Length", l)
		_, err := decode(t, bytes.NewReader([]byte("5\r\nhello\r\n0\r\n\r\n")), unsignedTrl, h, nil, Options{})
		requireCode(t, err, "IncompleteBody", 400)
	}
}

func TestTruncatedChunk(t *testing.T) {
	_, err := decode(t, bytes.NewReader([]byte("a\r\nhello")), unsignedTrl, hdr(), nil, Options{})
	requireCode(t, err, "IncompleteBody", 400)
}

func TestGarbageChunkSize(t *testing.T) {
	_, err := decode(t, bytes.NewReader([]byte("zz\r\nhello\r\n")), unsignedTrl, hdr(), nil, Options{})
	requireCode(t, err, "InvalidRequest", 400)
}

func TestBoundedHeaderAndTrailer(t *testing.T) {
	_, err := decode(t, bytes.NewReader(bytes.Repeat([]byte("1"), 5000)), unsignedTrl, hdr(), nil, Options{})
	requireCode(t, err, "InvalidRequest", 400)
	body := append([]byte("0\r\n"), bytes.Repeat([]byte("x-a:"+string(bytes.Repeat([]byte("v"), 1000))+"\r\n"), 20)...)
	_, err = decode(t, bytes.NewReader(body), unsignedTrl, hdr(), nil, Options{})
	requireCode(t, err, "InvalidRequest", 400)
}

func TestUnsupportedStreamingMode(t *testing.T) {
	_, err := ParseMode("STREAMING-AWS4-ECDSA-P256-SHA256-PAYLOAD")
	requireCode(t, err, "NotImplemented", 501)
	require.True(t, IsStreaming("STREAMING-X"))
	require.False(t, IsStreaming("UNSIGNED-PAYLOAD"))
}

func TestChecksumKnownVectors(t *testing.T) {
	for _, c := range []struct{ header, hex string }{
		{"x-amz-checksum-crc32", "cbf43926"},
		{"x-amz-checksum-crc32c", "e3069283"},
		{"x-amz-checksum-crc64nvme", "ae8b14860a799888"},
		{"x-amz-checksum-sha1", "f7c3bc1d808e04732adf679965ccc34ca7ae3441"},
		{"x-amz-checksum-sha256", "15e2b0d3c33891ebb0f1ef609ec419420c20e320ce94c65fbc8c3312448eb225"},
	} {
		a, ok := findAlgo(c.header)
		require.True(t, ok)
		h := a.newH()
		h.Write([]byte("123456789"))
		require.Equal(t, c.hex, hex.EncodeToString(h.Sum(nil)), c.header)
	}
}

func TestHeaderChecksum(t *testing.T) {
	body := []byte("5\r\nhello\r\n0\r\n\r\n")
	h := hdr("X-Amz-Checksum-Crc32", crc32b64([]byte("other")))
	_, err := decode(t, bytes.NewReader(body), unsignedTrl, h, nil, Options{VerifyChecksumHeaders: true})
	requireCode(t, err, "XAmzContentChecksumMismatch", 400)
	out, err := decode(t, bytes.NewReader(body), unsignedTrl, h, nil, Options{})
	require.NoError(t, err)
	require.Equal(t, "hello", string(out))
	h = hdr("X-Amz-Checksum-Crc32", crc32b64([]byte("hello")))
	out, err = decode(t, bytes.NewReader(body), unsignedTrl, h, nil, Options{VerifyChecksumHeaders: true})
	require.NoError(t, err)
	require.Equal(t, "hello", string(out))
}

// A mid-stream failure must surface from http.Client as an error that still
// unwraps to *Error, so the proxy can turn it into an S3 error response.
func TestErrorSurfacesThroughHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
	}))
	defer srv.Close()

	sigs := docSigs
	sigs[1] = "0000000000000000000000000000000000000000000000000000000000000000"
	r, err := newReader(t, &chunkReader{docBody(sigs), 4096}, signedSha, hdr(), docSigning(), Options{})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPut, srv.URL, io.NopCloser(r))
	require.NoError(t, err)
	req.ContentLength = 66560
	resp, err := http.DefaultClient.Do(req)
	if resp != nil {
		_ = resp.Body.Close()
	}
	requireCode(t, err, "SignatureDoesNotMatch", 403)
}
