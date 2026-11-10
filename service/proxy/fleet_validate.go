package proxy

import (
	"fmt"
	"time"

	"github.com/hieunyit/tutti/pkg/health"
	"github.com/hieunyit/tutti/pkg/proxystatus"
)

// resolveFleet validates the stored fleet document and the effective settings of this proxy (startup -> defaults
// -> group) with the same validators as the YAML config, and returns them. nStorages is the number of configured
// storages (the upper bound of the sync write quorum); syncAvailable tells whether this proxy can synchronize at
// all (it has a Syncer), so that sync is never reported effective when it cannot run.
func resolveFleet(startup proxystatus.Settings, group string, fc *proxystatus.FleetConfig, nStorages int, syncAvailable bool) (eff proxystatus.Settings, pending bool, reasons []string, err error) {
	if fc != nil {
		if err := fc.Validate(); err != nil {
			return eff, false, nil, fmt.Errorf("invalid fleet config: %w", err)
		}
	}
	eff, pending, reasons = proxystatus.Resolve(startup, group, fc)
	if err := validateEffective(eff, nStorages); err != nil {
		return eff, false, nil, fmt.Errorf("invalid effective settings for group %q: %w", group, err)
	}
	if !syncAvailable {
		eff.Replication.Sync.Enabled = false
	}
	return eff, pending, reasons, nil
}

func validateEffective(eff proxystatus.Settings, nStorages int) error {
	repl := ReplicationConfig{
		OnEventError:     eff.Replication.OnEventError,
		EventRetryBudget: eff.Replication.EventRetryBudget,
		Sync: SyncConfig{
			Enabled:         eff.Replication.Sync.Enabled,
			WriteQuorum:     eff.Replication.Sync.WriteQuorum,
			Timeout:         eff.Replication.Sync.Timeout,
			OnQuorumFailure: eff.Replication.Sync.OnQuorumFailure,
		},
	}
	if err := repl.Validate(); err != nil {
		return err
	}
	// writeQuorum counts the main storage and the destinations, all of which are configured storages
	if q := eff.Replication.Sync.WriteQuorum; nStorages > 0 && q > nStorages {
		return fmt.Errorf("replication.sync.writeQuorum %d exceeds the %d configured storages", q, nStorages)
	}
	if err := (health.Config{FailureThreshold: eff.Health.FailureThreshold, OpenDuration: eff.Health.OpenDuration}).Validate(); err != nil {
		return err
	}
	fo := FailoverConfig{
		Enabled:       eff.Failover.Enabled,
		After:         eff.Failover.After,
		MaxEventLag:   eff.Failover.MaxEventLag,
		CheckInterval: eff.Failover.CheckInterval,
	}
	if err := fo.Validate(); err != nil {
		return err
	}
	for name, d := range map[string]time.Duration{
		"health.openDuration":    eff.Health.OpenDuration,
		"failover.checkInterval": eff.Failover.CheckInterval,
		"readyMaxStaleness":      eff.ReadyMaxStaleness,
	} {
		if d < proxystatus.MinFleetInterval {
			return fmt.Errorf("%s must be at least %s", name, proxystatus.MinFleetInterval)
		}
	}
	if eff.DrainDelay < 0 || eff.ReadHeaderTimeout < 0 {
		return fmt.Errorf("drainDelay and readHeaderTimeout must not be negative")
	}
	return nil
}
