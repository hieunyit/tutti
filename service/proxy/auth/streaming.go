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
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3/chunked"
)

// decodeStreamingBody replaces the aws-chunked body of an authenticated
// request with a reader that yields the plain, verified payload.
//
// Chunk signatures chain from the seed signature computed with the CLIENT's
// key, so they cannot be forwarded once the proxy re-signs the request for
// the storage: the storage would reject the chain. The proxy verifies them
// itself and forwards a plain body declared as UNSIGNED-PAYLOAD.
func (m *middleware) decodeStreamingBody(r *http.Request, payloadHash string, sig *signingInfo) error {
	mode, err := chunked.ParseMode(payloadHash)
	if err != nil {
		return err
	}
	var signing *chunked.Signing
	if sig != nil {
		signing = &chunked.Signing{Key: sig.signingKey, AmzDate: sig.amzDate, Scope: sig.scope, Signature: sig.seedSig}
	}
	decodedLen, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get("X-Amz-Decoded-Content-Length")), 10, 64)
	if err != nil || decodedLen < 0 {
		return chunkedError("MissingContentLength", "x-amz-decoded-content-length is required for streaming payloads", http.StatusBadRequest)
	}
	// the checksum header describes the body only for PutObject / UploadPart
	method := s3.ParseReqForHost(r, m.endpoint).Method
	opts := chunked.Options{VerifyChecksumHeaders: method == s3.PutObject || method == s3.UploadPart}
	dec, err := chunked.New(r.Body, mode, r.Header, signing, opts)
	if err != nil {
		return err
	}

	r.Body = dec
	r.ContentLength = decodedLen
	r.Header.Set(s3.AmzContentSha256, unsignedPayload)
	r.Header.Set("Content-Length", strconv.FormatInt(decodedLen, 10))
	r.Header.Del("X-Amz-Decoded-Content-Length")
	r.Header.Del("X-Amz-Trailer")
	stripAwsChunkedEncoding(r.Header)

	if decodedLen == 0 {
		// nobody reads an empty body: run the end-of-body checks now
		if _, err = io.Copy(io.Discard, dec); err != nil {
			return err
		}
	}
	return nil
}

func chunkedError(code, msg string, status int) error {
	return &chunked.Error{Code: code, Message: msg, StatusCode: status}
}

// stripAwsChunkedEncoding removes the aws-chunked token from Content-Encoding.
func stripAwsChunkedEncoding(h http.Header) {
	var left []string
	for _, line := range h.Values("Content-Encoding") {
		for _, enc := range strings.Split(line, ",") {
			enc = strings.TrimSpace(enc)
			if enc != "" && !strings.EqualFold(enc, "aws-chunked") {
				left = append(left, enc)
			}
		}
	}
	if len(left) == 0 {
		h.Del("Content-Encoding")
		return
	}
	h.Set("Content-Encoding", strings.Join(left, ", "))
}
