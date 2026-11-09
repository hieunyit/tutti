package auth

import (
	"bytes"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/minio/minio-go/v7/pkg/signer"
	"github.com/stretchr/testify/require"
)

func TestReviewHeaderSignaturesMustRejectStaleRequests(t *testing.T) {
	for _, age := range []time.Duration{0, -10 * time.Minute, 10 * time.Minute, -48 * time.Hour, 48 * time.Hour} {
		t.Run(age.String(), func(t *testing.T) {
			date := time.Now().UTC().Add(age)
			t.Run("v4", func(t *testing.T) {
				r := httptest.NewRequest(http.MethodPut, "http://s3.example.com:9669/bucket/object", bytes.NewReader(nil))
				r = signer.StreamingSignV4(r, presignTestAccessKey, presignTestSecretKey, "", "us-east-1", 0, date, sha256Hasher{sha256.New()})
				_, _, _, err := presignTestMiddleware().authenticate(r)
				if age >= -maxHeaderClockSkew && age <= maxHeaderClockSkew {
					require.NoError(t, err)
				} else {
					require.Error(t, err, "header signature outside freshness window must be rejected")
				}
			})
			t.Run("v2", func(t *testing.T) {
				r := httptest.NewRequest(http.MethodDelete, "http://s3.example.com:9669/bucket/object", nil)
				r.RequestURI = r.URL.RequestURI()
				r.Header.Set("Date", date.Format(http.TimeFormat))
				cred := s3.CredentialsV4{AccessKeyID: presignTestAccessKey, SecretAccessKey: presignTestSecretKey}
				r.Header.Set("Authorization", "AWS "+presignTestAccessKey+":"+signatureV2(cred, r.Method, "/bucket/object", "", r.Header))
				m := presignTestMiddleware()
				m.allowV2 = true
				_, _, _, err := m.authenticate(r)
				if age >= -maxHeaderClockSkew && age <= maxHeaderClockSkew {
					require.NoError(t, err)
				} else {
					require.Error(t, err, "header signature outside freshness window must be rejected")
				}
			})
		})
	}
}

func TestReviewV2RejectsMissingOrMalformedDate(t *testing.T) {
	for _, date := range []string{"", "not-a-date"} {
		r := httptest.NewRequest(http.MethodDelete, "/bucket/object", nil)
		r.Header.Set("Date", date)
		cred := s3.CredentialsV4{AccessKeyID: presignTestAccessKey, SecretAccessKey: presignTestSecretKey}
		r.Header.Set("Authorization", "AWS "+presignTestAccessKey+":"+signatureV2(cred, r.Method, "/bucket/object", "", r.Header))
		m := presignTestMiddleware()
		m.allowV2 = true
		_, _, _, err := m.authenticate(r)
		require.Error(t, err)
	}
}

func TestReviewHeaderFreshnessBoundary(t *testing.T) {
	now := time.Now()
	require.NoError(t, checkHeaderTime(now.Add(-maxHeaderClockSkew), now))
	require.NoError(t, checkHeaderTime(now.Add(maxHeaderClockSkew), now))
	require.Error(t, checkHeaderTime(now.Add(-maxHeaderClockSkew-time.Second), now))
	require.Error(t, checkHeaderTime(now.Add(maxHeaderClockSkew+time.Second), now))
}
