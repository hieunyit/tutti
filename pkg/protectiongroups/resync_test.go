// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"iter"
	"slices"
	"testing"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/stretchr/testify/require"
)

type resyncMemory struct {
	data                  map[string]map[string][]byte
	versions              map[string]bool
	puts                  []string
	closes                int
	corrupt, ignoreDelete bool
	readErr               error
	// etags makes list report MD5 ETags; etag overrides the ETag of single keys (e.g. multipart).
	etags    bool
	etag     map[string]string
	gets     map[string]int
	unsorted bool
}

func (m *resyncMemory) buckets(context.Context) ([]string, error) {
	var out []string
	for b := range m.data {
		out = append(out, b)
	}
	slices.Sort(out)
	return out, nil
}
func (m *resyncMemory) bucketExists(_ context.Context, b string) (bool, error) {
	_, ok := m.data[b]
	return ok, nil
}
func (m *resyncMemory) createBucket(_ context.Context, b string) error {
	m.data[b] = map[string][]byte{}
	return nil
}
func (m *resyncMemory) removeBucket(_ context.Context, b string) error {
	if len(m.data[b]) != 0 {
		return errors.New("not empty")
	}
	delete(m.data, b)
	return nil
}
func (m *resyncMemory) list(_ context.Context, b string) iter.Seq2[objstore.CommonObjectInfo, error] {
	return func(yield func(objstore.CommonObjectInfo, error) bool) {
		var keys []string
		for k := range m.data[b] {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		if m.unsorted {
			slices.Reverse(keys)
		}
		for _, k := range keys {
			info := objstore.CommonObjectInfo{Key: k, Size: uint64(len(m.data[b][k]))}
			if m.etags {
				sum := md5.Sum(m.data[b][k])
				info.Etag = `"` + hex.EncodeToString(sum[:]) + `"`
			}
			if e, ok := m.etag[k]; ok {
				info.Etag = e
			}
			if !yield(info, nil) {
				return
			}
		}
	}
}
func (m *resyncMemory) exists(_ context.Context, b, k string) (bool, error) {
	_, ok := m.data[b][k]
	return ok, nil
}
func (m *resyncMemory) remove(_ context.Context, b, k string) error {
	if !m.ignoreDelete {
		delete(m.data[b], k)
	}
	return nil
}

type resyncReadCloser struct {
	io.Reader
	close func()
}

func (r resyncReadCloser) Close() error { r.close(); return nil }
func (m *resyncMemory) get(_ context.Context, b, k string) (io.Reader, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	data, ok := m.data[b][k]
	if !ok {
		return nil, dom.ErrNotFound
	}
	if m.gets == nil {
		m.gets = map[string]int{}
	}
	m.gets[k]++
	return resyncReadCloser{Reader: bytes.NewReader(data), close: func() { m.closes++ }}, nil
}
func (m *resyncMemory) put(_ context.Context, b, k string, r io.Reader, n uint64) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if uint64(len(data)) != n {
		return io.ErrUnexpectedEOF
	}
	if m.corrupt {
		data = append(data, 0)
	}
	m.data[b][k] = data
	m.puts = append(m.puts, b+"/"+k)
	return nil
}
func (m *resyncMemory) endpoint() resyncEndpoint {
	return resyncEndpoint{objects: m, versioned: func(_ context.Context, b string) (bool, error) { return m.versions[b], nil }}
}
func resyncFixture() (*S3Resyncer, *resyncMemory, *resyncMemory, Group) {
	source := &resyncMemory{data: map[string]map[string][]byte{"app": {"same": []byte("keep"), "changed": []byte("new"), "fresh": []byte("added")}}, versions: map[string]bool{}}
	target := &resyncMemory{data: map[string]map[string][]byte{"app": {"same": []byte("keep"), "changed": []byte("old"), "deleted": []byte("gone")}, "removed-bucket": {"old": []byte("old")}}, versions: map[string]bool{}}
	r := &S3Resyncer{open: func(_ context.Context, storage, user string) (resyncEndpoint, error) {
		if storage == "a" {
			return source.endpoint(), nil
		}
		return target.endpoint(), nil
	}}
	return r, source, target, Group{ID: "g", User: "tenant", Primary: "a", Members: []string{"a", "b"}, Copies: 2, Frozen: true}
}
func TestResyncExactCurrentObjectsAndDeletion(t *testing.T) {
	r, source, target, g := resyncFixture()
	require.NoError(t, r.Resync(t.Context(), g, "b"))
	require.Equal(t, source.data, target.data)
	require.ElementsMatch(t, []string{"app/changed", "app/fresh"}, target.puts, "matching objects must be adopted without rewriting")
	require.Greater(t, source.closes, 0)
	require.Greater(t, target.closes, 0)
}
func TestResyncSharedPolicyScope(t *testing.T) {
	r, source, target, g := resyncFixture()
	source.data["private"] = map[string][]byte{"secret": []byte("source secret")}
	target.data["private"] = map[string][]byte{"secret": []byte("target secret")}
	r.scope = &accesspolicy.Scope{Shared: func(string, string) bool { return true }, Get: func(context.Context, string) (accesspolicy.Policy, error) {
		return accesspolicy.Policy{User: g.User, Permission: accesspolicy.ReadWrite, Buckets: []string{"app"}}, nil
	}}
	require.NoError(t, r.Resync(t.Context(), g, "b"))
	require.Equal(t, source.data["app"], target.data["app"])
	require.Equal(t, []byte("target secret"), target.data["private"]["secret"])
	require.Contains(t, target.data, "removed-bucket", "outside policy must remain untouched")
	g.Buckets = []string{"private"}
	require.ErrorIs(t, r.Resync(t.Context(), g, "b"), dom.ErrFailedPrecondition)
}
func TestResyncRejectsVersionedBeforeMutating(t *testing.T) {
	for _, storage := range []string{"source", "target"} {
		t.Run(storage, func(t *testing.T) {
			r, src, dst, g := resyncFixture()
			if storage == "source" {
				src.versions["app"] = true
			} else {
				dst.versions["removed-bucket"] = true
			}
			require.ErrorIs(t, r.Validate(t.Context(), g), dom.ErrFailedPrecondition)
			require.ErrorIs(t, r.Resync(t.Context(), g, "b"), dom.ErrFailedPrecondition)
			require.Empty(t, dst.puts)
			require.Contains(t, dst.data, "removed-bucket")
			require.Equal(t, []byte("old"), dst.data["app"]["changed"])
		})
	}
}
func TestResyncFailsClosedOnCorruptionOrUnreadableSource(t *testing.T) {
	t.Run("corrupt target", func(t *testing.T) {
		r, _, dst, g := resyncFixture()
		dst.corrupt = true
		require.ErrorIs(t, r.Resync(t.Context(), g, "b"), dom.ErrFailedPrecondition)
	})
	t.Run("source unreadable", func(t *testing.T) {
		r, src, dst, g := resyncFixture()
		src.readErr = io.ErrUnexpectedEOF
		require.ErrorIs(t, r.Resync(t.Context(), g, "b"), io.ErrUnexpectedEOF)
		require.Contains(t, dst.data["app"], "deleted")
	})
	t.Run("unfrozen", func(t *testing.T) {
		r, _, dst, g := resyncFixture()
		g.Frozen = false
		require.ErrorIs(t, r.Resync(t.Context(), g, "b"), dom.ErrFailedPrecondition)
		require.Empty(t, dst.puts)
	})
	t.Run("delete ignored", func(t *testing.T) {
		r, _, dst, g := resyncFixture()
		dst.ignoreDelete = true
		require.ErrorIs(t, r.Resync(t.Context(), g, "b"), dom.ErrFailedPrecondition)
	})
}

