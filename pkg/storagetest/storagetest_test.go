package storagetest_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/storagetest"
	"github.com/hieunyit/tutti/pkg/storagetest/fakes3"
)

func opts(url string) storagetest.Options {
	return storagetest.Options{
		Address: s3.StorageAddress{Address: url, Provider: s3.ProviderMinIO, DefaultRegion: "us-east-1", HttpTimeout: 10 * time.Second},
		Cred:    s3.CredentialsV4{AccessKeyID: "tutti-svc", SecretAccessKey: "secret-secret"},
	}
}

func step(t *testing.T, r *storagetest.Result, id string) storagetest.Step {
	t.Helper()
	var found []storagetest.Step
	for _, s := range r.Steps {
		if s.ID == id {
			found = append(found, s)
		}
	}
	require.NotEmpty(t, found, "step %s missing in %+v", id, r.Steps)
	return found[0]
}

func TestAllStepsPassAndCleanUp(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	o := opts(srv.URL)
	o.Features = &storagetest.Features{Versioning: true, Tagging: true, ACL: true, Lifecycle: true, Policy: true}
	o.Throughput = true
	srv.CreateBucket("app-existing")
	o.ExistingBuckets = []string{"app-existing"}
	res := storagetest.Run(t.Context(), o)
	for _, s := range res.Steps {
		require.Contains(t, []storagetest.Status{storagetest.Pass, storagetest.Skip}, s.Status, "%+v", s)
	}
	require.True(t, res.OK)
	require.Equal(t, "full", res.Kind)
	// steps are ordered by number and cover the plan
	last := 0
	for _, s := range res.Steps {
		require.GreaterOrEqual(t, s.Number, last)
		last = s.Number
	}
	require.Equal(t, 17, last)
	require.Equal(t, storagetest.Pass, step(t, res, storagetest.StepVersioning).Status)
	require.Equal(t, storagetest.Pass, step(t, res, storagetest.StepExisting).Status)
	require.Equal(t, storagetest.Pass, step(t, res, storagetest.StepThroughput).Status)
	require.Equal(t, storagetest.Skip, step(t, res, storagetest.StepTLS).Status, "plain http")
	// the probe bucket is gone and nothing is left behind
	require.Equal(t, []string{"app-existing"}, srv.Buckets())
	require.Empty(t, res.Leftovers)
	require.True(t, strings.HasPrefix(res.ProbeBucket, "tutti-probe-"))
}

func TestProbeBucketPrefixAndExistingBucket(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	o := opts(srv.URL)
	o.ProbeBucketPrefix = "app-tutti-probe-"
	res := storagetest.Run(t.Context(), o)
	require.True(t, res.OK, "%+v", res.Steps)
	require.True(t, strings.HasPrefix(res.ProbeBucket, "app-tutti-probe-"))

	// bucket creation not allowed: the admin picks an existing bucket, which is kept
	srv.Deny["CreateBucket"] = true
	srv.CreateBucket("shared")
	res = storagetest.Run(t.Context(), o)
	require.False(t, res.OK)
	create := step(t, res, storagetest.StepCreate)
	require.Equal(t, storagetest.Fail, create.Status)
	require.Equal(t, "s3:CreateBucket", create.MissingPermission)
	require.Contains(t, create.Hint, "probeBucket")
	o.ProbeBucket = "shared"
	res = storagetest.Run(t.Context(), o)
	require.True(t, res.OK, "%+v", res.Steps)
	require.Equal(t, storagetest.Skip, step(t, res, storagetest.StepCreate).Status)
	require.Equal(t, []string{"shared"}, srv.Buckets())
	require.Zero(t, srv.Objects("shared"), "probe objects are removed from the existing bucket")

	o.ProbeBucket, o.ProbeBucketPrefix = "", "Bad Prefix"
	res = storagetest.Run(t.Context(), o)
	require.Equal(t, storagetest.Fail, step(t, res, storagetest.StepCreate).Status)
}

