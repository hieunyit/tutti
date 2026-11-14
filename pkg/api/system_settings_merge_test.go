package api

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
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
		Worker: &pb.SystemWorkerSettings{Concurrency: proto.Int32(5)},
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
		"negative concurrency":      {Worker: &pb.SystemWorkerSettings{Concurrency: proto.Int32(-1)}},
		"negative high watermark":   {Worker: &pb.SystemWorkerSettings{BacklogHighWatermark: proto.Int32(-1)}},
		"negative low watermark":    {Worker: &pb.SystemWorkerSettings{BacklogLowWatermark: proto.Int32(-1)}},
		"negative bandwidth":        {Worker: &pb.SystemWorkerSettings{BandwidthLimitBytes: proto.Int64(-1)}},
		"negative buffer":           {Worker: &pb.SystemWorkerSettings{BufferBudgetBytes: proto.Int64(-1)}},
		"negative shutdown timeout": {Worker: &pb.SystemWorkerSettings{ShutdownTimeout: neg}},
		"negative retry interval":   {Worker: &pb.SystemWorkerSettings{SwiftRetryInterval: neg}},
		"negative lock overlap":     {Lock: &pb.SystemLockSettings{Overlap: neg}},
		"low >= high":               {Worker: &pb.SystemWorkerSettings{BacklogHighWatermark: proto.Int32(50), BacklogLowWatermark: proto.Int32(50)}},
		"bulk percent too low":      {Worker: &pb.SystemWorkerSettings{BulkConcurrencyPercent: proto.Int32(4)}},
		"bulk percent too high":     {Worker: &pb.SystemWorkerSettings{BulkConcurrencyPercent: proto.Int32(96)}},
		"bulk percent negative":     {Worker: &pb.SystemWorkerSettings{BulkConcurrencyPercent: proto.Int32(-5)}},
		"multipart threads high":    {Worker: &pb.SystemWorkerSettings{MultipartThreads: proto.Int32(17)}},
		"multipart threads neg":     {Worker: &pb.SystemWorkerSettings{MultipartThreads: proto.Int32(-1)}},
		"part size too small":       {Worker: &pb.SystemWorkerSettings{MultipartPartSizeMib: proto.Int32(4)}},
		"part size too large":       {Worker: &pb.SystemWorkerSettings{MultipartPartSizeMib: proto.Int32(513)}},
		"low above stored high":     {Worker: &pb.SystemWorkerSettings{BacklogLowWatermark: proto.Int32(900)}},
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
			Concurrency: proto.Int32(42), BacklogHighWatermark: proto.Int32(900), BacklogLowWatermark: proto.Int32(100),
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
	require.EqualValues(t, 42, s.Worker.GetConcurrency())
	require.EqualValues(t, 900, s.Worker.GetBacklogHighWatermark())
	require.Equal(t, 17*time.Second, s.Worker.ShutdownTimeout.AsDuration())
	require.Equal(t, "https://h", s.Webhook.BaseUrl)

	bad := []*pb.UpdateSystemSettingsRequest{
		{Worker: &pb.SystemWorkerSettings{Concurrency: proto.Int32(-3)}},
		{Worker: &pb.SystemWorkerSettings{ShutdownTimeout: durationpb.New(-time.Second)}},
		{Worker: &pb.SystemWorkerSettings{BacklogHighWatermark: proto.Int32(10), BacklogLowWatermark: proto.Int32(10)}},
	}
	for _, req := range bad {
		_, err = c.UpdateSystemSettings(ctx, req)
		require.Equal(t, codes.InvalidArgument, status.Code(err), "%v", req)
	}

	// Rejected updates did not change stored state.
	got, err := c.GetSystemSettings(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.EqualValues(t, 42, got.Worker.GetConcurrency())
}

func TestSystemSettingsFromPb_TuningKnobs(t *testing.T) {
	base := storedSettings()
	out, err := systemSettingsFromPb(&pb.UpdateSystemSettingsRequest{Worker: &pb.SystemWorkerSettings{
		BulkConcurrencyPercent: proto.Int32(5), MultipartThreads: proto.Int32(16), MultipartPartSizeMib: proto.Int32(512)}}, base)
	require.NoError(t, err)
	require.Equal(t, []int{5, 16, 512}, []int{out.Worker.BulkConcurrencyPercent, out.Worker.MultipartThreads, out.Worker.MultipartPartSizeMiB})

	// omitted (0) keeps the stored values
	base = out
	out, err = systemSettingsFromPb(&pb.UpdateSystemSettingsRequest{Worker: &pb.SystemWorkerSettings{Concurrency: proto.Int32(7)}}, base)
	require.NoError(t, err)
	require.Equal(t, []int{5, 16, 512}, []int{out.Worker.BulkConcurrencyPercent, out.Worker.MultipartThreads, out.Worker.MultipartPartSizeMiB})
}

func workerFromJSON(t *testing.T, body string) *pb.UpdateSystemSettingsRequest {
	t.Helper()
	// Same unmarshal options as the grpc-gateway mux in grpc_http_gateway.go.
	req := &pb.UpdateSystemSettingsRequest{}
	require.NoError(t, protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal([]byte(body), req))
	return req
}

