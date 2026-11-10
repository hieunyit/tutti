package resilience

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/service/worker"
)

// Users without storage keys share one connection account: the access policy globs are their only isolation.
func TestSharedAccountIsolation(t *testing.T) {
	ctx := t.Context()
	mainAcc := createAccount(t, minioAdmin(t, mainKey), "tutti-iso", "tutti-iso-secret-1", "tutti-iso-rw", tuttiRW)
	archiveAcc := createAccount(t, minioAdmin(t, f2Key), "tutti-iso", "tutti-iso-secret-2", "tutti-iso-rw", tuttiRW)
	rdb := newFailingRedis(t)
	e := startEnv(t, envOpts{dynamicCreds: true, redis: rdb,
		proxyHook: func(c *proxy.Config) {
			delete(c.Storage.Storages, f1Key)
			delete(c.Storage.Storages, f2Key)
			st := c.Storage.Storages[mainKey].S3
			st.Credentials, st.ConnectionAccount = nil, &mainAcc
		},
		workerHook: func(c *worker.Config) {
			delete(c.Storage.Storages, f1Key)
			delete(c.Storage.Storages, f2Key)
			st := c.Storage.Storages[mainKey].S3
			st.Credentials, st.ConnectionAccount = nil, &mainAcc
		},
	})
	config := map[string]any{"name": "archive", "address": storages[f2Key].Address, "provider": "Minio", "defaultRegion": "us-east-1", "httpTimeout": "10s", "healthCheckInterval": "1s"}
	e.restJSON(t, http.MethodPost, "/tutti/storages", map[string]any{"storage": config,
		"connectionAccount": map[string]any{"accessKeyId": archiveAcc.AccessKeyID, "secretAccessKey": archiveAcc.SecretAccessKey}})

	// a keyless user must come with a policy
	e.restStatus(t, http.MethodPost, "/tutti/users", map[string]any{"name": "nopolicy", "createAccessKey": true}, http.StatusBadRequest)

	newUser := func(name string, buckets ...string) *mclient.Client {
		res := e.restJSON(t, http.MethodPost, "/tutti/users", map[string]any{"name": name, "createAccessKey": true,
			"policy": map[string]any{"permission": accesspolicy.ReadWrite, "buckets": buckets}})
		key := res["accessKey"].(map[string]any)
		return e.keyClient(t, key["accessKeyId"].(string), key["secretAccessKey"].(string))
	}
	alice := newUser("alice", "iso-alice-*")
	bob := newUser("bob", "iso-bob-*")
	// the last policy of a shared-account user cannot be deleted
	e.restStatus(t, http.MethodPut, "/tutti/access-policies/delete", map[string]any{"user": "bob"}, http.StatusBadRequest)
	// a user that lost its policy (e.g. deleted behind the API) gets nothing
	res := e.restJSON(t, http.MethodPost, "/tutti/users", map[string]any{"name": "ghost", "createAccessKey": true,
		"policy": map[string]any{"permission": accesspolicy.ReadWrite, "buckets": []string{"iso-ghost-*"}}})
	gk := res["accessKey"].(map[string]any)
	ghost := e.keyClient(t, gk["accessKeyId"].(string), gk["secretAccessKey"].(string))
	rdb.DB(3).HDel(accesspolicy.Key, "ghost") // config DB (redis.configDB: 3)

	suffix := bucketName("")
	aliceB, bobB, otherB := "iso-alice-1"+suffix, "iso-bob-1"+suffix, "iso-other"+suffix
	t.Cleanup(func() {
		for _, b := range []string{aliceB, bobB, otherB, "iso-alice-new" + suffix} {
			removeBucket(t, mainKey, b)
			removeBucket(t, f2Key, b)
		}
	})
	// proxies cache policies for a few seconds
	require.Eventually(t, func() bool {
		return alice.MakeBucket(ctx, aliceB, mclient.MakeBucketOptions{Region: "us-east-1"}) == nil
	}, 20*time.Second, 200*time.Millisecond)
	require.Eventually(t, func() bool {
		return bob.MakeBucket(ctx, bobB, mclient.MakeBucketOptions{Region: "us-east-1"}) == nil
	}, 20*time.Second, 200*time.Millisecond)
	// a bucket created by someone else on the same storage
	require.NoError(t, direct[mainKey].MakeBucket(ctx, otherB, mclient.MakeBucketOptions{}))
	for _, b := range []string{aliceB, bobB} {
		_, err := direct[mainKey].PutObject(ctx, b, "obj", bytes.NewReader([]byte(b)), int64(len(b)), mclient.PutObjectOptions{})
		require.NoError(t, err)
	}

	denied := func(err error) {
		t.Helper()
		require.Error(t, err)
		require.Equal(t, http.StatusForbidden, mclient.ToErrorResponse(err).StatusCode, "%v", err)
	}
	names := func(c *mclient.Client) []string {
		bs, err := c.ListBuckets(ctx)
		require.NoError(t, err)
		var out []string
		for _, b := range bs {
			if strings.HasSuffix(b.Name, suffix) {
				out = append(out, b.Name)
			}
		}
		return out
	}
	// create outside the globs, touch and see foreign buckets: all refused
	denied(alice.MakeBucket(ctx, "iso-bob-created-by-alice"+suffix, mclient.MakeBucketOptions{Region: "us-east-1"}))
	denied(alice.MakeBucket(ctx, "unrelated"+suffix, mclient.MakeBucketOptions{Region: "us-east-1"}))
	_, err := alice.StatObject(ctx, bobB, "obj", mclient.StatObjectOptions{})
	denied(err)
	_, err = bob.GetObject(ctx, aliceB, "obj", mclient.GetObjectOptions{})
	if err == nil {
		o, _ := bob.GetObject(ctx, aliceB, "obj", mclient.GetObjectOptions{})
		_, err = o.Stat()
	}
	denied(err)
	_, err = alice.PutObject(ctx, bobB, "evil", strings.NewReader("x"), 1, mclient.PutObjectOptions{})
	denied(err)
	denied(alice.RemoveObject(ctx, bobB, "obj", mclient.RemoveObjectOptions{}))
	denied(alice.RemoveBucket(ctx, bobB))
	_, err = alice.PutObject(ctx, aliceB, "mine", strings.NewReader("ok"), 2, mclient.PutObjectOptions{})
	require.NoError(t, err)
	require.Equal(t, []string{aliceB}, names(alice))
	require.Equal(t, []string{bobB}, names(bob))

	// a user without a policy: 403 on every bucket operation and an empty bucket list
	require.Eventually(t, func() bool {
		// inside its old globs the answer is 404 until the proxy drops the cached policy
		_, err := ghost.StatObject(ctx, "iso-ghost-1"+suffix, "obj", mclient.StatObjectOptions{})
		return err != nil && mclient.ToErrorResponse(err).StatusCode == http.StatusForbidden
	}, 20*time.Second, 200*time.Millisecond)
	err = ghost.MakeBucket(ctx, "iso-ghost-1"+suffix, mclient.MakeBucketOptions{Region: "us-east-1"})
	denied(err)
	require.Contains(t, err.Error(), "explicit policy")
	require.Empty(t, names(ghost))
	bs, err := ghost.ListBuckets(ctx)
	require.NoError(t, err)
	require.Empty(t, bs)

	// user-level replication of a user without policy is refused
	_, err = e.PolicyClient.AddReplication(ctx, &pb.AddReplicationRequest{Id: &pb.ReplicationID{User: "ghost", FromStorage: mainKey, ToStorage: "archive"}})
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "%v", err)

	// user-level replication of alice copies her buckets only
	_, err = e.PolicyClient.AddReplication(ctx, &pb.AddReplicationRequest{Id: &pb.ReplicationID{User: "alice", FromStorage: mainKey, ToStorage: "archive"}})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		o, err := direct[f2Key].GetObject(ctx, aliceB, "mine", mclient.GetObjectOptions{})
		if err != nil {
			return false
		}
		_, err = o.Stat()
		return err == nil
	}, e.WaitLong, e.RetryLong, "alice's bucket must reach archive")
	// new bucket created by alice through the proxy follows
	newB := "iso-alice-new" + suffix
	require.NoError(t, alice.MakeBucket(ctx, newB, mclient.MakeBucketOptions{Region: "us-east-1"}))
	require.Eventually(t, func() bool {
		ok, err := direct[f2Key].BucketExists(ctx, newB)
		return err == nil && ok
	}, e.WaitLong, e.RetryLong)
	time.Sleep(2 * time.Second)
	for _, b := range []string{bobB, otherB} {
		ok, err := direct[f2Key].BucketExists(context.Background(), b)
		require.NoError(t, err)
		require.False(t, ok, "bucket %s is not alice's and must not be replicated", b)
	}
}
