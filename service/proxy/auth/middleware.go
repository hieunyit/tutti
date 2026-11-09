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
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/xml"
	"net/http"
	"time"

	mclient "github.com/minio/minio-go/v7"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/log"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/proxyaccount"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3/chunked"
	"github.com/hieunyit/tutti/pkg/util"
)

// AccountLookup resolves a runtime-managed (dynamic) proxy account by access key id.
type AccountLookup func(ctx context.Context, accessKeyID string) (proxyaccount.Credential, bool)

// Option customizes the auth middleware.
type Option func(*middleware)

// WithAccounts enables runtime-managed proxy accounts. They have the lowest precedence:
// auth.custom, then auth.useStorage credentials, then dynamic accounts.
func WithAccounts(lookup AccountLookup) Option {
	return func(m *middleware) { m.accounts = lookup }
}

func Middleware(conf *Config, credsSvc objstore.CredsService, endpointAddress string, opts ...Option) *middleware {
	custom := map[string]credMeta{}
	for user, cred := range conf.Custom {
		custom[cred.AccessKeyID] = credMeta{
			cred: cred,
			user: user,
		}
	}
	m := &middleware{
		allowV2:     conf.AllowV2Signature,
		signedHdrs:  s3.SignedHeaderOpts{RequireSignedContentType: conf.RequireSignedContentType},
		custom:      custom,
		storageName: conf.UseStorage,
		credsSvc:    credsSvc,
		endpoint:    endpointAddress,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

type credMeta struct {
	cred s3.CredentialsV4
	user string
	// alt are additional credentials (same access key id, other secret) that also verify:
	// the previous secret of a dynamic account during the rotation grace period.
	alt []s3.CredentialsV4
}

type middleware struct {
	allowV2     bool
	signedHdrs  s3.SignedHeaderOpts
	custom      map[string]credMeta
	storageName string
	credsSvc    objstore.CredsService
	endpoint    string
	accounts    AccountLookup
}

func (m *middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, payloadHash, sig, err := m.authenticate(r)
		if err != nil {
			util.WriteError(r.Context(), w, err)
			return
		}
		// The proxy re-signs the request for the storage in pkg/s3client, so the
		// payload hash the signature was verified against must be declared as a
		// header: it is part of the new signature, and the storage would
		// otherwise fall back to sha256("") and reject it.
		r.Header.Set(s3.AmzContentSha256, payloadHash)
		if chunked.IsStreaming(payloadHash) {
			if err = m.decodeStreamingBody(r, payloadHash, sig); err != nil {
				util.WriteError(r.Context(), w, err)
				return
			}
		}
		if !isRequestSignatureV4(r) && isRequestPresignedSignatureV4(r) {
			// the presigned query-string auth params were validated:
			// strip them so that the request forwarded to the backend
			// carries only the Authorization header computed by the proxy.
			removePresignParams(r)
		}
		ctx := log.WithUser(r.Context(), user)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

var authDeniedErr = mclient.ErrorResponse{
	XMLName:    xml.Name{},
	Code:       "InvalidAccessKeyId",
	Message:    "The AWS access key ID that you provided does not exist in our records.",
	StatusCode: http.StatusForbidden,
}

// getCred resolves the credentials of an access key id. Precedence: auth.custom, then auth.useStorage credentials,
// then dynamic accounts.
func (m *middleware) getCred(accessKey string) (credMeta, error) {
	// check custom creds
	if res, ok := m.custom[accessKey]; ok {
		return res, nil
	}
	if m.storageName != "" {
		// check storage creds
		user, cred, err := m.credsSvc.FindS3Credentials(m.storageName, accessKey)
		if err == nil {
			return credMeta{cred: cred, user: user}, nil
		}
	}
	if m.accounts != nil {
		// a disabled account is indistinguishable from an unknown one
		if acc, ok := m.accounts(context.Background(), accessKey); ok && acc.Status == proxyaccount.StatusActive {
			secrets := acc.Secrets(time.Now())
			res := credMeta{user: acc.User, cred: s3.CredentialsV4{AccessKeyID: accessKey, SecretAccessKey: secrets[0]}}
			for _, sec := range secrets[1:] {
				res.alt = append(res.alt, s3.CredentialsV4{AccessKeyID: accessKey, SecretAccessKey: sec})
			}
			return res, nil
		}
	}
	return credMeta{}, authDeniedErr
}

// isReqAuthenticated verifies the request signature and returns the
// authenticated user together with the payload hash the signature was
// verified against.
func (m *middleware) isReqAuthenticated(r *http.Request) (string, string, error) {
	user, hash, _, err := m.authenticate(r)
	return user, hash, err
}

// authenticate is isReqAuthenticated that also returns the signing material
// of a SigV4 header signature (nil for any other kind of authentication).
func (m *middleware) authenticate(r *http.Request) (string, string, *signingInfo, error) {
	switch {
	case isRequestSignatureV4(r):
		sha256sum := getContentSha256Cksum(r)
		user, sig, err := m.doesSignatureV4Match(sha256sum, r)
		return user, sha256sum, sig, err
	case isRequestPresignedSignatureV4(r):
		user, hash, err := m.doesPresignedSignatureV4Match(r)
		return user, hash, nil, err
	case m.allowV2 && isRequestSignatureV2(r):
		// a V2 signature does not cover the payload
		user, err := m.doesSignatureV2Match(r)
		return user, unsignedPayload, nil, err
	}
	return "", unsignedPayload, nil, mclient.ErrorResponse{
		XMLName:    xml.Name{},
		Code:       "CredentialsNotSupported",
		Message:    "This request does not support given credentials type.",
		BucketName: xctx.GetBucket(r.Context()),
		Key:        xctx.GetObject(r.Context()),
		StatusCode: http.StatusBadRequest,
	}
}

func getContentSha256Cksum(r *http.Request) string {
	var (
		defaultSha256Cksum string
		v                  []string
		ok                 bool
	)
	defaultSha256Cksum = emptySHA256
	v, ok = r.Header[s3.AmzContentSha256]
	if ok {
		return v[0]
	}
	return defaultSha256Cksum
}

// Streaming AWS Signature Version '4' constants.
const (
	emptySHA256            = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	unsignedPayload        = "UNSIGNED-PAYLOAD"
	unsignedPayloadTrailer = "STREAMING-UNSIGNED-PAYLOAD-TRAILER"
)

func sumHMAC(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}
