package api

import (
	"context"
	"errors"
	"github.com/hieunyit/tutti/pkg/systemsettings"
	"testing"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/tasks"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
	"github.com/stretchr/testify/require"
)

type reviewWebhookCreds struct{ objstore.CredsService }

func (reviewWebhookCreds) Storages() map[string]dom.StorageType {
	return map[string]dom.StorageType{"main": dom.S3, "swift": dom.Swift}
}

type reviewWebhookPolicy struct{ policy.Service }

func (reviewWebhookPolicy) BuildAgentContext(ctx context.Context, _, _ string) (context.Context, error) {
	return ctx, nil
}

type reviewWebhookReplication struct{ calls int }

func (r *reviewWebhookReplication) Replicate(context.Context, string, tasks.ReplicationTask) error {
	r.calls++
	return errors.New("review: Redis queue unavailable")
}

func TestReviewWebhookMustNotACKWhenReplicationQueueUnavailable(t *testing.T) {
	old := systemsettings.Current()
	enabled := old
	enabled.Webhook.Enabled = true
	systemsettings.ApplySettings(enabled)
	t.Cleanup(func() { systemsettings.ApplySettings(old) })
	for _, kind := range []string{"s3", "swift"} {
		t.Run(kind, func(t *testing.T) {
			repl := &reviewWebhookReplication{}
			h := WebhookHandlers(reviewWebhookCreds{}, reviewWebhookPolicy{}, repl, repl)
			var err error
			if kind == "s3" {
				_, err = h.S3Notifications(t.Context(), &pb.S3NotificationRequest{Storage: "main", Records: []*pb.S3EventRecord{{EventName: "s3:ObjectCreated:Put", S3: &pb.S3EventPayload{ConfigurationId: "chorus-user-bucket", Bucket: &pb.S3BucketInfo{Name: "bucket"}, Object: &pb.S3ObjectInfo{Key: "object"}}}}})
			} else {
				_, err = h.SwiftEvents(t.Context(), &pb.SwiftEventsRequest{Storage: "swift", Events: []*pb.SwiftEvent{{Account: "user", Container: "bucket", Object: "object", Operation: pb.SwiftOperation_OBJECT_CREATED}}})
			}
			require.Equal(t, 1, repl.calls, "valid event must reach real handler replication call")
			t.Logf("replication calls=%d, handler error=%v", repl.calls, err)
			require.Error(t, err, "transient queue failure must be exposed so sender can retry")
		})
	}
}
