package copy

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/s3"
)

const markerHeader = "X-Amz-Meta-Tutti-Src-Etag"

func TestMultipartETagParts(t *testing.T) {
	h := strings.Repeat("a", 32)
	for in, want := range map[string]int{h + "-29": 29, `"` + h + `-3"`: 3, `W/"` + h + `-2"`: 2, h + "-1": 1} {
		n, ok := multipartETagParts(in)
		require.True(t, ok, in)
		require.Equal(t, want, n, in)
	}
	for _, in := range []string{"", h, `"` + h + `"`, h + "-", h + "-0", h + "-x", "abc-3", strings.Repeat("g", 32) + "-3", h + "1-3"} {
		_, ok := multipartETagParts(in)
		require.False(t, ok, in)
	}
}

func TestInferPartSize(t *testing.T) {
	const M = int64(mib)
	tests := []struct {
		name string
		size int64
		n    int
		want int64 // 0 = no inference
	}{
		// real observations: 15 MiB parts (s3cmd) are consistent with all three and win over 16 MiB
		{"448698984/29 only 15 MiB fits", 448698984, 29, 15 * M},
		{"151769090/10 15 and 16 MiB fit, 15 preferred", 151769090, 10, 15 * M},
		{"44325999/3 15..21 MiB fit, 15 preferred", 44325999, 3, 15 * M},
		{"16 MiB is the only candidate", 155 * M, 10, 16 * M},
		{"8 MiB (aws cli)", 8*M*4 + 100, 5, 8 * M},
		{"5 MiB minimum", 5*M*2 + 1, 3, 5 * M},
		{"exactly full last part", 3 * 15 * M, 3, 15 * M},
		{"one byte into the last part", 2*15*M + 1, 3, 15 * M},
		{"irregular 12345678 parts: 8 MiB is preferred", 2*12_345_678 + 99, 3, 8 * M},
		{"smallest MiB when no preferred size fits", 19*6*M + 1, 20, 6 * M},
		{"N=1 is not inferred", 100 * M, 1, 0},
		{"N=0", 100 * M, 0, 0},
		{"size 0", 0, 3, 0},
		{"N above 10000", 100 * 1024 * M, 10001, 0},
		{"part below 5 MiB", 2*4*M + 1, 3, 0},
		{"part above 128 MiB", 200 * M * 3, 3, 0},
		{"N=10000 with 5 MiB parts", 5*M*9999 + 1, 10000, 5 * M},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, ok := inferPartSize(tt.size, tt.n)
			require.Equal(t, tt.want != 0, ok)
			require.Equal(t, tt.want, p)
			if ok {
				require.EqualValues(t, tt.n, (tt.size+p-1)/p, "the inferred part size reproduces the part count")
			}
		})
	}
	// whole-MB fallback: p in [lo, hi] = [5_100_001, 5_200_000] holds no whole MiB (5 MiB = 5_242_880) but 5_200_000 is 5.2 MB
	p, ok := inferPartSize(2*5_200_000, 3)
	require.False(t, ok, "5.2e6 is not a whole MB and no MiB fits")
	p, ok = inferPartSize(5_000_000+1, 2) // p in [2_500_001, 5_000_000]: below the 5 MiB minimum
	require.False(t, ok)
	_ = p
}

func TestInferPartSizeExhaustive(t *testing.T) {
	// for every part size and size, the inferred size (if any) yields the same part count
	for n := 2; n < 40; n++ {
		for _, size := range []int64{int64(n-1)*5*mib + 1, int64(n) * 5 * mib, int64(n)*15*mib - 7, int64(n)*16*mib + 0, int64(n-1)*100*mib + 3} {
			if p, ok := inferPartSize(size, n); ok {
				require.EqualValues(t, n, (size+p-1)/p, "size %d n %d p %d", size, n, p)
				require.GreaterOrEqual(t, p, int64(5*mib))
			}
		}
	}
}

// ---- end to end ----