func TestSystemSettingsFromPb_PartialWorkerKeepsStored(t *testing.T) {
	base := DefaultSystemSettings()
	base.Worker.BandwidthLimitBytes = 5_000_000
	base.Worker.BufferBudgetBytes = 256 << 20
	base.Worker.BulkConcurrencyPercent = 30
	base.Worker.MultipartPartSizeMiB = 32

	// JSON body with only multipartThreads: every other knob must stay absent and keep stored.
	req := workerFromJSON(t, `{"worker":{"multipartThreads":4}}`)
	w := req.Worker
	require.Nil(t, w.BandwidthLimitBytes)
	require.Nil(t, w.BufferBudgetBytes)
	require.Nil(t, w.Concurrency)
	require.Nil(t, w.BacklogHighWatermark)
	require.Nil(t, w.BacklogLowWatermark)
	require.Nil(t, w.BulkConcurrencyPercent)
	require.Nil(t, w.MultipartPartSizeMib)

	out, err := systemSettingsFromPb(req, base)
	require.NoError(t, err)
	want := base
	want.Worker.MultipartThreads = 4
	require.Equal(t, want, out)
}

func TestSystemSettingsFromPb_ExplicitZero(t *testing.T) {
	base := DefaultSystemSettings()
	base.Worker.BandwidthLimitBytes = 5_000_000
	base.Worker.MultipartThreads = 4
	base.Worker.MultipartPartSizeMiB = 32
	base.Worker.BulkConcurrencyPercent = 30
	base.Worker.Concurrency = 42
	base.Worker.BacklogHighWatermark = 900
	base.Worker.BacklogLowWatermark = 100

	out, err := systemSettingsFromPb(workerFromJSON(t, `{"worker":{"multipartThreads":0,"bandwidthLimitBytes":"0","multipartPartSizeMib":0,"bulkConcurrencyPercent":0,"concurrency":0,"backlogHighWatermark":0,"backlogLowWatermark":0}}`), base)
	require.NoError(t, err)
	def := DefaultSystemSettings().Worker
	require.Equal(t, 0, out.Worker.MultipartThreads)
	require.EqualValues(t, 0, out.Worker.BandwidthLimitBytes)
	require.Equal(t, 0, out.Worker.MultipartPartSizeMiB)
	require.Equal(t, 0, out.Worker.BulkConcurrencyPercent)
	require.Equal(t, def.Concurrency, out.Worker.Concurrency)
	require.Equal(t, def.BacklogHighWatermark, out.Worker.BacklogHighWatermark)
	require.Equal(t, def.BacklogLowWatermark, out.Worker.BacklogLowWatermark)
	require.Equal(t, base.Worker.BufferBudgetBytes, out.Worker.BufferBudgetBytes)
}

func TestSystemSettingsFromPb_BufferBudgetMinimum(t *testing.T) {
	base := DefaultSystemSettings()
	for _, v := range []int64{0, 1, 16<<20 - 1} {
		_, err := systemSettingsFromPb(&pb.UpdateSystemSettingsRequest{Worker: &pb.SystemWorkerSettings{BufferBudgetBytes: proto.Int64(v)}}, base)
		require.ErrorIs(t, err, dom.ErrInvalidArg, "%d", v)
	}
	out, err := systemSettingsFromPb(&pb.UpdateSystemSettingsRequest{Worker: &pb.SystemWorkerSettings{BufferBudgetBytes: proto.Int64(16 << 20)}}, base)
	require.NoError(t, err)
	require.EqualValues(t, 16<<20, out.Worker.BufferBudgetBytes)
}

func TestResilience_UpdateSystemSettings_PartialPutRoundTrip(t *testing.T) {
	ctx := context.Background()
	c, _, _ := resilienceClient(t)

	_, err := c.UpdateSystemSettings(ctx, &pb.UpdateSystemSettingsRequest{Worker: &pb.SystemWorkerSettings{
		BandwidthLimitBytes: proto.Int64(1000), BufferBudgetBytes: proto.Int64(256 << 20)}})
	require.NoError(t, err)

	_, err = c.UpdateSystemSettings(ctx, &pb.UpdateSystemSettingsRequest{Worker: &pb.SystemWorkerSettings{MultipartThreads: proto.Int32(4)}})
	require.NoError(t, err)
	got, err := c.GetSystemSettings(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.EqualValues(t, 1000, got.Worker.GetBandwidthLimitBytes())
	require.EqualValues(t, 256<<20, got.Worker.GetBufferBudgetBytes())
	require.EqualValues(t, 4, got.Worker.GetMultipartThreads())

	_, err = c.UpdateSystemSettings(ctx, &pb.UpdateSystemSettingsRequest{Worker: &pb.SystemWorkerSettings{BufferBudgetBytes: proto.Int64(0)}})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = c.UpdateSystemSettings(ctx, &pb.UpdateSystemSettingsRequest{Worker: &pb.SystemWorkerSettings{
		MultipartThreads: proto.Int32(0), BandwidthLimitBytes: proto.Int64(0)}})
	require.NoError(t, err)
	got, err = c.GetSystemSettings(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.EqualValues(t, 0, got.Worker.GetMultipartThreads())
	require.EqualValues(t, 0, got.Worker.GetBandwidthLimitBytes())
	require.EqualValues(t, 256<<20, got.Worker.GetBufferBudgetBytes())
}
