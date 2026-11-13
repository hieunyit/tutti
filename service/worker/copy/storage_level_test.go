package copy

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/s3client"
)

// userRecordingClients records the user every client lookup is made with.
type userRecordingClients struct {
	comprehensiveClients
	mu    sync.Mutex
	users []string
}

func (c *userRecordingClients) AsS3(ctx context.Context, storage, user string) (s3client.Client, error) {
	c.mu.Lock()
	c.users = append(c.users, user)
	c.mu.Unlock()
	return c.comprehensiveClients.AsS3(ctx, storage, user)
}

// fakeS3 serves one object (etag, status 200) or none (404) and counts mutating requests.
type fakeS3 struct {
	srv     *httptest.Server
	puts    atomic.Int32
	deletes atomic.Int32
}

func newFakeS3(t *testing.T, present bool, etag string) *fakeS3 {
	f := &fakeS3{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "probe-health-test") {
			w.WriteHeader(403)
			return
		}
		switch r.Method {
		case http.MethodPut:
			_, _ = io.Copy(io.Discard, r.Body)
			f.puts.Add(1)
			w.Header().Set("ETag", `"`+etag+`"`)
		case http.MethodDelete:
			f.deletes.Add(1)
			w.WriteHeader(204)
		default:
			if !present {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Length", "4")
			w.Header().Set("ETag", `"`+etag+`"`)
			w.Header().Set("Last-Modified", "Wed, 11 Nov 2026 00:00:00 GMT")
			if r.Method == http.MethodGet {
				_, _ = io.WriteString(w, "data")
			}
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func copyWith(t *testing.T, user string, src, dst *fakeS3) (*userRecordingClients, error) {
	clients := &userRecordingClients{comprehensiveClients: comprehensiveClients{
		source: comprehensiveClient(t, src.srv.URL), dest: comprehensiveClient(t, dst.srv.URL)}}
	svc := NewS3CopySvc(comprehensiveCreds{}, clients, metrics.NewS3Service(false))
	err := svc.CopyObject(t.Context(), user, NewFile("source", "bucket", "object"), NewFile("dest", "bucket", "object"))
	return clients, err
}

const (
	etagA = "8d777f385d3dfec8815d20f7496026dc"
	etagB = "11111111111111111111111111111111"
)

// A storage-level replication (empty user) must resolve both clients with the empty user, which the
// registry maps to the connection account.
func TestCopyObject_EmptyUserUsesConnectionAccount(t *testing.T) {
	clients, err := copyWith(t, "", newFakeS3(t, true, etagA), newFakeS3(t, false, ""))
	require.NoError(t, err)
	require.Equal(t, []string{"", ""}, clients.users)
}

// Characterization: an object identical on both sides is skipped, nothing is written.
func TestCopyObject_IdenticalObjectIsSkipped(t *testing.T) {
	dst := newFakeS3(t, true, etagA)
	_, err := copyWith(t, "", newFakeS3(t, true, etagA), dst)
	require.NoError(t, err)
	require.Zero(t, dst.puts.Load())
	require.Zero(t, dst.deletes.Load())
}

// Characterization: the same object name with different content on the destination is OVERWRITTEN by the
// source version (the source always wins; no conflict detection, no timestamp comparison).
func TestCopyObject_SameNameDifferentContentIsOverwritten(t *testing.T) {
	dst := newFakeS3(t, true, etagB)
	_, err := copyWith(t, "", newFakeS3(t, true, etagA), dst)
	require.NoError(t, err)
	require.Equal(t, int32(1), dst.puts.Load())
}

// Characterization: an object that exists only on the destination is never touched by a copy; a copy of an
// object missing at the source returns NotFound and does not delete or write at the destination.
func TestCopyObject_ObjectOnlyOnDestinationIsKept(t *testing.T) {
	dst := newFakeS3(t, true, etagB)
	_, err := copyWith(t, "", newFakeS3(t, false, ""), dst)
	require.Error(t, err)
	require.Zero(t, dst.puts.Load())
	require.Zero(t, dst.deletes.Load())
}
