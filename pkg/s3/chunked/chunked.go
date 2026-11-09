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

// Package chunked decodes and verifies aws-chunked request bodies
// (x-amz-content-sha256: STREAMING-*).
//
// The reader returned by New yields only the decoded object bytes. It always
// withholds the most recently decoded piece until the following piece has been
// verified, and withholds the final piece until every end-of-body check
// (chunk/trailer signatures, x-amz-checksum-*, decoded length) has passed. A
// consumer that stops reading once it has the declared number of bytes can
// therefore never commit a payload that failed verification: it gets an error
// before the last byte.
package chunked

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // S3 checksum algorithm, not a security primitive
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/crc32"
	"hash/crc64"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const (
	emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	// maxChunkHeaderLen bounds a single "<hex-size>;chunk-signature=<sig>" line.
	maxChunkHeaderLen = 4 * 1024
	// maxTrailerLen bounds the whole trailer section.
	maxTrailerLen = 16 * 1024
	readSize      = 32 * 1024
)

// Error is a decoding or verification failure carrying the S3 error to report.
type Error struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func errSignature() *Error {
	return &Error{"SignatureDoesNotMatch", "The request signature that the server calculated does not match the signature that you provided. Check your AWS secret access key and signing method. For more information, see REST Authentication and SOAP Authentication.", http.StatusForbidden}
}

func errMalformed(msg string) *Error {
	return &Error{"InvalidRequest", "Malformed aws-chunked body: " + msg, http.StatusBadRequest}
}

func errIncomplete(msg string) *Error {
	return &Error{"IncompleteBody", msg, http.StatusBadRequest}
}

func errBadDigest(msg string) *Error {
	return &Error{"XAmzContentChecksumMismatch", msg, http.StatusBadRequest}
}

func errNotImplemented(msg string) *Error {
	return &Error{"NotImplemented", msg, http.StatusNotImplemented}
}

// Mode is the aws-chunked flavor announced by x-amz-content-sha256.
type Mode int

const (
	// ModeSigned is STREAMING-AWS4-HMAC-SHA256-PAYLOAD.
	ModeSigned Mode = iota + 1
	// ModeSignedTrailer is STREAMING-AWS4-HMAC-SHA256-PAYLOAD-TRAILER.
	ModeSignedTrailer
	// ModeUnsignedTrailer is STREAMING-UNSIGNED-PAYLOAD-TRAILER.
	ModeUnsignedTrailer
)

func (m Mode) signed() bool     { return m == ModeSigned || m == ModeSignedTrailer }
func (m Mode) hasTrailer() bool { return m == ModeSignedTrailer || m == ModeUnsignedTrailer }

// IsStreaming reports whether the x-amz-content-sha256 value announces a
// streaming (aws-chunked) payload.
func IsStreaming(contentSha256 string) bool {
	return strings.HasPrefix(contentSha256, "STREAMING-")
}

// ParseMode maps an x-amz-content-sha256 value to a Mode. Unsupported
// STREAMING-* values yield a NotImplemented error.
func ParseMode(contentSha256 string) (Mode, error) {
	switch contentSha256 {
	case "STREAMING-AWS4-HMAC-SHA256-PAYLOAD":
		return ModeSigned, nil
	case "STREAMING-AWS4-HMAC-SHA256-PAYLOAD-TRAILER":
		return ModeSignedTrailer, nil
	case "STREAMING-UNSIGNED-PAYLOAD-TRAILER":
		return ModeUnsignedTrailer, nil
	}
	return 0, errNotImplemented(fmt.Sprintf("Payload signing mode %q is not supported by this proxy", contentSha256))
}

// Signing is the material of the verified request signature that chunk
// signatures chain from.
type Signing struct {
	Key       []byte // derived SigV4 signing key
	AmzDate   string // e.g. 20130524T000000Z
	Scope     string // e.g. 20130524/us-east-1/s3/aws4_request
	Signature string // seed signature (from the Authorization header)
}

// Options controls which integrity checks apply.
type Options struct {
	// VerifyChecksumHeaders verifies x-amz-checksum-* request headers against
	// the decoded payload. Only valid when the header describes the request
	// body (PutObject, UploadPart).
	VerifyChecksumHeaders bool
}

// ---------------------------------------------------------------------------
// checksums
// ---------------------------------------------------------------------------

type checksumAlgo struct {
	header string
	newH   func() hash.Hash
}

var crc64NVMeTable = crc64.MakeTable(0x9a6c9329ac4bc9b5)

