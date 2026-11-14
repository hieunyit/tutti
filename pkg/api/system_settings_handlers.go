package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/systemsettings"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

// DefaultSystemSettings provides fallback defaults when no DB settings exist yet.
func DefaultSystemSettings() controlplane.SystemSettings {
	return controlplane.SystemSettings{
		ID: "system",
		Features: controlplane.SystemFeaturesSettings{
			Tagging:           false,
			ACL:               false,
			Lifecycle:         false,
			Policy:            false,
			PreserveACLGrants: false,
			Versioning:        false,
		},
		Worker: controlplane.SystemWorkerSettings{
			Concurrency:              10,
			ShutdownTimeout:          5 * time.Second,
			QueueUpdateInterval:      3 * time.Second,
			TaskCheckInterval:        1 * time.Second,
			DelayedTaskCheckInterval: 3 * time.Second,
			SwiftRetryInterval:       1 * time.Minute,
			PauseRetryInterval:       30 * time.Second,
			SwitchRetryInterval:      30 * time.Second,
			BacklogHighWatermark:     10000,
			BacklogLowWatermark:      5000,
			BandwidthLimitBytes:      0,                 // 0 = unlimited
			BufferBudgetBytes:        256 * 1024 * 1024, // 256 MiB default
		},
		Lock: controlplane.SystemLockSettings{
			Overlap: 2 * time.Second,
		},
		Webhook: controlplane.SystemWebhookSettings{
			Enabled: false,
			BaseURL: "",
		},
	}
}

func (h *resilienceHandlers) GetSystemSettings(ctx context.Context, _ *emptypb.Empty) (*pb.SystemSettings, error) {
	if h.cpStore != nil {
		v, err := h.cpStore.Repos().SystemSettings.Get(ctx, "system")
		if err == nil {
			return systemSettingsToPb(v), nil
		}
		if !errors.Is(err, dom.ErrNotFound) && !controlplane.IsUnavailable(err) {
			return nil, err
		}
		if controlplane.IsUnavailable(err) {
			zerolog.Ctx(ctx).Warn().Err(err).Msg("system settings: PostgreSQL get failed, falling back to Redis")
		}
	}

	// Fallback to Redis projection if available
	if h.redis != nil {
		raw, err := h.redis.Get(ctx, pg.KeySystemSettings).Result()
		if err == nil && len(raw) > 0 {
			var s controlplane.SystemSettings
			if uerr := json.Unmarshal([]byte(raw), &s); uerr == nil {
				vInt, _ := h.redis.Get(ctx, pg.KeySystemSettingsVersion).Int64()
				if vInt <= 0 {
					vInt = 1
				}
				return systemSettingsToPb(controlplane.Versioned[controlplane.SystemSettings]{
					Value:     s,
					Version:   vInt,
					UpdatedAt: time.Now().UTC(),
				}), nil
			}
		}
	}

	// Default fallback
	defaults := DefaultSystemSettings()
	return systemSettingsToPb(controlplane.Versioned[controlplane.SystemSettings]{
		Value:     defaults,
		Version:   0,
		UpdatedAt: time.Now().UTC(),
	}), nil
}

