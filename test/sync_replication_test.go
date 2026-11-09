package test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/test/app"
)

// With synchronous replication the follower copies must exist as soon as the proxy answers
// (no polling), and deletes must be applied on the followers before the proxy answers.
func TestApi_SyncReplication_PutAndDelete(t *testing.T) {
	r := require.New(t)
	pc, err := app.DeepCopyStruct(proxyConf)
	r.NoError(err)
	pc.Replication.Sync = proxy.SyncConfig{Enabled: true, Timeout: 20 * time.Second, OnQuorumFailure: "fail"}

	e := app.SetupEmbedded(t, workerConf, pc)
	e.CreateMainFollowerUserReplications(t)
	ctx := t.Context()
	bucket := "sync-repl"

	r.NoError(e.ProxyClient.MakeBucket(ctx, bucket, mclient.MakeBucketOptions{Region: "us-east"}))
	r.Eventually(func() bool {
		for _, c := range []*mclient.Client{e.MainClient, e.F1Client, e.F2Client} {
			if ok, err := c.BucketExists(ctx, bucket); err != nil || !ok {
				return false
			}
		}
		return true
	}, e.WaitShort, e.RetryShort)

	for i := range 5 {
		name := fmt.Sprintf("obj-%d", i)
		data := bytes.Repeat([]byte{byte('a' + i)}, 1024*(i+1))
		_, err = e.ProxyClient.PutObject(ctx, bucket, name, bytes.NewReader(data), int64(len(data)), mclient.PutObjectOptions{DisableContentSha256: true})
		r.NoError(err)
		// immediately, no polling
		for n, c := range map[string]*mclient.Client{"main": e.MainClient, "f1": e.F1Client, "f2": e.F2Client} {
			st, err := c.StatObject(ctx, bucket, name, mclient.StatObjectOptions{})
			r.NoError(err, "%s must have %s right after PUT", n, name)
			r.EqualValues(len(data), st.Size, n)
		}
	}

	r.NoError(e.ProxyClient.RemoveObject(ctx, bucket, "obj-0", mclient.RemoveObjectOptions{}))
	for n, c := range map[string]*mclient.Client{"main": e.MainClient, "f1": e.F1Client, "f2": e.F2Client} {
		_, err := c.StatObject(ctx, bucket, "obj-0", mclient.StatObjectOptions{})
		r.Error(err, "%s must not have obj-0 right after DELETE", n)
	}
}