var checksumAlgos = []checksumAlgo{
	{"x-amz-checksum-crc32", func() hash.Hash { return crc32.NewIEEE() }},
	{"x-amz-checksum-crc32c", func() hash.Hash { return crc32.New(crc32.MakeTable(crc32.Castagnoli)) }},
	{"x-amz-checksum-crc64nvme", func() hash.Hash { return crc64.New(crc64NVMeTable) }},
	{"x-amz-checksum-sha1", func() hash.Hash { return sha1.New() }}, //nolint:gosec
	{"x-amz-checksum-sha256", func() hash.Hash { return sha256.New() }},
}

func findAlgo(header string) (checksumAlgo, bool) {
	for _, a := range checksumAlgos {
		if a.header == header {
			return a, true
		}
	}
	return checksumAlgo{}, false
}

type checksumCheck struct {
	algo        checksumAlgo
	h           hash.Hash
	expected    string
	fromTrailer bool
}

func (c *checksumCheck) verify(trailers []trailer) error {
	expected := c.expected
	if c.fromTrailer {
		found := false
		for _, t := range trailers {
			if t.name == c.algo.header {
				expected, found = t.value, true
				break
			}
		}
		if !found {
			return &Error{"InvalidRequest", fmt.Sprintf("Declared trailer %q is missing from the request body", c.algo.header), http.StatusBadRequest}
		}
	}
	actual := base64.StdEncoding.EncodeToString(c.h.Sum(nil))
	if actual != strings.TrimSpace(expected) {
		return errBadDigest(fmt.Sprintf("The %s you specified did not match the calculated checksum.", c.algo.header))
	}
	return nil
}

func newChecksumCheck(h http.Header, mode Mode, opts Options) (*checksumCheck, error) {
	if decl := strings.TrimSpace(h.Get("X-Amz-Trailer")); decl != "" {
		if !mode.hasTrailer() {
			return nil, &Error{"InvalidRequest", "x-amz-trailer requires a STREAMING-*-TRAILER payload", http.StatusBadRequest}
		}
		var found *checksumAlgo
		for _, name := range strings.Split(decl, ",") {
			lower := strings.ToLower(strings.TrimSpace(name))
			if !strings.HasPrefix(lower, "x-amz-checksum-") {
				continue
			}
			a, ok := findAlgo(lower)
			if !ok {
				return nil, errNotImplemented(fmt.Sprintf("Checksum trailer %q is not supported", name))
			}
			if found != nil {
				return nil, &Error{"InvalidRequest", "Only one checksum trailer may be declared", http.StatusBadRequest}
			}
			found = &a
		}
		if found != nil {
			return &checksumCheck{algo: *found, h: found.newH(), fromTrailer: true}, nil
		}
	}
	if opts.VerifyChecksumHeaders {
		for _, a := range checksumAlgos {
			if v := strings.TrimSpace(h.Get(a.header)); v != "" {
				return &checksumCheck{algo: a, h: a.newH(), expected: v}, nil
			}
		}
	}
	return nil, nil
}

// ---------------------------------------------------------------------------
// reader
// ---------------------------------------------------------------------------

type state int

const (
	stHeader state = iota
	stData
	stDataCRLF
	stFinalCRLF
	stTrailer
	stDone
)

type trailer struct{ name, value string }

type step int

const (
	stepData step = iota
	stepNeedMore
	stepDone
)

// Reader yields the decoded, verified payload of an aws-chunked body.
type Reader struct {
	src     io.Reader
	srcDone bool
	buf     []byte // undecoded input

	mode  Mode
	state state
	left  int64 // bytes left in the current chunk

	sign      *Signing
	prevSig   string
	chunkSig  string
	chunkHash hash.Hash

	trailerBytes int
	trailerSig   string
	trailers     []trailer

	check       *checksumCheck
	expectedLen int64 // -1 when unknown
	decodedLen  int64

	pending  []byte // withheld piece
	ready    []byte // verified bytes not yet returned
	finished bool
	err      error
}

// New creates a decoder for the aws-chunked body src. mode is the parsed
// x-amz-content-sha256 value; h holds the request headers; sign is required
// for signed modes.
func New(src io.Reader, mode Mode, h http.Header, sign *Signing, opts Options) (*Reader, error) {
	r := &Reader{src: src, mode: mode, expectedLen: -1, sign: sign}
	if mode.signed() {
		if sign == nil {
			return nil, &Error{"AccessDenied", "Access Denied.", http.StatusForbidden}
		}
		r.prevSig = sign.Signature
	}
	if v := strings.TrimSpace(h.Get("X-Amz-Decoded-Content-Length")); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			return nil, &Error{"InvalidRequest", "Invalid x-amz-decoded-content-length", http.StatusBadRequest}
		}
		r.expectedLen = n
	}
	c, err := newChecksumCheck(h, mode, opts)
	if err != nil {
		return nil, err
	}
	r.check = c
	return r, nil
}