// NEW-03: matching objects are decided from listing metadata without reading them; inconclusive
// (multipart) ETags are hashed once per side; copied objects are verified from the streamed digest.
func TestResyncComparesMetadataFirstAndReadsEachObjectAtMostOnce(t *testing.T) {
	r, source, target, g := resyncFixture()
	source.etags, target.etags = true, true
	source.data["app"]["big"] = bytes.Repeat([]byte("x"), 1000)
	target.data["app"]["big"] = bytes.Repeat([]byte("x"), 1000)
	source.data["app"]["multi"] = []byte("same content")
	target.data["app"]["multi"] = []byte("same content")
	source.etag = map[string]string{"multi": `"0123456789abcdef0123456789abcdef-2"`}
	target.etag = map[string]string{"multi": `"fedcba9876543210fedcba9876543210-3"`}
	require.NoError(t, r.Resync(t.Context(), g, "b"))
	require.Equal(t, source.data, target.data)
	require.ElementsMatch(t, []string{"app/changed", "app/fresh"}, target.puts)
	require.Zero(t, source.gets["same"]+target.gets["same"]+source.gets["big"]+target.gets["big"], "equal size+ETag must not be read")
	require.Equal(t, 1, source.gets["multi"], "inconclusive multipart ETags: hash once per side")
	require.Equal(t, 1, target.gets["multi"])
	require.Equal(t, 1, source.gets["changed"], "different single-part ETags: copy without hashing first")
	require.Equal(t, 1, source.gets["fresh"])
	require.LessOrEqual(t, target.gets["changed"], 1)
}

func TestResyncSizeMismatchCopiesWithoutHashing(t *testing.T) {
	r, source, target, g := resyncFixture()
	target.data["app"]["changed"] = []byte("older value")
	require.NoError(t, r.Resync(t.Context(), g, "b"))
	require.Equal(t, source.data, target.data)
	require.Equal(t, 1, source.gets["changed"])
}

func TestResyncFailsClosedOnUnsortedListing(t *testing.T) {
	for _, side := range []string{"source", "target"} {
		t.Run(side, func(t *testing.T) {
			r, source, target, g := resyncFixture()
			if side == "source" {
				source.unsorted = true
			} else {
				target.unsorted = true
			}
			require.ErrorIs(t, r.Resync(t.Context(), g, "b"), dom.ErrFailedPrecondition)
			require.Contains(t, target.data["app"], "same", "no deletion decided from an unordered listing")
			require.Contains(t, target.data["app"], "deleted", "deletions wait for a complete ordered pass")
		})
	}
}

func TestMD5ETag(t *testing.T) {
	for in, want := range map[string]bool{
		`"d41d8cd98f00b204e9800998ecf8427e"`:   true,
		`D41D8CD98F00B204E9800998ECF8427E`:     true,
		`"d41d8cd98f00b204e9800998ecf8427e-7"`: true,
		`"d41d8cd98f00b204e9800998ecf8427e-"`:  false,
		`"abc"`:                                false,
		``:                                     false,
		`"zz1d8cd98f00b204e9800998ecf8427e"`:   false,
	} {
		_, ok := md5ETag(in)
		require.Equal(t, want, ok, in)
	}
}