func (h *resilienceHandlers) UpdateSystemSettings(ctx context.Context, req *pb.UpdateSystemSettingsRequest) (*pb.SystemSettings, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: empty request", dom.ErrInvalidArg)
	}

	current, err := h.currentSystemSettings(ctx)
	if err != nil {
		return nil, err
	}
	target, err := systemSettingsFromPb(req, current)
	if err != nil {
		return nil, err
	}
	target.ID = "system"

	var saved controlplane.Versioned[controlplane.SystemSettings]

	if h.cpStore != nil {
		if req.ExpectedVersion != nil && *req.ExpectedVersion > 0 {
			saved, err = h.cpStore.Repos().SystemSettings.Update(ctx, target, *req.ExpectedVersion)
		} else {
			// First-time or unversioned save: try Put or Update
			saved, err = h.cpStore.Repos().SystemSettings.Put(ctx, target)
		}
		if err != nil {
			return nil, controlplane.WrapWriteError(err)
		}
	} else if h.redis != nil {
		// Redis-only fallback mode
		raw, jerr := json.Marshal(target)
		if jerr != nil {
			return nil, jerr
		}
		pipe := h.redis.TxPipeline()
		pipe.Set(ctx, pg.KeySystemSettings, string(raw), 0)
		pipe.Incr(ctx, pg.KeySystemSettingsVersion)
		cmds, perr := pipe.Exec(ctx)
		if perr != nil {
			return nil, perr
		}
		vInt := cmds[1].(*redis.IntCmd).Val()
		saved = controlplane.Versioned[controlplane.SystemSettings]{
			Value:     target,
			Version:   vInt,
			UpdatedAt: time.Now().UTC(),
		}
	} else {
		return nil, fmt.Errorf("%w: neither control plane database nor redis is available", dom.ErrInternal)
	}

	// Dynamically apply feature flags to in-memory runtime
	features.Set(&features.Config{
		Tagging:           target.Features.Tagging,
		ACL:               target.Features.ACL,
		Lifecycle:         target.Features.Lifecycle,
		Policy:            target.Features.Policy,
		PreserveACLGrants: target.Features.PreserveACLGrants,
		Versioning:        target.Features.Versioning,
	})
	systemsettings.ApplySettings(target)

	// Broadcast change to proxies and workers
	if h.redis != nil {
		_ = h.redis.Publish(ctx, pg.SystemSettingsChannel, fmt.Sprintf("%d", saved.Version)).Err()
		_ = h.redis.Publish(ctx, pg.SnapshotChannel, "changed").Err()
	}

	actor := controlplane.ActorFrom(ctx).Name
	zerolog.Ctx(ctx).Info().
		Str("audit", "system_settings").
		Str("action", "update").
		Str("actor", actor).
		Int64("version", saved.Version).
		Msg("system settings updated")

	return systemSettingsToPb(saved), nil
}

func systemSettingsToPb(v controlplane.Versioned[controlplane.SystemSettings]) *pb.SystemSettings {
	s := v.Value
	out := &pb.SystemSettings{
		Version:   v.Version,
		UpdatedAt: timestamppb.New(v.UpdatedAt),
		Features: &pb.SystemFeaturesSettings{
			Tagging:           s.Features.Tagging,
			Acl:               s.Features.ACL,
			Lifecycle:         s.Features.Lifecycle,
			Policy:            s.Features.Policy,
			PreserveAclGrants: s.Features.PreserveACLGrants,
			Versioning:        s.Features.Versioning,
		},
		Worker: &pb.SystemWorkerSettings{
			Concurrency:              int32(s.Worker.Concurrency),
			ShutdownTimeout:          durationpb.New(s.Worker.ShutdownTimeout),
			QueueUpdateInterval:      durationpb.New(s.Worker.QueueUpdateInterval),
			TaskCheckInterval:        durationpb.New(s.Worker.TaskCheckInterval),
			DelayedTaskCheckInterval: durationpb.New(s.Worker.DelayedTaskCheckInterval),
			SwiftRetryInterval:       durationpb.New(s.Worker.SwiftRetryInterval),
			PauseRetryInterval:       durationpb.New(s.Worker.PauseRetryInterval),
			SwitchRetryInterval:      durationpb.New(s.Worker.SwitchRetryInterval),
			BacklogHighWatermark:     int32(s.Worker.BacklogHighWatermark),
			BacklogLowWatermark:      int32(s.Worker.BacklogLowWatermark),
			BandwidthLimitBytes:      s.Worker.BandwidthLimitBytes,
			BufferBudgetBytes:        s.Worker.BufferBudgetBytes,
			BulkConcurrencyPercent:   int32(s.Worker.BulkConcurrencyPercent),
			MultipartThreads:         int32(s.Worker.MultipartThreads),
			MultipartPartSizeMib:     int32(s.Worker.MultipartPartSizeMiB),
		},
		Lock: &pb.SystemLockSettings{
			Overlap: durationpb.New(s.Lock.Overlap),
		},
		Webhook: &pb.SystemWebhookSettings{
			Enabled: s.Webhook.Enabled,
			BaseUrl: s.Webhook.BaseURL,
		},
	}
	return out
}