func TestMissingPermissionPerAction(t *testing.T) {
	cases := []struct {
		deny    string
		stepID  string
		wantErr string // missing permission
	}{
		{"CreateBucket", storagetest.StepCreate, "s3:CreateBucket"},
		{"PutObject", storagetest.StepWrite, "s3:PutObject"},
		{"GetObject", storagetest.StepRead, "s3:GetObject"},
		{"HeadObject", storagetest.StepRead, "s3:GetObject"},
		{"ListObjectsV2", storagetest.StepList, "s3:ListBucket"},
		{"ListParts", storagetest.StepMultipart, "s3:ListMultipartUploadParts"},
		{"ListMultipartUploads", storagetest.StepMultipart, "s3:ListBucketMultipartUploads"},
		{"AbortMultipartUpload", storagetest.StepMultipart, "s3:AbortMultipartUpload"},
		{"GetBucketVersioning", storagetest.StepVersioning, "s3:GetBucketVersioning"},
		{"PutBucketVersioning", storagetest.StepVersioning, "s3:PutBucketVersioning"},
		{"ListObjectVersions", storagetest.StepVersioning, "s3:ListBucketVersions"},
		{"DeleteObjectVersion", storagetest.StepVersioning, "s3:DeleteObjectVersion"},
		{"PutObjectTagging", storagetest.StepFeatures, "s3:PutObjectTagging"},
		{"PutBucketAcl", storagetest.StepFeatures, "s3:PutBucketAcl"},
		{"GetBucketLifecycleConfiguration", storagetest.StepFeatures, "s3:GetLifecycleConfiguration"},
		{"DeleteBucketPolicy", storagetest.StepFeatures, "s3:DeleteBucketPolicy"},
		{"DeleteObject", storagetest.StepDelete, "s3:DeleteObject"},
		{"DeleteBucket", storagetest.StepDelete, "s3:DeleteBucket"},
	}
	for _, c := range cases {
		t.Run(c.deny, func(t *testing.T) {
			srv := fakes3.New()
			defer srv.Close()
			srv.Deny[c.deny] = true
			o := opts(srv.URL)
			o.Features = &storagetest.Features{Versioning: true, Tagging: true, ACL: true, Lifecycle: true, Policy: true}
			res := storagetest.Run(t.Context(), o)
			require.False(t, res.OK)
			var failed *storagetest.Step
			for i, s := range res.Steps {
				if s.Status == storagetest.Fail && s.ID == c.stepID {
					failed = &res.Steps[i]
				}
			}
			require.NotNil(t, failed, "steps: %+v", res.Steps)
			require.Contains(t, failed.MissingPermission, c.wantErr)
			require.Contains(t, failed.Message, c.deny)
			require.Equal(t, "AccessDenied", failed.Code)
		})
	}
}

func TestDeleteFailureListsLeftovers(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	srv.Deny["DeleteObject"] = true
	o := opts(srv.URL)
	o.Features = &storagetest.Features{} // no versioning: the object cannot be removed another way
	res := storagetest.Run(t.Context(), o)
	del := step(t, res, storagetest.StepDelete)
	require.Equal(t, storagetest.Fail, del.Status)
	require.Equal(t, "s3:DeleteObject", del.MissingPermission)
	require.NotEmpty(t, res.Leftovers)
	require.Contains(t, strings.Join(res.Leftovers, " "), res.ProbeBucket)
	require.Contains(t, del.Message, "Left behind")
	require.True(t, res.HasWarnings)
	// the bucket is still there: bucket deletion was impossible while objects remain
	require.Contains(t, srv.Buckets(), res.ProbeBucket)
}

func TestAuthenticationErrors(t *testing.T) {
	for _, code := range []string{"SignatureDoesNotMatch", "InvalidAccessKeyId", "AccessDenied"} {
		t.Run(code, func(t *testing.T) {
			srv := fakes3.New()
			defer srv.Close()
			o := opts(srv.URL)
			if code == "InvalidAccessKeyId" {
				o.Cred.AccessKeyID = "unknown"
			} else if code == "AccessDenied" {
				srv.Deny["ListBuckets"] = true
			} else {
				srv.AuthError = code
			}
			res := storagetest.Run(t.Context(), o)
			s := step(t, res, storagetest.StepAuth)
			require.Equal(t, storagetest.Fail, s.Status)
			require.Equal(t, code, s.Code)
			require.NotEmpty(t, s.Hint)
			for _, later := range []string{storagetest.StepCreate, storagetest.StepWrite, storagetest.StepDelete} {
				require.Equal(t, storagetest.Skip, step(t, res, later).Status)
			}
			// the periodic probe reports the step name and the code
			err := storagetest.Probe(t.Context(), o)
			require.EqualError(t, err, "authentication: "+code)
		})
	}
}

