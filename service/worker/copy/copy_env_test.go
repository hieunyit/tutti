package copy

import (
	"context"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/minio/minio-go/v7"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/systemsettings"
)

// copyEnv wires an S3CopySvc to two in-process S3 servers.
type copyEnv struct {
	t        *testing.T
	src, dst *memS3
	svc      *S3CopySvc
}

func newCopyEnv(t *testing.T, worker controlplane.SystemWorkerSettings, feat features.Config) *copyEnv {
	t.Helper()
	old := &features.Config{Versioning: features.Versioning(t.Context()), Tagging: features.Tagging(t.Context()), ACL: features.ACL(t.Context())}
	t.Cleanup(func() { features.Set(old) })
	fc := feat
	features.Set(&fc)
	prev := systemsettings.Current()
	t.Cleanup(func() { systemsettings.ApplySettings(prev) })
	if worker.BufferBudgetBytes == 0 {
		worker.BufferBudgetBytes = 256 << 20
	}
	systemsettings.ApplySettings(controlplane.SystemSettings{Worker: worker})
	e := &copyEnv{t: t, src: newMemS3(), dst: newMemS3()}
	t.Cleanup(e.src.Close)
	t.Cleanup(e.dst.Close)
	clients := comprehensiveClients{source: comprehensiveClient(t, e.src.URL()), dest: comprehensiveClient(t, e.dst.URL())}
	e.svc = NewS3CopySvc(comprehensiveCreds{}, clients, metrics.NewS3Service(false))
	return e
}

func (e *copyEnv) copy(ctx context.Context, name string) error {
	return e.svc.CopyObject(ctx, "u", NewFile("source", "bucket", name), NewFile("dest", "bucket", name))
}

func (e *copyEnv) putSource(name string, data []byte, h http.Header) {
	e.src.Put("bucket/"+name, data, h)
}

func (e *copyEnv) destBytes(name string) []byte {
	o, ok := e.dst.Get("bucket/" + name)
	require.True(e.t, ok, "destination object %s missing", name)
	return o.data
}

func coreOf(c s3client.Client) minio.Core { return minio.Core{Client: c.S3().Client} }
