package app

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	minio "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedHeadSelectsRequestedVersion(t *testing.T) {
	server := httptest.NewServer(versionAwareFakeServer(gofakes3.New(s3mem.New()).Server()))
	defer server.Close()
	client, _ := CreateClient(t, s3.StorageAddress{Address: server.URL, DefaultRegion: "us-east-1"}, s3.CredentialsV4{AccessKeyID: "access", SecretAccessKey: "secret"})
	ctx := t.Context()
	require.NoError(t, client.MakeBucket(ctx, "bucket", minio.MakeBucketOptions{}))
	require.NoError(t, client.SetBucketVersioning(ctx, "bucket", minio.BucketVersioningConfiguration{Status: "Enabled"}))
	old, err := client.PutObject(ctx, "bucket", "key", bytes.NewBufferString("old"), 3, minio.PutObjectOptions{})
	require.NoError(t, err)
	_, err = client.PutObject(ctx, "bucket", "key", bytes.NewBufferString("newer"), 5, minio.PutObjectOptions{})
	require.NoError(t, err)
	stat, err := client.StatObject(ctx, "bucket", "key", minio.StatObjectOptions{VersionID: old.VersionID})
	require.NoError(t, err)
	require.EqualValues(t, 3, stat.Size)
	require.Equal(t, old.ETag, stat.ETag)
}

func TestEmbeddedVersionListingIsNewestFirstAcrossPages(t *testing.T) {
	server := httptest.NewServer(versionAwareFakeServer(gofakes3.New(newVersionedMemory()).Server()))
	defer server.Close()
	client, _ := CreateClient(t, s3.StorageAddress{Address: server.URL, DefaultRegion: "us-east-1"}, s3.CredentialsV4{AccessKeyID: "access", SecretAccessKey: "secret"})
	ctx := t.Context()
	require.NoError(t, client.MakeBucket(ctx, "bucket", minio.MakeBucketOptions{}))
	require.NoError(t, client.SetBucketVersioning(ctx, "bucket", minio.BucketVersioningConfiguration{Status: "Enabled"}))
	var ids []string
	for _, body := range []string{"one", "two", "three"} {
		v, err := client.PutObject(ctx, "bucket", "key", bytes.NewBufferString(body), int64(len(body)), minio.PutObjectOptions{})
		require.NoError(t, err)
		ids = append(ids, v.VersionID)
	}
	var listed []string
	for obj := range client.ListObjects(ctx, "bucket", minio.ListObjectsOptions{WithVersions: true, Recursive: true, MaxKeys: 1}) {
		require.NoError(t, obj.Err)
		listed = append(listed, obj.VersionID)
	}
	require.Equal(t, []string{ids[2], ids[1], ids[0]}, listed)
}
