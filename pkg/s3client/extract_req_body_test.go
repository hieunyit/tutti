package s3client

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mclient "github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
)

type delReq struct {
	Quiet bool `xml:"Quiet"`
}

func TestExtractReqBody(t *testing.T) {
	const doc = `<Delete><Quiet>true</Quiet></Delete>`
	r := httptest.NewRequest(http.MethodPost, "/b?delete", strings.NewReader(doc))
	var got delReq
	require.NoError(t, ExtractReqBody(r, &got))
	require.True(t, got.Quiet)
	rest, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	require.Equal(t, doc, string(rest), "body is restored for forwarding")
}

func TestExtractReqBodyLimit(t *testing.T) {
	pad := func(n int) string { return "<Delete><Quiet>true</Quiet><!--" + strings.Repeat("a", n) + "--></Delete>" }
	overhead := len(pad(0))

	exact := pad(MaxXMLBodySize - overhead)
	require.Equal(t, MaxXMLBodySize, len(exact))
	require.NoError(t, ExtractReqBody(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(exact)), &delReq{}))

	over := pad(MaxXMLBodySize - overhead + 1)
	err := ExtractReqBody(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(over)), &delReq{})
	var s3Err mclient.ErrorResponse
	require.True(t, errors.As(err, &s3Err))
	require.Equal(t, "EntityTooLarge", s3Err.Code)
	require.Equal(t, http.StatusBadRequest, s3Err.StatusCode)

	// the buffer never grows past limit+1 even for huge bodies
	huge := io.LimitReader(zeros{}, 1<<30)
	r := httptest.NewRequest(http.MethodPost, "/", huge)
	require.Error(t, ExtractReqBody(r, &delReq{}))
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }
