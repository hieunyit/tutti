package router

import (
	"context"
	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/hieunyit/tutti/pkg/tasks"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type comprehensiveDeleteTransport struct{ s3client.Client }

func (comprehensiveDeleteTransport) Do(req *http.Request) (*http.Response, bool, error) {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`<DeleteResult><Error><Key>same-key</Key><VersionId>forbidden-version</VersionId><Code>AccessDenied</Code></Error></DeleteResult>`))}, false, nil
}

type comprehensiveRouterClients struct{ objstore.Clients }

func (comprehensiveRouterClients) AsS3(context.Context, string, string) (s3client.Client, error) {
	return comprehensiveDeleteTransport{}, nil
}
func TestRegression_QuietMultiDeleteMustRetainSuccessfulVersionEvent(t *testing.T) {
	req := httptest.NewRequest("POST", "http://proxy/bucket?delete", strings.NewReader(`<Delete><Quiet>true</Quiet><Object><Key>same-key</Key><VersionId>deleted-version</VersionId></Object><Object><Key>same-key</Key><VersionId>forbidden-version</VersionId></Object></Delete>`))
	ctx := xctx.SetUser(req.Context(), "u")
	ctx = xctx.SetBucket(ctx, "bucket")
	ctx = xctx.SetRoutingPolicy(ctx, "source")
	req = req.WithContext(ctx)
	s := &s3Router{clients: comprehensiveRouterClients{}}
	resp, tasksOut, _, apiErr, err := s.deleteObjects(req)
	require.NoError(t, err)
	require.False(t, apiErr)
	defer resp.Body.Close()
	require.Len(t, tasksOut, 1, "successful version delete needs a replication event even if another version of the same key failed")
	require.Equal(t, "deleted-version", tasksOut[0].(*tasks.ObjectSyncPayload).Object.Version)
}
