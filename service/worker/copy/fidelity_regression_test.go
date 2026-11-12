package copy

import (
	"context"
	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
	minio "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type comprehensiveMetrics struct{}

func (comprehensiveMetrics) Count(xctx.Flow, string, string)         {}
func (comprehensiveMetrics) Upload(xctx.Flow, string, string, int)   {}
func (comprehensiveMetrics) Download(xctx.Flow, string, string, int) {}

type comprehensiveClients struct {
	objstore.Clients
	source, dest s3client.Client
}

func (c comprehensiveClients) AsS3(_ context.Context, storage, user string) (s3client.Client, error) {
	if storage == "source" {
		return c.source, nil
	}
	return c.dest, nil
}

type comprehensiveCreds struct{ objstore.CredsService }

func (comprehensiveCreds) GetS3Address(string) (s3.StorageAddress, error) {
	return s3.StorageAddress{}, nil
}
func comprehensiveClient(t *testing.T, url string) s3client.Client {
	t.Helper()
	c, err := s3client.NewClient(t.Context(), comprehensiveMetrics{}, s3.StorageAddress{Address: url, DefaultRegion: "us-east-1"}, s3.CredentialsV4{AccessKeyID: "access", SecretAccessKey: "secret"}, "storage", "u")
	require.NoError(t, err)
	return c
}
func TestRegression_CopyPreservesStandardHeadersAndTags(t *testing.T) {
	old := &features.Config{Versioning: features.Versioning(t.Context()), Tagging: features.Tagging(t.Context()), ACL: features.ACL(t.Context())}
	t.Cleanup(func() { features.Set(old) })
	features.Set(&features.Config{Tagging: true})
	headers := http.Header{"Content-Type": {"text/plain"}, "Content-Encoding": {"br"}, "Cache-Control": {"private, max-age=42"}, "Content-Disposition": {"attachment; filename=secret.txt"}, "Content-Language": {"vi"}}
	var tagReads atomic.Int32
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "probe-health-test") {
			w.WriteHeader(403)
			return
		}
		if r.URL.Query().Has("tagging") {
			tagReads.Add(1)
			w.Header().Set("Content-Type", "application/xml")
			io.WriteString(w, `<Tagging><TagSet><Tag><Key>classification</Key><Value>private</Value></Tag></TagSet></Tagging>`)
			return
		}
		for k, v := range headers {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Length", "4")
		w.Header().Set("ETag", `"8d777f385d3dfec8815d20f7496026dc"`)
		w.Header().Set("Last-Modified", "Wed, 11 Nov 2026 00:00:00 GMT")
		w.Header().Set("X-Amz-Tagging-Count", "1")
		if r.Method != "HEAD" {
			io.WriteString(w, "data")
		}
	}))
	defer src.Close()
	got := make(chan http.Header, 1)
	dst := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "probe-health-test") {
			w.WriteHeader(403)
			return
		}
		if r.Method == "HEAD" {
			w.WriteHeader(404)
			return
		}
		if r.Method == "PUT" {
			_, err := io.Copy(io.Discard, r.Body)
			if err != nil {
				t.Error(err)
			}
			got <- r.Header.Clone()
			w.Header().Set("ETag", `"8d777f385d3dfec8815d20f7496026dc"`)
			return
		}
		w.WriteHeader(400)
	}))
	defer dst.Close()
	clients := comprehensiveClients{source: comprehensiveClient(t, src.URL), dest: comprehensiveClient(t, dst.URL)}
	// Validate fixture's source HEAD before assessing the copy.
	stat, err := clients.source.S3().StatObject(t.Context(), "bucket", "object", minio.StatObjectOptions{})
	require.NoError(t, err)
	require.Equal(t, "br", stat.ContentEncoding)
	require.Equal(t, 1, stat.UserTagCount)
	svc := NewS3CopySvc(comprehensiveCreds{}, clients, metrics.NewS3Service(false))
	require.NoError(t, svc.CopyObject(t.Context(), "u", NewFile("source", "bucket", "object"), NewFile("dest", "bucket", "object")))
	actual := <-got
	for _, k := range []string{"Content-Type", "Content-Encoding", "Cache-Control", "Content-Disposition", "Content-Language"} {
		if actual.Get(k) != headers.Get(k) {
			t.Errorf("lost %s: source=%q destination=%q", k, headers.Get(k), actual.Get(k))
		}
	}
	if actual.Get("X-Amz-Tagging") != "classification=private" {
		t.Errorf("lost source tags: destination=%q sourceTagGETs=%d", actual.Get("X-Amz-Tagging"), tagReads.Load())
	}
}