// DecodedLength returns the declared decoded length, or -1.
func (r *Reader) DecodedLength() int64 { return r.expectedLen }

// Close is a no-op so that the Reader can be used as an io.ReadCloser.
func (r *Reader) Close() error { return nil }

func (r *Reader) hmacHex(s string) string {
	m := hmac.New(sha256.New, r.sign.Key)
	m.Write([]byte(s))
	return hex.EncodeToString(m.Sum(nil))
}

func (r *Reader) verifyChunk(dataHashHex string) error {
	if !r.mode.signed() {
		return nil
	}
	if r.chunkSig == "" {
		return errMalformed("missing chunk-signature")
	}
	sts := "AWS4-HMAC-SHA256-PAYLOAD\n" + r.sign.AmzDate + "\n" + r.sign.Scope + "\n" + r.prevSig + "\n" + emptySHA256 + "\n" + dataHashHex
	exp := r.hmacHex(sts)
	if subtle.ConstantTimeCompare([]byte(exp), []byte(r.chunkSig)) != 1 {
		return errSignature()
	}
	r.prevSig = exp
	r.chunkSig = ""
	return nil
}

func (r *Reader) verifyTrailerSig() error {
	provided := r.trailerSig
	if provided == "" {
		return errMalformed("missing x-amz-trailer-signature")
	}
	var canon strings.Builder
	for _, t := range r.trailers {
		canon.WriteString(t.name + ":" + t.value + "\n")
	}
	sum := sha256.Sum256([]byte(canon.String()))
	sts := "AWS4-HMAC-SHA256-TRAILER\n" + r.sign.AmzDate + "\n" + r.sign.Scope + "\n" + r.prevSig + "\n" + hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(r.hmacHex(sts)), []byte(provided)) != 1 {
		return errSignature()
	}
	return nil
}

func (r *Reader) needMore() (step, error) {
	if r.srcDone {
		return 0, errIncomplete("The request body terminated unexpectedly (incomplete aws-chunked encoding)")
	}
	return stepNeedMore, nil
}

// next advances the decoder over the buffered input.
func (r *Reader) next() (piece []byte, s step, err error) {
	for {
		switch r.state {
		case stHeader:
			pos := bytes.Index(r.buf, []byte("\r\n"))
			if pos < 0 {
				if len(r.buf) > maxChunkHeaderLen {
					return nil, 0, errMalformed("chunk header too long")
				}
				s, err = r.needMore()
				return nil, s, err
			}
			if pos > maxChunkHeaderLen {
				return nil, 0, errMalformed("chunk header too long")
			}
			line := string(r.buf[:pos])
			r.buf = r.buf[pos+2:]
			parts := strings.Split(line, ";")
			size, perr := strconv.ParseUint(strings.TrimSpace(parts[0]), 16, 63)
			if perr != nil {
				return nil, 0, errMalformed("invalid chunk size")
			}
			r.chunkSig = ""
			for _, ext := range parts[1:] {
				if v, ok := strings.CutPrefix(strings.TrimSpace(ext), "chunk-signature="); ok {
					r.chunkSig = strings.TrimSpace(v)
					break
				}
			}
			if r.mode.signed() && r.chunkSig == "" {
				return nil, 0, errMalformed("missing chunk-signature")
			}
			if size == 0 {
				if err = r.verifyChunk(emptySHA256); err != nil {
					return nil, 0, err
				}
				if r.mode.hasTrailer() {
					r.state = stTrailer
				} else {
					r.state = stFinalCRLF
				}
			} else {
				r.chunkHash = sha256.New()
				r.left = int64(size)
				r.state = stData
			}
		case stData:
			if len(r.buf) == 0 {
				s, err = r.needMore()
				return nil, s, err
			}
			n := int64(len(r.buf))
			if r.left < n {
				n = r.left
			}
			piece = append([]byte(nil), r.buf[:n]...)
			r.buf = r.buf[n:]
			if r.mode.signed() {
				r.chunkHash.Write(piece)
			}
			r.left -= n
			if r.left == 0 {
				r.state = stDataCRLF
			}
			return piece, stepData, nil
		case stDataCRLF:
			if len(r.buf) < 2 {
				s, err = r.needMore()
				return nil, s, err
			}
			if r.buf[0] != '\r' || r.buf[1] != '\n' {
				return nil, 0, errMalformed("chunk data not terminated by CRLF")
			}
			r.buf = r.buf[2:]
			if r.mode.signed() {
				if err = r.verifyChunk(hex.EncodeToString(r.chunkHash.Sum(nil))); err != nil {
					return nil, 0, err
				}
			}
			r.state = stHeader
		case stFinalCRLF:
			switch {
			case len(r.buf) >= 2:
				if r.buf[0] != '\r' || r.buf[1] != '\n' {
					return nil, 0, errMalformed("unexpected data after final chunk")
				}
				r.buf = r.buf[2:]
				r.state = stDone
			case r.srcDone:
				// some clients omit the terminating CRLF
				if len(r.buf) != 0 {
					return nil, 0, errMalformed("unexpected data after final chunk")
				}
				r.state = stDone
			default:
				return nil, stepNeedMore, nil
			}
		case stTrailer:
			pos := bytes.Index(r.buf, []byte("\r\n"))
			if pos < 0 {
				if r.trailerBytes+len(r.buf) > maxTrailerLen {
					return nil, 0, errMalformed("trailer section too large")
				}
				if r.srcDone {
					// tolerate a missing final empty line, not a truncated line
					if len(r.buf) != 0 {
						return nil, 0, errIncomplete("The request body terminated unexpectedly (incomplete aws-chunked encoding)")
					}
					if err = r.finishTrailer(); err != nil {
						return nil, 0, err
					}
					continue
				}
				return nil, stepNeedMore, nil
			}
			r.trailerBytes += pos + 2
			if r.trailerBytes > maxTrailerLen {
				return nil, 0, errMalformed("trailer section too large")
			}
			line := string(r.buf[:pos])
			r.buf = r.buf[pos+2:]
			if line == "" {
				if err = r.finishTrailer(); err != nil {
					return nil, 0, err
				}
				continue
			}
			name, value, ok := strings.Cut(line, ":")
			if !ok {
				return nil, 0, errMalformed("invalid trailer line")
			}
			name = strings.ToLower(strings.TrimSpace(name))
			value = strings.TrimSpace(value)
			if name == "x-amz-trailer-signature" {
				r.trailerSig = value
			} else {
				r.trailers = append(r.trailers, trailer{name, value})
			}
		case stDone:
			return nil, stepDone, nil
		}
	}
}

