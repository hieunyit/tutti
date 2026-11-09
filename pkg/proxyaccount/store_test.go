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

package proxyaccount

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
)

type testSealer struct{ key []byte }

func (s testSealer) Seal(p []byte) (string, error) {
	b, _ := aes.NewCipher(s.key)
	g, _ := cipher.NewGCM(b)
	nonce := make([]byte, g.NonceSize())
	_, _ = rand.Read(nonce)
	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, p, nil)), nil
}

func (s testSealer) Open(v string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, err
	}
	b, _ := aes.NewCipher(s.key)
	g, _ := cipher.NewGCM(b)
	if len(raw) < g.NonceSize() {
		return nil, errors.New("short")
	}
	return g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], nil)
}

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis, *time.Time) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	s := NewStore(rc, func() (Sealer, error) { return testSealer{key: []byte("0123456789abcdef0123456789abcdef")}, nil })
	now := time.Now()
	s.now = func() time.Time { return now }
	return s, mr, &now
}

func TestStore_CreateSealed(t *testing.T) {
	s, mr, _ := newTestStore(t)
	ctx := t.Context()
	acc, secret, err := s.Create(ctx, "alice", "ci")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(acc.AccessKeyID, "TUT"))
	require.Len(t, acc.AccessKeyID, 20)
	require.Equal(t, strings.ToUpper(acc.AccessKeyID), acc.AccessKeyID)
	require.Len(t, secret, 40)
	require.Equal(t, StatusActive, acc.Status)

	// the stored bytes never contain the plaintext secret
	raw := mr.HGet(Key, acc.AccessKeyID)
	require.NotEmpty(t, raw)
	require.NotContains(t, raw, secret)
	require.NotContains(t, mr.Dump(), secret)

	// list has no secrets and sorted metadata
	list, err := s.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "alice", list[0].User)

	creds, skipped, err := s.LoadCredentials(ctx)
	require.NoError(t, err)
	require.Empty(t, skipped)
	require.Equal(t, secret, creds[acc.AccessKeyID].Secret)
	require.Equal(t, []string{secret}, creds[acc.AccessKeyID].Secrets(time.Now()))
}

func TestStore_RefusesWithoutSealer(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	for _, s := range []*Store{
		NewStore(rc, nil),
		NewStore(rc, func() (Sealer, error) { return nil, dom.ErrFailedPrecondition }),
	} {
		_, _, err := s.Create(t.Context(), "alice", "")
		require.ErrorIs(t, err, dom.ErrFailedPrecondition)
	}
	require.False(t, mr.Exists(Key), "nothing must be stored")
}

func TestStore_RotationGrace(t *testing.T) {
	s, _, now := newTestStore(t)
	ctx := t.Context()
	acc, oldSecret, err := s.Create(ctx, "alice", "")
	require.NoError(t, err)

	_, newSecret, err := s.Rotate(ctx, acc.AccessKeyID, time.Hour)
	require.NoError(t, err)
	require.NotEqual(t, oldSecret, newSecret)
	creds, _, err := s.LoadCredentials(ctx)
	require.NoError(t, err)
	c := creds[acc.AccessKeyID]
	require.ElementsMatch(t, []string{newSecret, oldSecret}, c.Secrets(*now))
	require.ElementsMatch(t, []string{newSecret, oldSecret}, c.Secrets(now.Add(59*time.Minute)))
	// after the grace period only the new secret verifies
	require.Equal(t, []string{newSecret}, c.Secrets(now.Add(61*time.Minute)))

	// zero grace: the old secret is gone at once
	_, newer, err := s.Rotate(ctx, acc.AccessKeyID, 0)
	require.NoError(t, err)
	creds, _, err = s.LoadCredentials(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{newer}, creds[acc.AccessKeyID].Secrets(*now))

	_, _, err = s.Rotate(ctx, "TUTMISSING", time.Hour)
	require.ErrorIs(t, err, dom.ErrNotFound)
	_, _, err = s.Rotate(ctx, acc.AccessKeyID, -time.Second)
	require.ErrorIs(t, err, dom.ErrInvalidArg)
}

func TestStore_StatusDelete(t *testing.T) {
	s, _, _ := newTestStore(t)
	ctx := t.Context()
	acc, _, err := s.Create(ctx, "alice", "")
	require.NoError(t, err)
	require.NoError(t, s.SetStatus(ctx, acc.AccessKeyID, StatusDisabled))
	list, _ := s.List(ctx)
	require.Equal(t, StatusDisabled, list[0].Status)
	require.ErrorIs(t, s.SetStatus(ctx, acc.AccessKeyID, "bogus"), dom.ErrInvalidArg)
	require.ErrorIs(t, s.SetStatus(ctx, "TUTMISSING", StatusActive), dom.ErrNotFound)
	require.NoError(t, s.Delete(ctx, acc.AccessKeyID))
	require.ErrorIs(t, s.Delete(ctx, acc.AccessKeyID), dom.ErrNotFound)
}

func TestCache_ReloadsWithinTTL(t *testing.T) {
	s, _, _ := newTestStore(t)
	ctx := t.Context()
	c := NewCache(s, 50*time.Millisecond)
	acc, _, err := s.Create(ctx, "alice", "")
	require.NoError(t, err)
	got, ok := c.Lookup(ctx, acc.AccessKeyID)
	require.True(t, ok)
	require.Equal(t, "alice", got.User)

	require.NoError(t, s.SetStatus(ctx, acc.AccessKeyID, StatusDisabled))
	require.Eventually(t, func() bool {
		got, ok := c.Lookup(ctx, acc.AccessKeyID)
		return ok && got.Status == StatusDisabled
	}, 2*time.Second, 20*time.Millisecond)
	require.NoError(t, s.Delete(ctx, acc.AccessKeyID))
	require.Eventually(t, func() bool { _, ok := c.Lookup(ctx, acc.AccessKeyID); return !ok }, 2*time.Second, 20*time.Millisecond)
}
