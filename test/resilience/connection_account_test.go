package resilience

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/minio/madmin-go/v4"
	mclient "github.com/minio/minio-go/v7"
	mcredentials "github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/s3"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/service/worker"
)

// tuttiRW is the MinIO policy of docs/tutti/storage-accounts.md (whole storage, all features; MinIO has no ACL actions).
const tuttiRW = `{
  "Version": "2012-10-17",
  "Statement": [
    {"Effect": "Allow", "Action": ["s3:ListAllMyBuckets", "s3:GetBucketLocation"], "Resource": ["arn:aws:s3:::*"]},
    {"Effect": "Allow",
     "Action": ["s3:CreateBucket", "s3:DeleteBucket", "s3:ListBucket", "s3:ListBucketVersions",
                "s3:ListBucketMultipartUploads", "s3:GetBucketVersioning", "s3:PutBucketVersioning",
                "s3:GetBucketTagging", "s3:PutBucketTagging",
                "s3:GetLifecycleConfiguration", "s3:PutLifecycleConfiguration",
                "s3:GetBucketPolicy", "s3:PutBucketPolicy", "s3:DeleteBucketPolicy"],
     "Resource": ["arn:aws:s3:::*"]},
    {"Effect": "Allow",
     "Action": ["s3:GetObject", "s3:GetObjectVersion", "s3:PutObject", "s3:DeleteObject", "s3:DeleteObjectVersion",
                "s3:ListMultipartUploadParts", "s3:AbortMultipartUpload",
                "s3:GetObjectTagging", "s3:PutObjectTagging"],
     "Resource": ["arn:aws:s3:::*/*"]}
  ]
}`

// noDelete is tuttiRW without s3:DeleteObject.
var noDelete = strings.Replace(tuttiRW, `"s3:PutObject", "s3:DeleteObject", `, `"s3:PutObject", `, 1)

func minioAdmin(t *testing.T, key string) *madmin.AdminClient {
	t.Helper()
	root := storages[key].Credentials[user]
	adm, err := madmin.NewWithOptions(strings.TrimPrefix(storages[key].Address, "http://"), &madmin.Options{
		Creds: mcredentials.NewStaticV4(root.AccessKeyID, root.SecretAccessKey, ""),
	})
	require.NoError(t, err)
	return adm
}

// createAccount creates a MinIO user with a canned policy, like the mc commands of the storage accounts guide.
func createAccount(t *testing.T, adm *madmin.AdminClient, name, secret, policyName, policy string) s3.CredentialsV4 {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, adm.AddCannedPolicy(ctx, policyName, []byte(policy)))
	require.NoError(t, adm.AddUser(ctx, name, secret))
	_, err := adm.AttachPolicy(ctx, madmin.PolicyAssociationReq{Policies: []string{policyName}, User: name})
	require.NoError(t, err)
	return s3.CredentialsV4{AccessKeyID: name, SecretAccessKey: secret}
}

func stepByID(res map[string]any, id string) map[string]any {
	for _, s := range res["steps"].([]any) {
		if m := s.(map[string]any); m["id"] == id {
			return m
		}
	}
	return nil
}

func failedSteps(res map[string]any) string {
	var out []string
	for _, s := range res["steps"].([]any) {
		if m := s.(map[string]any); m["status"] == "fail" {
			out = append(out, fmt.Sprintf("%v %v: %v", m["number"], m["id"], m["message"]))
		}
	}
	return strings.Join(out, "; ")
}

// removeBucket force-removes a bucket (all versions) with the root account of the storage.
func removeBucket(t *testing.T, key, bucket string) {
	t.Helper()
	ctx := context.Background()
	c := direct[key]
	for o := range c.ListObjects(ctx, bucket, mclient.ListObjectsOptions{Recursive: true, WithVersions: true}) {
		if o.Err == nil {
			_ = c.RemoveObject(ctx, bucket, o.Key, mclient.RemoveObjectOptions{VersionID: o.VersionID})
		}
	}
	_ = c.RemoveBucket(ctx, bucket)
}