// currentSystemSettings returns the stored settings (PostgreSQL, then Redis projection), or defaults if none exist.
func (h *resilienceHandlers) currentSystemSettings(ctx context.Context) (controlplane.SystemSettings, error) {
	if h.cpStore != nil {
		v, err := h.cpStore.Repos().SystemSettings.Get(ctx, "system")
		if err == nil {
			return v.Value, nil
		}
		if !errors.Is(err, dom.ErrNotFound) && !controlplane.IsUnavailable(err) {
			return controlplane.SystemSettings{}, err
		}
	}
	if h.redis != nil {
		raw, err := h.redis.Get(ctx, pg.KeySystemSettings).Result()
		if err == nil && len(raw) > 0 {
			var s controlplane.SystemSettings
			if json.Unmarshal([]byte(raw), &s) == nil {
				return s, nil
			}
		}
	}
	return DefaultSystemSettings(), nil
}

// systemSettingsFromPb merges the request over base: omitted blocks (and zero-valued worker/lock
// numeric fields) keep their current value. Negative values and inverted watermarks are rejected.
func systemSettingsFromPb(req *pb.UpdateSystemSettingsRequest, base controlplane.SystemSettings) (controlplane.SystemSettings, error) {
	out := base

	if req.Features != nil {
		out.Features.Tagging = req.Features.Tagging
		out.Features.ACL = req.Features.Acl
		out.Features.Lifecycle = req.Features.Lifecycle
		out.Features.Policy = req.Features.Policy
		out.Features.PreserveACLGrants = req.Features.PreserveAclGrants
		out.Features.Versioning = req.Features.Versioning
	}

	if req.Worker != nil {
		w := req.Worker
		if w.Concurrency < 0 || w.BacklogHighWatermark < 0 || w.BacklogLowWatermark < 0 || w.BandwidthLimitBytes < 0 || w.BufferBudgetBytes < 0 {
			return out, fmt.Errorf("%w: worker numeric settings must not be negative", dom.ErrInvalidArg)
		}
		if w.BulkConcurrencyPercent != 0 && (w.BulkConcurrencyPercent < 5 || w.BulkConcurrencyPercent > 95) {
			return out, fmt.Errorf("%w: worker.bulkConcurrencyPercent must be 0 (default) or in 5..95: %d", dom.ErrInvalidArg, w.BulkConcurrencyPercent)
		}
		if w.MultipartThreads < 0 || w.MultipartThreads > 16 {
			return out, fmt.Errorf("%w: worker.multipartThreads must be 0 (default) or in 1..16: %d", dom.ErrInvalidArg, w.MultipartThreads)
		}
		if w.MultipartPartSizeMib != 0 && (w.MultipartPartSizeMib < 5 || w.MultipartPartSizeMib > 512) {
			return out, fmt.Errorf("%w: worker.multipartPartSizeMib must be 0 (default) or in 5..512: %d", dom.ErrInvalidArg, w.MultipartPartSizeMib)
		}
		// 0 means "omitted": the stored value is kept like for the other numeric worker fields.
		if w.BulkConcurrencyPercent != 0 {
			out.Worker.BulkConcurrencyPercent = int(w.BulkConcurrencyPercent)
		}
		if w.MultipartThreads != 0 {
			out.Worker.MultipartThreads = int(w.MultipartThreads)
		}
		if w.MultipartPartSizeMib != 0 {
			out.Worker.MultipartPartSizeMiB = int(w.MultipartPartSizeMib)
		}
		if req.Worker.Concurrency > 0 {
			out.Worker.Concurrency = int(req.Worker.Concurrency)
		}
		if d, err := durationPtrFromPb("worker.shutdownTimeout", req.Worker.ShutdownTimeout); err != nil {
			return out, err
		} else if d != nil && *d < 0 {
			return out, fmt.Errorf("%w: durations must not be negative", dom.ErrInvalidArg)
		} else if d != nil && *d > 0 {
			out.Worker.ShutdownTimeout = *d
		}
		if d, err := durationPtrFromPb("worker.queueUpdateInterval", req.Worker.QueueUpdateInterval); err != nil {
			return out, err
		} else if d != nil && *d < 0 {
			return out, fmt.Errorf("%w: durations must not be negative", dom.ErrInvalidArg)
		} else if d != nil && *d > 0 {
			out.Worker.QueueUpdateInterval = *d
		}
		if d, err := durationPtrFromPb("worker.taskCheckInterval", req.Worker.TaskCheckInterval); err != nil {
			return out, err
		} else if d != nil && *d < 0 {
			return out, fmt.Errorf("%w: durations must not be negative", dom.ErrInvalidArg)
		} else if d != nil && *d > 0 {
			out.Worker.TaskCheckInterval = *d
		}
		if d, err := durationPtrFromPb("worker.delayedTaskCheckInterval", req.Worker.DelayedTaskCheckInterval); err != nil {
			return out, err
		} else if d != nil && *d < 0 {
			return out, fmt.Errorf("%w: durations must not be negative", dom.ErrInvalidArg)
		} else if d != nil && *d > 0 {
			out.Worker.DelayedTaskCheckInterval = *d
		}
		if d, err := durationPtrFromPb("worker.swiftRetryInterval", req.Worker.SwiftRetryInterval); err != nil {
			return out, err
		} else if d != nil && *d < 0 {
			return out, fmt.Errorf("%w: durations must not be negative", dom.ErrInvalidArg)
		} else if d != nil && *d > 0 {
			out.Worker.SwiftRetryInterval = *d
		}
		if d, err := durationPtrFromPb("worker.pauseRetryInterval", req.Worker.PauseRetryInterval); err != nil {
			return out, err
		} else if d != nil && *d < 0 {
			return out, fmt.Errorf("%w: durations must not be negative", dom.ErrInvalidArg)
		} else if d != nil && *d > 0 {
			out.Worker.PauseRetryInterval = *d
		}
		if d, err := durationPtrFromPb("worker.switchRetryInterval", req.Worker.SwitchRetryInterval); err != nil {
			return out, err
		} else if d != nil && *d < 0 {
			return out, fmt.Errorf("%w: durations must not be negative", dom.ErrInvalidArg)
		} else if d != nil && *d > 0 {
			out.Worker.SwitchRetryInterval = *d
		}
		if req.Worker.BacklogHighWatermark > 0 {
			out.Worker.BacklogHighWatermark = int(req.Worker.BacklogHighWatermark)
		}
		if req.Worker.BacklogLowWatermark > 0 {
			out.Worker.BacklogLowWatermark = int(req.Worker.BacklogLowWatermark)
		}
		if req.Worker.BandwidthLimitBytes >= 0 {
			out.Worker.BandwidthLimitBytes = req.Worker.BandwidthLimitBytes
		}
		if req.Worker.BufferBudgetBytes >= 0 {
			out.Worker.BufferBudgetBytes = req.Worker.BufferBudgetBytes
		}
	}

	if req.Lock != nil {
		if d, err := durationPtrFromPb("lock.overlap", req.Lock.Overlap); err != nil {
			return out, err
		} else if d != nil && *d < 0 {
			return out, fmt.Errorf("%w: durations must not be negative", dom.ErrInvalidArg)
		} else if d != nil && *d > 0 {
			out.Lock.Overlap = *d
		}
	}

	if req.Webhook != nil {
		out.Webhook.Enabled = req.Webhook.Enabled
		out.Webhook.BaseURL = req.Webhook.BaseUrl
	}

	if out.Worker.BacklogHighWatermark > 0 && out.Worker.BacklogLowWatermark >= out.Worker.BacklogHighWatermark {
		return out, fmt.Errorf("%w: worker.backlogLowWatermark (%d) must be less than backlogHighWatermark (%d)", dom.ErrInvalidArg, out.Worker.BacklogLowWatermark, out.Worker.BacklogHighWatermark)
	}

	return out, nil
}