// multipartETag is the S3 ETag of data uploaded with parts of partSize.
func multipartETag(data []byte, partSize int) string {
	var sums []byte
	n := 0
	for off := 0; off < len(data); off += partSize {
		s := md5.Sum(data[off:min(off+partSize, len(data))])
		sums = append(sums, s[:]...)
		n++
	}
	s := md5.Sum(sums)
	return fmt.Sprintf("%s-%d", hex.EncodeToString(s[:]), n)
}

func (e *copyEnv) putMultipart(s *memS3, name string, data []byte, partSize int, h http.Header) string {
	s.Put("bucket/"+name, data, h)
	o, _ := s.Get("bucket/" + name)
	o.etag = multipartETag(data, partSize)
	return o.etag
}

func (e *copyEnv) destETag(name string) string {
	o, ok := e.dst.Get("bucket/" + name)
	require.True(e.t, ok)
	return o.etag
}

// transfers counts the bytes and requests that actually move data.
func (e *copyEnv) transfers() (gets int64, putBytes int64, dstWrites int) {
	return e.src.gets.Load(), e.dst.putBytes.Load(), e.dst.CountRequests("PUT") + e.dst.CountRequests("POST")
}

func engineEnv(t *testing.T, engine string, threads int) *copyEnv {
	return newCopyEnv(t, controlplane.SystemWorkerSettings{CopyEngine: engine, MultipartThreads: threads, MultipartPartSizeMiB: 16}, features.Config{})
}

func requireSecondRunSkips(t *testing.T, e *copyEnv, name string) {
	t.Helper()
	g, b, w := e.transfers()
	require.NoError(t, e.copy(t.Context(), name))
	g2, b2, w2 := e.transfers()
	require.Equal(t, g, g2, "no source GET on the second run")
	require.Equal(t, b, b2, "no byte uploaded on the second run")
	require.Equal(t, w, w2, "no PUT/POST on the second run")
}

func engines() map[string]func(*testing.T) *copyEnv {
	return map[string]func(*testing.T) *copyEnv{
		"legacy":          func(t *testing.T) *copyEnv { return engineEnv(t, "legacy", 1) },
		"legacy-parallel": func(t *testing.T) *copyEnv { return engineEnv(t, "legacy", 3) },
		"pipeline":        func(t *testing.T) *copyEnv { return engineEnv(t, "pipeline", 2) },
	}
}

func TestCopyPreservesMultipartETag15MiB(t *testing.T) {
	for name, mk := range engines() {
		t.Run(name, func(t *testing.T) {
			e := mk(t)
			data := patterned(2*15*mib+1234567, 3) // 3 parts of 15 MiB
			want := e.putMultipart(e.src, "o", data, 15*mib, http.Header{"Content-Type": {"application/x-bak"}})
			require.True(t, strings.HasSuffix(want, "-3"))
			require.NoError(t, e.copy(t.Context(), "o"))
			require.Equal(t, data, e.destBytes("o"))
			require.Equal(t, want, e.destETag("o"), "destination ETag equals the source ETag")
			requireSecondRunSkips(t, e, "o")
		})
	}
}

func TestCopyPreservesMultipartETag16MiB(t *testing.T) {
	for name, mk := range engines() {
		t.Run(name, func(t *testing.T) {
			e := mk(t)
			data := patterned(155*mib, 4) // 10 parts of 16 MiB; 15 MiB would give 11
			want := e.putMultipart(e.src, "o", data, 16*mib, nil)
			require.True(t, strings.HasSuffix(want, "-10"))
			require.NoError(t, e.copy(t.Context(), "o"))
			require.Equal(t, want, e.destETag("o"))
			requireSecondRunSkips(t, e, "o")
		})
	}
}

func TestIrregularPartSizeSkipsThroughMarker(t *testing.T) {
	for name, mk := range engines() {
		t.Run(name, func(t *testing.T) {
			e := mk(t)
			data := patterned(2*12_345_678+99, 5)
			want := e.putMultipart(e.src, "o", data, 12_345_678, nil)
			require.NoError(t, e.copy(t.Context(), "o"))
			require.Equal(t, data, e.destBytes("o"))
			require.NotEqual(t, want, e.destETag("o"), "an irregular layout cannot be reproduced")
			o, _ := e.dst.Get("bucket/o")
			require.Equal(t, want, o.header.Get(markerHeader), "the marker records the source ETag")
			requireSecondRunSkips(t, e, "o")

			// a changed source (same size, new ETag) is copied again
			data2 := patterned(len(data), 6)
			e.putMultipart(e.src, "o", data2, 12_345_678, nil)
			require.NoError(t, e.copy(t.Context(), "o"))
			require.Equal(t, data2, e.destBytes("o"))
		})
	}
}

