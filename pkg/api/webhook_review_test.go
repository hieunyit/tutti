package api

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/systemsettings"
	"github.com/hieunyit/tutti/pkg/tasks"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

type reviewWebhookCreds struct{ objstore.CredsService }

func (reviewWebhookCreds) Storages() map[string]dom.StorageType {
	return map[string]dom.StorageType{"main": dom.S3, "swift": dom.Swift}
}

type webhookCaptureReplication struct {
	objects []dom.Object
}

func (r *webhookCaptureReplication) Replicate(_ context.Context, _ string, task tasks.ReplicationTask) error {
	r.objects = append(r.objects, task.(*tasks.ObjectSyncPayload).Object)
	return nil
}

func TestS3WebhookDecodesObjectKeyExactlyOnce(t *testing.T) {
	for _, event := range []string{"s3:ObjectCreated:Put", "s3:ObjectRemoved:Delete"} {
		for _, key := range []string{"plain", "reports/red flower+1.jpg", "literal%2Fname", "thư mục/ảnh %.jpg"} {
			t.Run(event+"/"+key, func(t *testing.T) {
				repl := &webhookCaptureReplication{}
				h := &webhookHandlers{s3ReplSvc: repl}
				logger := zerolog.Nop()
				record := &pb.S3EventRecord{EventName: event, S3: &pb.S3EventPayload{Object: &pb.S3ObjectInfo{Key: url.QueryEscape(key)}}}
				require.NoError(t, h.processS3Record(t.Context(), &logger, "main", "bucket", record))
				require.Equal(t, []dom.Object{{Bucket: "bucket", Name: key}}, repl.objects)
			})
		}
	}
	repl := &webhookCaptureReplication{}
	h := &webhookHandlers{s3ReplSvc: repl}
	logger := zerolog.Nop()
	record := &pb.S3EventRecord{EventName: "ObjectCreated:Put", S3: &pb.S3EventPayload{Object: &pb.S3ObjectInfo{Key: "broken%2"}}}
	require.ErrorIs(t, h.processS3Record(t.Context(), &logger, "main", "bucket", record), dom.ErrInvalidArg)
	require.Empty(t, repl.objects, "malformed keys must not reach the replication queue")
}

type reviewWebhookPolicy struct{ policy.Service }

func (reviewWebhookPolicy) BuildAgentContext(ctx context.Context, _, _, _ string) (context.Context, error) {
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
				_, err = h.S3Notifications(t.Context(), &pb.S3NotificationRequest{Storage: "main", Records: []*pb.S3EventRecord{{EventName: "s3:ObjectCreated:Put", S3: &pb.S3EventPayload{ConfigurationId: "tutti-user-bucket", Bucket: &pb.S3BucketInfo{Name: "bucket"}, Object: &pb.S3ObjectInfo{Key: "object"}}}}})
			} else {
				_, err = h.SwiftEvents(t.Context(), &pb.SwiftEventsRequest{Storage: "swift", Events: []*pb.SwiftEvent{{Account: "user", Container: "bucket", Object: "object", Operation: pb.SwiftOperation_OBJECT_CREATED}}})
			}
			require.Equal(t, 1, repl.calls, "valid event must reach real handler replication call")
			t.Logf("replication calls=%d, handler error=%v", repl.calls, err)
			require.Error(t, err, "transient queue failure must be exposed so sender can retry")
		})
	}
}

func TestWebhookBatchLimitExceeded(t *testing.T) {
	old := systemsettings.Current()
	enabled := old
	enabled.Webhook.Enabled = true
	systemsettings.ApplySettings(enabled)
	t.Cleanup(func() { systemsettings.ApplySettings(old) })

	h := WebhookHandlers(reviewWebhookCreds{}, reviewWebhookPolicy{}, nil, nil)
	oversizedRecords := make([]*pb.S3EventRecord, MaxWebhookBatchSize+1)
	for i := range oversizedRecords {
		oversizedRecords[i] = &pb.S3EventRecord{}
	}
	_, err := h.S3Notifications(t.Context(), &pb.S3NotificationRequest{
		Storage: "main",
		Records: oversizedRecords,
	})
	require.Error(t, err)

	oversizedEvents := make([]*pb.SwiftEvent, MaxWebhookBatchSize+1)
	for i := range oversizedEvents {
		oversizedEvents[i] = &pb.SwiftEvent{}
	}
	_, err = h.SwiftEvents(t.Context(), &pb.SwiftEventsRequest{
		Storage: "swift",
		Events:  oversizedEvents,
	})
	require.Error(t, err)
}
