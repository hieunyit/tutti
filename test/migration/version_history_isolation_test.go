package migration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"testing"
	"time"

	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/service/worker"
	"github.com/hieunyit/tutti/test/app"
	minio "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
)

func TestVersionHistory_TwoDestinationsPreserveAllBytesAndLatest(t *testing.T) {
	wc, err := worker.GetConfig()
	require.NoError(t, err)
	pc, err := proxy.GetConfig()
	require.NoError(t, err)
	wc.Features.Versioning = true
	wc.Features.ACL = false
	wc.Features.Tagging = false
	pc.Features.Versioning = true
	pc.Features.ACL = false
	pc.Features.Tagging = false
	e := app.SetupEmbedded(t, wc, pc)
	ctx := t.Context()
	bucket := "version-history-isolation"
	require.NoError(t, e.MainClient.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}))
	require.NoError(t, e.MainClient.SetBucketVersioning(ctx, bucket, minio.BucketVersioningConfiguration{Status: "Enabled"}))
	bodies := []string{"version-one", "version-two", "version-three"}
	want := map[[32]byte]bool{}
	for _, body := range bodies {
		info, err := e.MainClient.PutObject(ctx, bucket, "key", bytes.NewBufferString(body), int64(len(body)), minio.PutObjectOptions{ContentType: "text/plain"})
		require.NoError(t, err)
		require.NotEmpty(t, info.VersionID)
		want[sha256.Sum256([]byte(body))] = true
	}
	for _, dst := range []string{"f1", "f2"} {
		_, err := e.PolicyClient.AddReplication(ctx, &pb.AddReplicationRequest{Id: &pb.ReplicationID{User: user, FromStorage: "main", ToStorage: dst, FromBucket: &bucket, ToBucket: &bucket}})
		require.NoError(t, err)
	}
	readVersions := func(client *minio.Client) (map[[32]byte]int, error) {
		scanCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		got := map[[32]byte]int{}
		for obj := range client.ListObjects(scanCtx, bucket, minio.ListObjectsOptions{Recursive: true, WithVersions: true}) {
			if obj.Err != nil {
				return nil, obj.Err
			}
			if obj.IsDeleteMarker {
				continue
			}
			r, err := client.GetObject(scanCtx, bucket, obj.Key, minio.GetObjectOptions{VersionID: obj.VersionID})
			if err != nil {
				return nil, err
			}
			body, err := io.ReadAll(r)
			r.Close()
			if err != nil {
				return nil, err
			}
			got[sha256.Sum256(body)]++
		}
		return got, nil
	}
	for name, client := range map[string]*minio.Client{"f1": e.F1Client, "f2": e.F2Client} {
		require.Eventually(t, func() bool {
			got, err := readVersions(client)
			if err != nil || len(got) != len(want) {
				return false
			}
			for hash := range want {
				if got[hash] != 1 {
					return false
				}
			}
			return true
		}, 20*time.Second, 100*time.Millisecond, "destination %s must contain every source version", name)
		object, err := client.GetObject(ctx, bucket, "key", minio.GetObjectOptions{})
		require.NoError(t, err)
		body, err := io.ReadAll(object)
		object.Close()
		require.NoError(t, err)
		require.Equal(t, bodies[len(bodies)-1], string(body), "destination %s must retain newest bytes as latest", name)
	}
}
