package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

func storedSettings() controlplane.SystemSettings {
	s := DefaultSystemSettings()
	s.Features.Tagging = true
	s.Features.Versioning = true
	s.Worker.Concurrency = 42
	s.Worker.ShutdownTimeout = 17 * time.Second
	s.Worker.BacklogHighWatermark = 900
	s.Worker.BacklogLowWatermark = 100
	s.Worker.BufferBudgetBytes = 12345
	s.Lock.Overlap = 9 * time.Second
	s.Webhook.Enabled = true
	s.Webhook.BaseURL = "https://hook.example"
	return s
}

func TestSystemSettingsFromPb_OmittedBlocksKeepStored(t *testing.T) {
	base := storedSettings()
	out, err := systemSettingsFromPb(&pb.UpdateSystemSettingsRequest{}, base)
	require.NoError(t, err)
	require.Equal(t, base, out)

	// Only webhook supplied: features/worker/lock stay at stored values, not defaults.
	out, err = systemSettingsFromPb(&pb.UpdateSystemSettingsRequest{
		Webhook: &pb.SystemWebhookSettings{Enabled: false, BaseUrl: "x"},
	}, base)
	require.NoError(t, err)
	require.Equal(t, base.Features, out.Features)
	require.Equal(t, base.Worker, out.Worker)
	require.Equal(t, base.Lock, out.Lock)
	require.Equal(t, "x", out.Webhook.BaseURL)
	require.False(t, out.Webhook.Enabled)

	// Partial worker block: zero fields keep stored values.
	out, err = systemSettingsFromPb(&pb.UpdateSystemSettingsRequest{
		Worker: &pb.SystemWorkerSettings{Concurrency: 5},
	}, base)
	require.NoError(t, err)
	require.Equal(t, 5, out.Worker.Concurrency)
	require.Equal(t, base.Worker.ShutdownTimeout, out.Worker.ShutdownTimeout)
	require.Equal(t, base.Worker.BacklogHighWatermark, out.Worker.BacklogHighWatermark)
	require.Equal(t, base.Worker.BacklogLowWatermark, out.Worker.BacklogLowWatermark)
}

func TestSystemSettingsFromPb_Invalid(t *testing.T) {
	base := storedSettings()
	neg := durationpb.New(-time.Second)
	cases := map[string]*pb.UpdateSystemSettingsRequest{
		"negative concurrency":      {Worker: &pb.SystemWorkerSettings{Concurrency: -1}},
		"negative high watermark":   {Worker: &pb.SystemWorkerSettings{BacklogHighWatermark: -1}},
		"negative low watermark":    {Worker: &pb.SystemWorkerSettings{BacklogLowWatermark: -1}},
		"negative bandwidth":        {Worker: &pb.SystemWorkerSettings{BandwidthLimitBytes: -1}},
		"negative buffer":           {Worker: &pb.SystemWorkerSettings{BufferBudgetBytes: -1}},
		"negative shutdown timeout": {Worker: &pb.SystemWorkerSettings{ShutdownTimeout: neg}},
		"negative retry interval":   {Worker: &pb.SystemWorkerSettings{SwiftRetryInterval: neg}},
		"negative lock overlap":     {Lock: &pb.SystemLockSettings{Overlap: neg}},
		"low >= high":               {Worker: &pb.SystemWorkerSettings{BacklogHighWatermark: 50, BacklogLowWatermark: 50}},
		"low above stored high":     {Worker: &pb.SystemWorkerSettings{BacklogLowWatermark: 900}},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := systemSettingsFromPb(req, base)
			require.ErrorIs(t, err, dom.ErrInvalidArg)
		})
	}
}

func TestResilience_UpdateSystemSettings_MergeAndValidation(t *testing.T) {
	ctx := context.Background()
	c, _, _ := resilienceClient(t)

	_, err := c.UpdateSystemSettings(ctx, &pb.UpdateSystemSettingsRequest{
		Features: &pb.SystemFeaturesSettings{Tagging: true, Versioning: true},
		Worker: &pb.SystemWorkerSettings{
			Concurrency: 42, BacklogHighWatermark: 900, BacklogLowWatermark: 100,
			ShutdownTimeout: durationpb.New(17 * time.Second),
		},
	})
	require.NoError(t, err)

	// Update with only the webhook block: stored values survive.
	s, err := c.UpdateSystemSettings(ctx, &pb.UpdateSystemSettingsRequest{
		Webhook: &pb.SystemWebhookSettings{Enabled: true, BaseUrl: "https://h"},
	})
	require.NoError(t, err)
	require.True(t, s.Features.Tagging)
	require.True(t, s.Features.Versioning)
	require.EqualValues(t, 42, s.Worker.Concurrency)
	require.EqualValues(t, 900, s.Worker.BacklogHighWatermark)
	require.Equal(t, 17*time.Second, s.Worker.ShutdownTimeout.AsDuration())
	require.Equal(t, "https://h", s.Webhook.BaseUrl)

	bad := []*pb.UpdateSystemSettingsRequest{
		{Worker: &pb.SystemWorkerSettings{Concurrency: -3}},
		{Worker: &pb.SystemWorkerSettings{ShutdownTimeout: durationpb.New(-time.Second)}},
		{Worker: &pb.SystemWorkerSettings{BacklogHighWatermark: 10, BacklogLowWatermark: 10}},
	}
	for _, req := range bad {
		_, err = c.UpdateSystemSettings(ctx, req)
		require.Equal(t, codes.InvalidArgument, status.Code(err), "%v", req)
	}

	// Rejected updates did not change stored state.
	got, err := c.GetSystemSettings(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.EqualValues(t, 42, got.Worker.Concurrency)
}
