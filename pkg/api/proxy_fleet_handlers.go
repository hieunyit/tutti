package api

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

func (h *resilienceHandlers) GetProxyFleetConfig(ctx context.Context, _ *emptypb.Empty) (*pb.ProxyFleetConfig, error) {
	fc, err := proxystatus.NewFleetConfigStore(h.redis).Get(ctx)
	if err != nil {
		return nil, err
	}
	return fleetConfigToPb(fc), nil
}

func (h *resilienceHandlers) SetProxyFleetConfig(ctx context.Context, req *pb.SetProxyFleetConfigRequest) (*pb.ProxyFleetConfig, error) {
	if req == nil {
		req = &pb.SetProxyFleetConfigRequest{}
	}
	defaults, err := settingsOverrideFromPb(req.GetDefaults())
	if err != nil {
		return nil, fmt.Errorf("defaults: %w", err)
	}
	groups := make(map[string]proxystatus.SettingsOverride, len(req.GetGroups()))
	for group, ov := range req.GetGroups() {
		parsed, err := settingsOverrideFromPb(ov)
		if err != nil {
			return nil, fmt.Errorf("group %q: %w", group, err)
		}
		groups[group] = parsed
	}
	saved, err := proxystatus.NewFleetConfigStore(h.redis).Set(ctx, proxystatus.FleetConfig{
		Defaults: defaults,
		Groups:   groups,
	})
	if err != nil {
		return nil, err
	}
	return fleetConfigToPb(saved), nil
}

func fleetConfigToPb(fc *proxystatus.FleetConfig) *pb.ProxyFleetConfig {
	if fc == nil {
		return &pb.ProxyFleetConfig{
			Defaults: &pb.ProxySettingsOverride{},
			Groups:   map[string]*pb.ProxySettingsOverride{},
		}
	}
	out := &pb.ProxyFleetConfig{
		Version:  fc.Version,
		Defaults: settingsOverrideToPb(fc.Defaults),
		Groups:   make(map[string]*pb.ProxySettingsOverride, len(fc.Groups)),
	}
	if !fc.UpdatedAt.IsZero() {
		out.UpdatedAt = timestamppb.New(fc.UpdatedAt)
	}
	for group, ov := range fc.Groups {
		out.Groups[group] = settingsOverrideToPb(ov)
	}
	return out
}

func settingsOverrideToPb(o proxystatus.SettingsOverride) *pb.ProxySettingsOverride {
	out := &pb.ProxySettingsOverride{
		RequireSignedContentType: o.RequireSignedContentType,
		ReadHeaderTimeout:        durationPtrToPb(o.ReadHeaderTimeout),
		WriterLease:              durationPtrToPb(o.WriterLease),
		DrainDelay:               durationPtrToPb(o.DrainDelay),
		ReadyMaxStaleness:        durationPtrToPb(o.ReadyMaxStaleness),
	}
	if r := o.Replication; r != nil {
		out.Replication = &pb.ProxyReplicationSettingsOverride{
			OnEventError:     r.OnEventError,
			EventRetryBudget: durationPtrToPb(r.EventRetryBudget),
		}
		if s := r.Sync; s != nil {
			out.Replication.Sync = &pb.ProxySyncSettingsOverride{
				Enabled:         s.Enabled,
				WriteQuorum:     intPtrToInt32(s.WriteQuorum),
				Timeout:         durationPtrToPb(s.Timeout),
				OnQuorumFailure: s.OnQuorumFailure,
			}
		}
	}
	if h := o.Health; h != nil {
		out.Health = &pb.ProxyHealthSettingsOverride{
			FailureThreshold: intPtrToInt32(h.FailureThreshold),
			OpenDuration:     durationPtrToPb(h.OpenDuration),
		}
	}
	if f := o.Failover; f != nil {
		out.Failover = &pb.ProxyFailoverSettingsOverride{
			Enabled:       f.Enabled,
			CheckInterval: durationPtrToPb(f.CheckInterval),
			After:         durationPtrToPb(f.After),
			MaxEventLag:   intPtrToInt32(f.MaxEventLag),
		}
	}
	return out
}

func settingsOverrideFromPb(in *pb.ProxySettingsOverride) (proxystatus.SettingsOverride, error) {
	if in == nil {
		return proxystatus.SettingsOverride{}, nil
	}
	var out proxystatus.SettingsOverride
	out.RequireSignedContentType = in.RequireSignedContentType

	var err error
	if out.ReadHeaderTimeout, err = durationPtrFromPb("readHeaderTimeout", in.ReadHeaderTimeout); err != nil {
		return out, err
	}
	if out.WriterLease, err = durationPtrFromPb("writerLease", in.WriterLease); err != nil {
		return out, err
	}
	if out.DrainDelay, err = durationPtrFromPb("drainDelay", in.DrainDelay); err != nil {
		return out, err
	}
	if out.ReadyMaxStaleness, err = durationPtrFromPb("readyMaxStaleness", in.ReadyMaxStaleness); err != nil {
		return out, err
	}
	if r := in.Replication; r != nil {
		ro := &proxystatus.ReplicationOverride{
			OnEventError: r.OnEventError,
		}
		if ro.EventRetryBudget, err = durationPtrFromPb("replication.eventRetryBudget", r.EventRetryBudget); err != nil {
			return out, err
		}
		if s := r.Sync; s != nil {
			so := &proxystatus.SyncOverride{
				Enabled:         s.Enabled,
				WriteQuorum:     int32PtrToInt(s.WriteQuorum),
				OnQuorumFailure: s.OnQuorumFailure,
			}
			if so.Timeout, err = durationPtrFromPb("replication.sync.timeout", s.Timeout); err != nil {
				return out, err
			}
			ro.Sync = so
		}
		out.Replication = ro
	}
	if h := in.Health; h != nil {
		ho := &proxystatus.HealthOverride{
			FailureThreshold: int32PtrToInt(h.FailureThreshold),
		}
		if ho.OpenDuration, err = durationPtrFromPb("health.openDuration", h.OpenDuration); err != nil {
			return out, err
		}
		out.Health = ho
	}
	if f := in.Failover; f != nil {
		fo := &proxystatus.FailoverOverride{
			Enabled:     f.Enabled,
			MaxEventLag: int32PtrToInt(f.MaxEventLag),
		}
		if fo.CheckInterval, err = durationPtrFromPb("failover.checkInterval", f.CheckInterval); err != nil {
			return out, err
		}
		if fo.After, err = durationPtrFromPb("failover.after", f.After); err != nil {
			return out, err
		}
		out.Failover = fo
	}
	return out, nil
}

func durationPtrToPb(d *time.Duration) *durationpb.Duration {
	if d == nil {
		return nil
	}
	return durationpb.New(*d)
}

func durationPtrFromPb(field string, d *durationpb.Duration) (*time.Duration, error) {
	if d == nil {
		return nil, nil
	}
	if err := d.CheckValid(); err != nil {
		return nil, fmt.Errorf("%w: invalid %s: %v", dom.ErrInvalidArg, field, err)
	}
	v := d.AsDuration()
	return &v, nil
}

func intPtrToInt32(v *int) *int32 {
	if v == nil {
		return nil
	}
	i := int32(*v)
	return &i
}

func int32PtrToInt(v *int32) *int {
	if v == nil {
		return nil
	}
	i := int(*v)
	return &i
}