func TestMarkerOffDoesNotRecordNorSkip(t *testing.T) {
	SetRecordSourceETag(false)
	t.Cleanup(func() { SetRecordSourceETag(true) })
	e := engineEnv(t, "legacy", 1)
	data := patterned(2*12_345_678+99, 5)
	e.putMultipart(e.src, "o", data, 12_345_678, nil)
	require.NoError(t, e.copy(t.Context(), "o"))
	o, _ := e.dst.Get("bucket/o")
	require.Empty(t, o.header.Get(markerHeader))
	g, _, _ := e.transfers()
	require.NoError(t, e.copy(t.Context(), "o"))
	g2, _, _ := e.transfers()
	require.Greater(t, g2, g, "without the marker an irregular layout is copied again")
}

func TestMarkerIsInternal(t *testing.T) {
	e := engineEnv(t, "legacy", 1)
	data := patterned(2*12_345_678+99, 5)
	src := e.putMultipart(e.src, "o", data, 12_345_678, http.Header{"X-Amz-Meta-Owner": {"alice"}})
	require.NoError(t, e.copy(t.Context(), "o"))
	dstETag := e.destETag("o")

	// reverse copy (dest -> source name "back"): the marker of the destination is not copied, the new one is written
	require.NoError(t, e.svc.CopyObject(t.Context(), "u", NewFile("dest", "bucket", "o"), NewFile("source", "bucket", "back")))
	back, _ := e.src.Get("bucket/back")
	require.Equal(t, dstETag, back.header.Get(markerHeader), "records the ETag of ITS source, not the old marker")
	require.NotEqual(t, src, back.header.Get(markerHeader))
	require.Equal(t, "alice", back.header.Get("X-Amz-Meta-Owner"))

	// sameMetadata ignores the key on either side
	a := minio.ObjectInfo{UserMetadata: map[string]string{"Owner": "alice"}}
	b := minio.ObjectInfo{UserMetadata: map[string]string{"Owner": "alice", s3.TuttiSrcETagMeta: "x"}}
	require.True(t, sameMetadata(a, b, false))
	require.True(t, sameMetadata(b, a, false))
	b.UserMetadata["Owner"] = "bob"
	require.False(t, sameMetadata(a, b, false))
	require.NotContains(t, userMetaWithoutMarker(b.UserMetadata), s3.TuttiSrcETagMeta)
}

func TestSinglePartObjectsUnchanged(t *testing.T) {
	for name, mk := range engines() {
		t.Run(name, func(t *testing.T) {
			e := mk(t)
			data := patterned(1<<20+5, 7)
			e.putSource("o", data, nil)
			require.NoError(t, e.copy(t.Context(), "o"))
			src, _ := e.src.Get("bucket/o")
			require.Equal(t, src.etag, e.destETag("o"))
			require.Zero(t, e.dst.CountRequests("POST"), "a small object is one PUT")
			requireSecondRunSkips(t, e, "o")
		})
	}
}

func TestPartSizeEdgeCases(t *testing.T) {
	for name, mk := range engines() {
		t.Run(name, func(t *testing.T) {
			e := mk(t)
			// empty object
			e.putSource("empty", nil, nil)
			require.NoError(t, e.copy(t.Context(), "empty"))
			requireSecondRunSkips(t, e, "empty")

			// a source ETag claiming 1 part, and one that cannot be a real multipart layout, still copy fine
			for i, etag := range []string{strings.Repeat("a", 32) + "-1", strings.Repeat("b", 32) + "-9999", strings.Repeat("c", 32) + "-3"} {
				key := fmt.Sprintf("odd%d", i)
				data := patterned(300_000, byte(i))
				e.putSource(key, data, nil)
				o, _ := e.src.Get("bucket/" + key)
				o.etag = etag
				require.NoError(t, e.copy(t.Context(), key), etag)
				require.Equal(t, data, e.destBytes(key))
				requireSecondRunSkips(t, e, key)
			}
		})
	}
}

