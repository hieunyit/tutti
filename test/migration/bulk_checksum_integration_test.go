package migration

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"

	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/hieunyit/tutti/test/app"
)

// TestPR7_Integration_DestinationByteChecksumVerification
// Integration test running against embedded in-process S3 endpoints (gofakes3/s3mem)
// and miniredis via app.SetupEmbedded:
// 1. Ingests objects of varying sizes into source S3 storage ("main").
// 2. Triggers bulk migration via AddReplication.
// 3. Directly downloads all object bytes from destination S3 storage ("f1") over HTTP.
// 4. Computes SHA-256 directly from destination bytes and asserts byte-for-byte equality with source.
func TestPR7_Integration_DestinationByteChecksumVerification(t *testing.T) {
	e := app.SetupEmbedded(t, workerConf, proxyConf)
	ctx := t.Context()
	r := require.New(t)

	bucket := "bulk-checksum-test"
	err := e.MainClient.MakeBucket(ctx, bucket, mclient.MakeBucketOptions{})
	r.NoError(err)

	type testFile struct {
		name       string
		size       int
		sourceHash string
	}

	testSizes := []struct {
		name string
		size int
	}{
		{"small-512b.bin", 512},
		{"page-4k.bin", 4096},
		{"medium-64k.bin", 64 * 1024},
		{"large-256k.bin", 256 * 1024},
		{"one-mb.bin", 1024 * 1024},
	}

	var files []testFile
	var totalBytes int64

	for idx, ts := range testSizes {
		data := make([]byte, ts.size)
		for i := range data {
			data[i] = byte((i + idx*17) % 256)
		}
		h := sha256.Sum256(data)
		hashStr := hex.EncodeToString(h[:])

		_, err = e.MainClient.PutObject(ctx, bucket, ts.name, bytes.NewReader(data), int64(ts.size), mclient.PutObjectOptions{
			ContentType: "application/octet-stream",
		})
		r.NoError(err)

		files = append(files, testFile{
			name:       ts.name,
			size:       ts.size,
			sourceHash: hashStr,
		})
		totalBytes += int64(ts.size)
	}

	replID := &pb.ReplicationID{
		FromBucket:  &bucket,
		ToBucket:    &bucket,
		FromStorage: "main",
		ToStorage:   "f1",
		User:        user,
	}

	start := time.Now()
	_, err = e.PolicyClient.AddReplication(ctx, &pb.AddReplicationRequest{
		Id: replID,
	})
	r.NoError(err)

	// Wait until all objects appear in destination MinIO ("f1")
	r.Eventually(func() bool {
		objects, err := listObjects(e.F1Client, bucket, "")
		if err != nil {
			return false
		}
		return len(objects) == len(files)
	}, 15*time.Second, 100*time.Millisecond)

	elapsed := time.Since(start)

	// Verify destination bytes directly from destination MinIO storage
	for _, f := range files {
		obj, err := e.F1Client.GetObject(ctx, bucket, f.name, mclient.GetObjectOptions{})
		r.NoError(err)

		hasher := sha256.New()
		n, err := io.Copy(hasher, obj)
		_ = obj.Close()
		r.NoError(err)
		r.Equal(int64(f.size), n, "size mismatch on destination object %s", f.name)

		destHash := hex.EncodeToString(hasher.Sum(nil))
		r.Equal(f.sourceHash, destHash, "SHA-256 hash mismatch on destination object %s", f.name)

		t.Logf("[Verified Destination S3] Object: %-15s | Size: %7d bytes | SHA256: %s (MATCH)",
			f.name, f.size, destHash)
	}

	throughputMBps := (float64(totalBytes) / (1024 * 1024)) / elapsed.Seconds()
	t.Logf("[Bulk S3 Integration Result] Total Objects: %d | Total Bytes: %d | Elapsed: %v | Rate: %.2f MB/s",
		len(files), totalBytes, elapsed, throughputMBps)
}