func TestPeriodicProbeOnlyRunsSteps1To5(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	require.NoError(t, storagetest.Probe(t.Context(), opts(srv.URL)))
	require.Len(t, srv.Ops(), 2)
	for _, op := range srv.Ops() {
		require.Contains(t, []string{"ListBuckets"}, op, "only GET / (unauthenticated) and ListBuckets")
	}
	require.Empty(t, srv.Buckets())
}

func TestUnreachableHost(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	res := storagetest.Run(t.Context(), opts("http://"+addr))
	s := step(t, res, storagetest.StepDNSTCP)
	require.Equal(t, storagetest.Fail, s.Status)
	require.Contains(t, s.Message, "Cannot reach")
	require.Contains(t, s.Message, "refused")
	require.Equal(t, storagetest.Skip, step(t, res, storagetest.StepAuth).Status)
}

func TestNotAnS3Endpoint(t *testing.T) {
	for name, h := range map[string]http.HandlerFunc{
		"html": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<!DOCTYPE html><html><body>MinIO Console</body></html>"))
		},
		"redirect": func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/login", http.StatusFound) },
		"json":     func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"ok":true}`)) },
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(h)
			defer srv.Close()
			res := storagetest.Run(t.Context(), opts(srv.URL))
			s := step(t, res, storagetest.StepEndpoint)
			require.Equal(t, storagetest.Fail, s.Status)
			require.Contains(t, s.Message, "does not look like S3")
			require.Equal(t, storagetest.Skip, step(t, res, storagetest.StepAuth).Status)
		})
	}
}

func TestClockSkew(t *testing.T) {
	for _, c := range []struct {
		offset time.Duration
		want   storagetest.Status
	}{{0, storagetest.Pass}, {-2 * time.Minute, storagetest.Warn}, {10 * time.Minute, storagetest.Fail}, {-10 * time.Minute, storagetest.Fail}} {
		srv := fakes3.New()
		srv.DateOffset = c.offset
		res := storagetest.Run(t.Context(), opts(srv.URL))
		s := step(t, res, storagetest.StepClock)
		require.Equal(t, c.want, s.Status, "offset %s: %+v", c.offset, s)
		if c.want == storagetest.Fail {
			require.Contains(t, s.Message, "SigV4 will fail")
		}
		srv.Close()
	}
}

func TestTLSInvalidCertificateNeedsCustomCA(t *testing.T) {
	srv := fakes3.NewTLS()
	defer srv.Close()
	o := opts(srv.URL)
	res := storagetest.Run(t.Context(), o)
	s := step(t, res, storagetest.StepTLS)
	require.Equal(t, storagetest.Fail, s.Status)
	require.Contains(t, s.Hint, "custom CA")
	require.Contains(t, s.Message, "expires")
	require.Equal(t, storagetest.Skip, step(t, res, storagetest.StepEndpoint).Status)

	// with the CA of the server the whole test passes
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	o.RootCAs = pool
	res = storagetest.Run(t.Context(), o)
	require.True(t, res.OK, "%+v", res.Steps)
	require.Equal(t, storagetest.Pass, step(t, res, storagetest.StepTLS).Status)
}

func selfSigned(t *testing.T, notAfter time.Time) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "fake"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: notAfter,
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, IsCA: true, BasicConstraintsValid: true,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func TestTLSExpiring(t *testing.T) {
	for _, c := range []struct {
		name     string
		notAfter time.Time
		want     storagetest.Status
	}{
		{"5 days left", time.Now().Add(5 * 24 * time.Hour), storagetest.Warn},
		{"60 days left", time.Now().Add(60 * 24 * time.Hour), storagetest.Pass},
		{"expired", time.Now().Add(-time.Hour), storagetest.Fail},
	} {
		t.Run(c.name, func(t *testing.T) {
			cert := selfSigned(t, c.notAfter)
			fake := fakes3.New()
			defer fake.Close()
			srv := httptest.NewUnstartedServer(fake.Config.Handler)
			srv.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
			srv.StartTLS()
			defer srv.Close()
			parsed, err := x509.ParseCertificate(cert.Certificate[0])
			require.NoError(t, err)
			pool := x509.NewCertPool()
			pool.AddCert(parsed)
			o := opts(srv.URL)
			o.RootCAs = pool
			res := storagetest.Run(t.Context(), o)
			s := step(t, res, storagetest.StepTLS)
			require.Equal(t, c.want, s.Status, "%+v", s)
			if c.want != storagetest.Fail {
				require.True(t, res.OK)
			}
			if c.want == storagetest.Warn {
				require.Contains(t, s.Message, "expires")
				require.True(t, res.HasWarnings)
			}
			if c.want == storagetest.Fail {
				require.Contains(t, s.Hint, "expired")
			}
		})
	}
}

func TestRootAccountAndRegionWarnings(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	srv.AccessKeys["minioadmin"] = true
	srv.Region = "eu-west-1"
	o := opts(srv.URL)
	o.Cred.AccessKeyID = "minioadmin"
	res := storagetest.Run(t.Context(), o)
	require.True(t, res.OK, "warnings are not failures")
	require.Equal(t, storagetest.Warn, step(t, res, storagetest.StepRoot).Status)
	reg := step(t, res, storagetest.StepRegion)
	require.Equal(t, storagetest.Warn, reg.Status)
	require.Contains(t, reg.Hint, "eu-west-1")

	// owner reported as an administrator (MinIO's fixed owner "minio" is ignored: it is the same for every user)
	srv.AccessKeys["svc"] = true
	srv.Owner = "minio"
	o.Cred.AccessKeyID = "svc"
	res = storagetest.Run(t.Context(), o)
	require.Equal(t, storagetest.Pass, step(t, res, storagetest.StepRoot).Status)
	srv.Owner = "rgwadmin"
	o.Cred.AccessKeyID = "svc"
	res = storagetest.Run(t.Context(), o)
	require.Equal(t, storagetest.Warn, step(t, res, storagetest.StepRoot).Status)
	srv.Owner = "tutti"
	res = storagetest.Run(t.Context(), o)
	require.Equal(t, storagetest.Pass, step(t, res, storagetest.StepRoot).Status)
}

func TestVersioningNotSupported(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	srv.NoVersioning = true
	res := storagetest.Run(t.Context(), opts(srv.URL))
	v := step(t, res, storagetest.StepVersioning)
	require.Equal(t, storagetest.Warn, v.Status)
	require.Contains(t, v.Message, "does not support versioning")
	require.True(t, res.OK)
	require.Empty(t, res.Leftovers)
	// the provider default: no versioning check for "Other"
	o := opts(srv.URL)
	o.Address.Provider = s3.ProviderOther
	res = storagetest.Run(t.Context(), o)
	require.Equal(t, storagetest.Skip, step(t, res, storagetest.StepVersioning).Status)
}

func TestExistingBucketsAccess(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	srv.CreateBucket("app-a")
	srv.CreateBucket("other")
	srv.DenyBuckets["app-a"] = true
	o := opts(srv.URL)
	o.ExistingBuckets = []string{"routed-bucket"}
	o.MatchBucket = func(b string) bool { return strings.HasPrefix(b, "app-") }
	// DenyBuckets denies ListBuckets entries? the bucket is listed by ListBuckets, HeadBucket/List are denied
	res := storagetest.Run(t.Context(), o)
	s := step(t, res, storagetest.StepExisting)
	require.Equal(t, storagetest.Fail, s.Status, "%+v", s)
	require.Contains(t, s.Message, "app-a")
	require.Contains(t, s.Hint, "bucket policy")
	require.Contains(t, s.Hint, "tutti-svc")
	require.Contains(t, s.MissingPermission, "s3:ListBucket on app-a")
	// a referenced bucket that does not exist yet is only a warning
	delete(srv.DenyBuckets, "app-a")
	res = storagetest.Run(t.Context(), o)
	s = step(t, res, storagetest.StepExisting)
	require.Equal(t, storagetest.Warn, s.Status)
	require.Contains(t, s.Message, "routed-bucket")
}

func TestContextCancelled(t *testing.T) {
	srv := fakes3.New()
	defer srv.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	res := storagetest.Run(ctx, opts(srv.URL))
	require.False(t, res.OK)
}