func TestOldLayoutDestinationIsNotSkippedUnlessTrusted(t *testing.T) {
	data := patterned(2*15*mib+mib+100, 9) // 15 MiB -> 3 parts, 16 MiB -> 2 parts
	seed := func(t *testing.T) *copyEnv {
		e := engineEnv(t, "legacy", 1)
		e.putMultipart(e.src, "o", data, 15*mib, nil)
		e.putMultipart(e.dst, "o", data, 16*mib, nil) // copied by an older worker: no marker, other part count
		require.True(t, strings.HasSuffix(e.destETag("o"), "-2"))
		return e
	}
	t.Run("default: copied once more", func(t *testing.T) {
		e := seed(t)
		g, _, _ := e.transfers()
		require.NoError(t, e.copy(t.Context(), "o"))
		g2, _, _ := e.transfers()
		require.Greater(t, g2, g)
		src, _ := e.src.Get("bucket/o")
		require.Equal(t, src.etag, e.destETag("o"), "the copy is now ETag-identical")
		requireSecondRunSkips(t, e, "o")
	})
	t.Run("trustSameSizeMultipart skips", func(t *testing.T) {
		SetTrustSameSizeMultipart(true)
		t.Cleanup(func() { SetTrustSameSizeMultipart(false) })
		e := seed(t)
		d, _ := e.dst.Get("bucket/o")
		d.modified = time.Now().Add(time.Hour).UTC().Truncate(time.Second)
		requireSecondRunSkips(t, e, "o")
	})
	t.Run("trustSameSizeMultipart needs a destination not older than the source", func(t *testing.T) {
		SetTrustSameSizeMultipart(true)
		t.Cleanup(func() { SetTrustSameSizeMultipart(false) })
		e := seed(t)
		d, _ := e.dst.Get("bucket/o")
		d.modified = time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
		g, _, _ := e.transfers()
		require.NoError(t, e.copy(t.Context(), "o"))
		g2, _, _ := e.transfers()
		require.Greater(t, g2, g)
	})
	t.Run("trust never skips a different size", func(t *testing.T) {
		SetTrustSameSizeMultipart(true)
		t.Cleanup(func() { SetTrustSameSizeMultipart(false) })
		e := seed(t)
		d, _ := e.dst.Get("bucket/o")
		d.modified = time.Now().Add(time.Hour).UTC().Truncate(time.Second)
		e.putMultipart(e.dst, "o", data[:len(data)-1], 16*mib, nil)
		d, _ = e.dst.Get("bucket/o")
		d.modified = time.Now().Add(time.Hour).UTC().Truncate(time.Second)
		g, _, _ := e.transfers()
		require.NoError(t, e.copy(t.Context(), "o"))
		g2, _, _ := e.transfers()
		require.Greater(t, g2, g)
	})
}

func TestResumeRecordUsesTheInferredPartSize(t *testing.T) {
	e, st := resumeEnv(t, 1)
	data := patterned(6*testPart+777, 11) // 7 parts of 5 MiB
	e.putMultipart(e.src, "o", data, testPart, nil)
	ctx, cancel := context.WithCancel(t.Context())
	e.dst.Hook = crashAtPart(4, cancel)
	require.Error(t, e.copy(resumeCtx(ctx, st), "o"))
	cancel()

	var rec resumeRecord
	var found bool
	st.mu.Lock()
	for _, v := range st.data {
		require.NoError(t, json.Unmarshal(v, &rec))
		found = true
	}
	st.mu.Unlock()
	require.True(t, found)
	require.EqualValues(t, testPart, rec.PartSize)

	e.dst.Hook = nil
	before := e.dst.partsPut.Load()
	require.NoError(t, e.copy(resumeCtx(t.Context(), st), "o"))
	require.EqualValues(t, 4, e.dst.partsPut.Load()-before, "resumes with the same layout")
	src, _ := e.src.Get("bucket/o")
	require.Equal(t, src.etag, e.destETag("o"))
}
