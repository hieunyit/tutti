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

package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	mclient "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/signer"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/proxyaccount"
	"github.com/hieunyit/tutti/pkg/s3"
)

type aesSealer struct{ key []byte }

func (s aesSealer) Seal(p []byte) (string, error) {
	b, _ := aes.NewCipher(s.key)
	g, _ := cipher.NewGCM(b)
	nonce := make([]byte, g.NonceSize())
	_, _ = rand.Read(nonce)
	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, p, nil)), nil
}

func (s aesSealer) Open(v string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, err
	}
	b, _ := aes.NewCipher(s.key)
	g, _ := cipher.NewGCM(b)
	return g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
}

func signedRequest(accessKey, secret string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/bucket/object", nil)
	req.Host = "s3.example.com:9669"
	req.Header.Set(s3.AmzContentSha256, emptySHA256)
	return signer.SignV4(*req, accessKey, secret, "", "us-east-1")
}

func authStatus(t *testing.T, m *middleware, req *http.Request) (int, string) {
	t.Helper()
	var user string
	rec := httptest.NewRecorder()
	m.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user = "ok"
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	return rec.Code, user
}

func TestDynamicAccounts(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	store := proxyaccount.NewStore(rc, func() (proxyaccount.Sealer, error) {
		return aesSealer{key: []byte("0123456789abcdef0123456789abcdef")}, nil
	})
	// ttl 1ms: every lookup sees the latest state
	cache := proxyaccount.NewCache(store, time.Millisecond)
	m := &middleware{
		endpoint: "http://s3.example.com:9669",
		custom: map[string]credMeta{
			"STATIC": {cred: s3.CredentialsV4{AccessKeyID: "STATIC", SecretAccessKey: "static-secret"}, user: "static-user"},
		},
		accounts: cache.Lookup,
	}
	ctx := t.Context()
	acc, secret, err := store.Create(ctx, "alice", "")
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)

	code, _ := authStatus(t, m, signedRequest(acc.AccessKeyID, secret))
	require.Equal(t, http.StatusOK, code)
	code, _ = authStatus(t, m, signedRequest(acc.AccessKeyID, "wrong-secret"))
	require.Equal(t, http.StatusForbidden, code)
	// user is the mapped storage user
	got, err := m.getCred(acc.AccessKeyID)
	require.NoError(t, err)
	require.Equal(t, "alice", got.user)

	// rotation: both secrets verify during grace, old fails after an immediate rotation
	_, newSecret, err := store.Rotate(ctx, acc.AccessKeyID, time.Hour)
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	for _, sec := range []string{secret, newSecret} {
		code, _ = authStatus(t, m, signedRequest(acc.AccessKeyID, sec))
		require.Equal(t, http.StatusOK, code, "secret within grace")
	}
	_, newest, err := store.Rotate(ctx, acc.AccessKeyID, 0)
	require.NoError(t, err)
	time.Sleep(5 * time.Millisecond)
	code, _ = authStatus(t, m, signedRequest(acc.AccessKeyID, newSecret))
	require.Equal(t, http.StatusForbidden, code)
	code, _ = authStatus(t, m, signedRequest(acc.AccessKeyID, newest))
	require.Equal(t, http.StatusOK, code)

	// presigned URLs work with dynamic accounts too
	preq := httptest.NewRequest(http.MethodGet, "/bucket/object", nil)
	preq.Host = "s3.example.com:9669"
	preq = signer.PreSignV4(*preq, acc.AccessKeyID, newest, "", "us-east-1", 3600)
	code, _ = authStatus(t, m, preq)
	require.Equal(t, http.StatusOK, code)

	// disabled: rejected as an unknown key
	require.NoError(t, store.SetStatus(ctx, acc.AccessKeyID, proxyaccount.StatusDisabled))
	time.Sleep(5 * time.Millisecond)
	_, _, _, err = m.authenticate(signedRequest(acc.AccessKeyID, newest))
	var s3Err mclient.ErrorResponse
	require.ErrorAs(t, err, &s3Err)
	require.Equal(t, "InvalidAccessKeyId", s3Err.Code)
	require.Equal(t, http.StatusForbidden, s3Err.StatusCode)

	// re-enabled, then deleted
	require.NoError(t, store.SetStatus(ctx, acc.AccessKeyID, proxyaccount.StatusActive))
	time.Sleep(5 * time.Millisecond)
	code, _ = authStatus(t, m, signedRequest(acc.AccessKeyID, newest))
	require.Equal(t, http.StatusOK, code)
	require.NoError(t, store.Delete(ctx, acc.AccessKeyID))
	time.Sleep(5 * time.Millisecond)
	code, _ = authStatus(t, m, signedRequest(acc.AccessKeyID, newest))
	require.Equal(t, http.StatusForbidden, code)

	// static custom credentials keep working and take precedence
	code, _ = authStatus(t, m, signedRequest("STATIC", "static-secret"))
	require.Equal(t, http.StatusOK, code)
}

func TestDynamicAccounts_Precedence(t *testing.T) {
	m := &middleware{
		custom: map[string]credMeta{"K": {cred: s3.CredentialsV4{AccessKeyID: "K", SecretAccessKey: "s"}, user: "custom-user"}},
		accounts: func(_ context.Context, id string) (proxyaccount.Credential, bool) {
			c := proxyaccount.Credential{Secret: "dyn"}
			c.AccessKeyID, c.User, c.Status = id, "dynamic-user", proxyaccount.StatusActive
			return c, true
		},
	}
	got, err := m.getCred("K")
	require.NoError(t, err)
	require.Equal(t, "custom-user", got.user)
	got, err = m.getCred("OTHER")
	require.NoError(t, err)
	require.Equal(t, "dynamic-user", got.user)
}