func (r *Reader) finishTrailer() error {
	if r.mode == ModeSignedTrailer {
		if err := r.verifyTrailerSig(); err != nil {
			return err
		}
	}
	r.state = stDone
	return nil
}

// produce returns the next decoded piece, or (nil, nil) at end of payload.
func (r *Reader) produce() ([]byte, error) {
	for {
		piece, s, err := r.next()
		if err != nil {
			return nil, err
		}
		switch s {
		case stepData:
			return piece, nil
		case stepDone:
			return nil, nil
		}
		// need more input
		chunk := make([]byte, readSize)
		n, rerr := r.src.Read(chunk)
		r.buf = append(r.buf, chunk[:n]...)
		if rerr == io.EOF {
			r.srcDone = true
		} else if rerr != nil {
			return nil, rerr
		}
	}
}

func (r *Reader) finalChecks() error {
	if r.expectedLen >= 0 && r.expectedLen != r.decodedLen {
		return errIncomplete(fmt.Sprintf("Decoded payload length %d does not match declared length %d", r.decodedLen, r.expectedLen))
	}
	if r.check != nil {
		return r.check.verify(r.trailers)
	}
	return nil
}

// Read implements io.Reader.
func (r *Reader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for len(r.ready) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		if r.finished {
			r.err = io.EOF
			return 0, io.EOF
		}
		piece, err := r.produce()
		switch {
		case err != nil:
			r.fail(err)
		case piece == nil:
			r.finished = true
			if ferr := r.finalChecks(); ferr != nil {
				r.fail(ferr)
			} else {
				r.ready, r.pending = r.pending, nil
			}
		case len(piece) == 0:
		default:
			r.decodedLen += int64(len(piece))
			if r.expectedLen >= 0 && r.decodedLen > r.expectedLen {
				r.fail(errIncomplete(fmt.Sprintf("Payload exceeds declared length %d", r.expectedLen)))
				break
			}
			if r.check != nil {
				r.check.h.Write(piece)
			}
			r.ready, r.pending = r.pending, piece
		}
	}
	n := copy(p, r.ready)
	r.ready = r.ready[n:]
	return n, nil
}

func (r *Reader) fail(err error) {
	if err == io.EOF {
		err = io.ErrUnexpectedEOF
	}
	r.err = err
	r.finished = true
	r.pending, r.ready = nil, nil
}