func TestConnectionAccountsEndToEnd(t *testing.T) {
	ctx := t.Context()
	// the dedicated accounts on the two MinIO servers (never the root account)
	mainAcc := createAccount(t, minioAdmin(t, mainKey), "tutti-svc", "tutti-svc-secret-1", "tutti-rw", tuttiRW)
	f2Admin := minioAdmin(t, f2Key)
	archiveAcc := createAccount(t, f2Admin, "tutti-svc", "tutti-svc-secret-2", "tutti-rw", tuttiRW)

	// "main" is a config storage with connectionAccount and NO per-user credentials; "archive" is created later through the API
	useConnection := func(st *s3.Storage) { st.Credentials, st.ConnectionAccount = nil, &mainAcc }
	e := startEnv(t, envOpts{dynamicCreds: true,
		proxyHook: func(c *proxy.Config) {
			delete(c.Storage.Storages, f1Key)
			delete(c.Storage.Storages, f2Key)
			useConnection(c.Storage.Storages[mainKey].S3)
		},
		workerHook: func(c *worker.Config) {
			delete(c.Storage.Storages, f1Key)
			delete(c.Storage.Storages, f2Key)
			useConnection(c.Storage.Storages[mainKey].S3)
		},
	})

	// --- test connection of the config storage: every step passes with the dedicated account
	res := e.restJSON(t, http.MethodPost, "/tutti/storages/test", map[string]any{"name": mainKey})
	require.Equal(t, true, res["ok"], failedSteps(res))
	require.Len(t, res["steps"], 16)
	require.Equal(t, "pass", stepByID(res, "authentication")["status"])
	require.Equal(t, "pass", stepByID(res, "root-account")["status"], "tutti-svc is not a root account")
	require.Equal(t, "pass", stepByID(res, "multipart")["status"])
	require.Equal(t, "pass", stepByID(res, "delete")["status"])
	require.Empty(t, res["leftovers"], "nothing left behind")

	// the root account is flagged
	rootRes := e.restJSON(t, http.MethodPost, "/tutti/storages/test", map[string]any{"name": mainKey,
		"connectionAccount": map[string]any{"accessKeyId": mainCreds.AccessKeyID, "secretAccessKey": mainCreds.SecretAccessKey}})
	require.Equal(t, "warn", stepByID(rootRes, "root-account")["status"], "%v", stepByID(rootRes, "root-account"))

	// --- add the second storage through the API with a connection account (the test runs and is stored)
	config := map[string]any{"name": "archive", "address": storages[f2Key].Address, "provider": "Minio", "defaultRegion": "us-east-1", "httpTimeout": "10s", "healthCheckInterval": "1s"}
	created := e.restJSON(t, http.MethodPost, "/tutti/storages", map[string]any{"storage": config,
		"connectionAccount": map[string]any{"accessKeyId": archiveAcc.AccessKeyID, "secretAccessKey": archiveAcc.SecretAccessKey}})
	require.Equal(t, "tutti-svc", created["connectionAccount"].(map[string]any)["accessKeyId"])
	require.Equal(t, true, created["lastTest"].(map[string]any)["ok"])
	require.Equal(t, false, created["legacyCredentials"])
	require.NotContains(t, fmt.Sprint(created), archiveAcc.SecretAccessKey)

	// --- a Tutti user without any storage key
	keyless := e.restJSON(t, http.MethodPost, "/tutti/users", map[string]any{"name": "keyless", "createAccessKey": true,
		"policy": map[string]any{"permission": "read-write", "buckets": []string{"conn-acc-*"}}})
	key := keyless["accessKey"].(map[string]any)
	require.True(t, strings.HasPrefix(key["accessKeyId"].(string), "TUT"))
	u := keyless["user"].(map[string]any)
	require.Equal(t, true, u["complete"])
	for _, s := range u["storages"].([]any) {
		require.Equal(t, "connection-account", s.(map[string]any)["mode"], "%v", s)
		require.Equal(t, false, s.(map[string]any)["configured"])
	}
	users := e.restJSON(t, http.MethodGet, "/tutti/users", nil)
	found := false
	for _, x := range users["users"].([]any) {
		found = found || x.(map[string]any)["name"] == "keyless"
	}
	require.True(t, found, "identity-only users are listed")
	// a user without an override has nothing to remove
	e.restStatus(t, http.MethodPut, "/tutti/users/credentials/delete", map[string]any{"name": "keyless", "storage": mainKey}, http.StatusNotFound)

	// --- its Tutti key creates a bucket, writes and reads through the proxy; the proxy signs with tutti-svc
	client := e.keyClient(t, key["accessKeyId"].(string), key["secretAccessKey"].(string))
	bucket := bucketName("conn-acc")
	require.Eventually(t, func() bool {
		return client.MakeBucket(ctx, bucket, mclient.MakeBucketOptions{Region: "us-east-1"}) == nil
	}, 15*time.Second, 200*time.Millisecond)
	t.Cleanup(func() { removeBucket(t, mainKey, bucket); removeBucket(t, f2Key, bucket) })
	_, err := client.PutObject(ctx, bucket, "obj", bytes.NewReader([]byte("via connection account")), 22, mclient.PutObjectOptions{})
	require.NoError(t, err)
	got, err := readObject(t, client, bucket, "obj")
	require.NoError(t, err)
	require.Equal(t, "via connection account", got)
	got, err = readObject(t, direct[mainKey], bucket, "obj")
	require.NoError(t, err)
	require.Equal(t, "via connection account", got)

	// --- replicate main -> archive; both storages use a connection account, the user has no key on either
	repl := replID(bucket, "archive")
	repl.User = "keyless"
	_, err = e.PolicyClient.AddReplication(ctx, &pb.AddReplicationRequest{Id: repl})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		r, err := e.PolicyClient.GetReplication(ctx, repl)
		return err == nil && r.IsInitDone
	}, e.WaitLong, e.RetryLong, "initial sync main->archive")
	_, err = client.PutObject(ctx, bucket, "second", bytes.NewReader([]byte("replicated")), 10, mclient.PutObjectOptions{})
	require.NoError(t, err)
	for _, name := range []string{"obj", "second"} {
		require.Eventually(t, func() bool {
			v, err := readObject(t, direct[f2Key], bucket, name)
			return err == nil && v != ""
		}, e.WaitLong, e.RetryLong, "%s must reach archive", name)
	}

	// --- Test connection with a policy that misses s3:DeleteObject names the exact failing step
	noDel := createAccount(t, f2Admin, "tutti-nodel", "tutti-nodel-secret-3", "tutti-nodelete", noDelete)
	bad := e.restJSON(t, http.MethodPost, "/tutti/storages/test", map[string]any{"name": "archive",
		"connectionAccount": map[string]any{"accessKeyId": noDel.AccessKeyID, "secretAccessKey": noDel.SecretAccessKey}})
	require.Equal(t, false, bad["ok"])
	del := stepByID(bad, "delete")
	require.Equal(t, "fail", del["status"], failedSteps(bad))
	require.Contains(t, del["missingPermission"], "s3:DeleteObject")
	require.Equal(t, "pass", stepByID(bad, "write")["status"])
	require.Equal(t, "pass", stepByID(bad, "read")["status"])
	require.Contains(t, del["message"], "DeleteObject")
	if probe, _ := bad["probeBucket"].(string); probe != "" {
		t.Cleanup(func() { removeBucket(t, f2Key, probe) })
	}
	// and a rotation to it is refused (400), the stored account stays
	e.restStatus(t, http.MethodPut, "/tutti/storages/connection-account", map[string]any{"name": "archive",
		"newAccount": map[string]any{"accessKeyId": noDel.AccessKeyID, "secretAccessKey": noDel.SecretAccessKey}}, http.StatusBadRequest)
	got1 := e.restJSON(t, http.MethodGet, "/tutti/storages/archive", nil)
	require.Equal(t, "tutti-svc", got1["connectionAccount"].(map[string]any)["accessKeyId"])

	// --- rotation: a new account on the storage, tested, switched; traffic keeps flowing with the same Tutti key
	acc2 := createAccount(t, f2Admin, "tutti-svc2", "tutti-svc2-secret-4", "tutti-rw2", tuttiRW)
	rotated := e.restJSON(t, http.MethodPut, "/tutti/storages/connection-account", map[string]any{"name": "archive",
		"newAccount": map[string]any{"accessKeyId": acc2.AccessKeyID, "secretAccessKey": acc2.SecretAccessKey}})
	ca := rotated["connectionAccount"].(map[string]any)
	require.Equal(t, "tutti-svc2", ca["accessKeyId"])
	require.Equal(t, "tutti-svc", ca["previousAccessKeyId"])
	require.NotEmpty(t, ca["safeToRevokePreviousAfter"])
	_, err = client.PutObject(ctx, bucket, "after-rotation", bytes.NewReader([]byte("rotated")), 7, mclient.PutObjectOptions{})
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		v, err := readObject(t, direct[f2Key], bucket, "after-rotation")
		return err == nil && v == "rotated"
	}, e.WaitLong, e.RetryLong, "replication after rotation (worker uses the new account)")

	// --- the periodic check surfaces a disabled account with the step name in the health error
	require.NoError(t, f2Admin.SetUserStatus(ctx, "tutti-svc2", madmin.AccountDisabled))
	require.Eventually(t, func() bool {
		st := e.proxyStatus(t)
		list, _ := st["storages"].([]any)
		for _, s := range list {
			m := s.(map[string]any)
			if m["name"] == "archive" {
				msg, _ := m["lastError"].(string)
				return strings.HasPrefix(msg, "authentication: ")
			}
		}
		return false
	}, 20*time.Second, 300*time.Millisecond, "health error must name the failing step")
	// back to healthy once the account is enabled again
	require.NoError(t, f2Admin.SetUserStatus(ctx, "tutti-svc2", madmin.AccountEnabled))
	require.Eventually(t, func() bool {
		st := e.proxyStatus(t)
		a, found := storageAvailable(st, "archive")
		for _, s := range st["storages"].([]any) {
			if m := s.(map[string]any); m["name"] == "archive" && m["lastError"] != "" {
				return false
			}
		}
		return found && a
	}, 30*time.Second, 300*time.Millisecond, "the health error is cleared by the next clean check")
}

// restStatus calls the REST gateway and asserts the status code.
func (e *testEnv) restStatus(t testing.TB, method, path string, body any, want int) {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(t.Context(), method, e.UrlHttpApi+path, bytes.NewReader(b))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	require.Equal(t, want, res.StatusCode, string(data))
}
