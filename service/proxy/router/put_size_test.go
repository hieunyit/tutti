package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/hieunyit/tutti/pkg/tasks"
)

type singleClients struct {
	objstore.Clients
	c s3client.Client
}

func (s singleClients) AsS3(context.Context, string, string) (s3client.Client, error) {
	return s.c, nil
}

func TestPutObject_SizeFromRequestNoStat(t *testing.T) {
	var heads, puts atomic.Int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/bkt/obj":
			puts.Add(1)
			_, _ = r.Body.Read(make([]byte, 1))
			w.Header().Set("ETag", `"abc"`)
		case r.Method == http.MethodHead && r.URL.Path == "/bkt/obj":
			heads.Add(1)
			w.Header().Set("Content-Length", "777")
			w.Header().Set("ETag", `"abc"`)
			w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<VersioningConfiguration/>`))
		}
	}))
	defer up.Close()
	cl, err := s3client.NewClient(t.Context(), metrics.NewS3Service(false),
		s3.StorageAddress{Address: strings.TrimPrefix(up.URL, "http://"), HttpTimeout: 5e9, DefaultRegion: "us-east-1"},
		s3.CredentialsV4{AccessKeyID: "a", SecretAccessKey: "b"}, "main", "u")
	require.NoError(t, err)
	rt := &s3Router{clients: singleClients{c: cl}}

	newReq := func(body string, hdr map[string]string, cl int64) *http.Request {
		ctx := xctx.SetUser(t.Context(), "u")
		ctx = xctx.SetBucket(ctx, "bkt")
		ctx = xctx.SetObject(ctx, "obj")
		ctx = xctx.SetMethod(ctx, s3.PutObject)
		ctx = xctx.SetRoutingPolicy(ctx, "main")
		req := httptest.NewRequest(http.MethodPut, "/bkt/obj", strings.NewReader(body)).WithContext(ctx)
		req.ContentLength = cl
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		return req
	}
	size := func(tl []tasks.ReplicationTask) int64 { return tl[0].(*tasks.ObjectSyncPayload).ObjSize }

	// known Content-Length
	resp, tl, _, _, err := rt.putObject(newReq("hello", nil, 5))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.EqualValues(t, 5, size(tl))
	require.Zero(t, heads.Load(), "no HEAD for a PUT with known size")

	// aws-chunked: decoded length
	resp, tl, _, _, err = rt.putObject(newReq("xxxxxxxxxxxxxxxxxxxx", map[string]string{
		"X-Amz-Decoded-Content-Length": "11", "Content-Encoding": "aws-chunked"}, 20))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.EqualValues(t, 11, size(tl))
	require.Zero(t, heads.Load())

	// unknown size (chunked, no decoded length): falls back to StatObject
	resp, tl, _, _, err = rt.putObject(newReq("hello", nil, -1))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.EqualValues(t, 777, size(tl))
	require.EqualValues(t, 1, heads.Load())
}
