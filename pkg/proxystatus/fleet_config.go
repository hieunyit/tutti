package proxystatus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/dom"
)

const (
	// FleetConfigKey is the Redis key storing the centralized proxy fleet configuration.
	FleetConfigKey = "tutti:proxy-config"
	// DefaultGroup is the proxy group assigned when none is configured.
	DefaultGroup = "default"
	// ConfigVersionKey is the shared control-plane config version key incremented on fleet config updates.
	ConfigVersionKey = "tutti:cfg:version"
	// ConfigChangedChannel is the Redis pub/sub channel notified on fleet config updates.
	ConfigChangedChannel = "tutti:cfg:changed"
)

// MinFleetInterval is the lower bound of fleet-configured loop intervals and durations (health open duration,
// failover check interval, readiness staleness): shorter values only burn CPU or flap.
const MinFleetInterval = 100 * time.Millisecond

// MinWriterLease matches the YAML minimum of protectionGroups.writerLease.
const MinWriterLease = time.Second

// SyncOverride holds optional overrides for synchronous replication settings.
type SyncOverride struct {
	Enabled         *bool          `json:"enabled,omitempty"`
	WriteQuorum     *int           `json:"writeQuorum,omitempty"`
	Timeout         *time.Duration `json:"timeout,omitempty"`
	OnQuorumFailure *string        `json:"onQuorumFailure,omitempty"`
}

// ReplicationOverride holds optional overrides for proxy replication settings.
type ReplicationOverride struct {
	OnEventError     *string        `json:"onEventError,omitempty"`
	EventRetryBudget *time.Duration `json:"eventRetryBudget,omitempty"`
	Sync             *SyncOverride  `json:"sync,omitempty"`
}

// HealthOverride holds optional overrides for storage circuit breaker settings.
type HealthOverride struct {
	FailureThreshold *int           `json:"failureThreshold,omitempty"`
	OpenDuration     *time.Duration `json:"openDuration,omitempty"`
}

// FailoverOverride holds optional overrides for automatic failover controller defaults.
type FailoverOverride struct {
	Enabled       *bool          `json:"enabled,omitempty"`
	CheckInterval *time.Duration `json:"checkInterval,omitempty"`
	After         *time.Duration `json:"after,omitempty"`
	MaxEventLag   *int           `json:"maxEventLag,omitempty"`
}

// SettingsOverride holds optional overrides for proxy settings. Unset (nil) fields inherit from the base layer.
type SettingsOverride struct {
	Replication              *ReplicationOverride `json:"replication,omitempty"`
	Health                   *HealthOverride      `json:"health,omitempty"`
	Failover                 *FailoverOverride    `json:"failover,omitempty"`
	RequireSignedContentType *bool                `json:"requireSignedContentType,omitempty"`
	ReadHeaderTimeout        *time.Duration       `json:"readHeaderTimeout,omitempty"`
	WriterLease              *time.Duration       `json:"writerLease,omitempty"`
	DrainDelay               *time.Duration       `json:"drainDelay,omitempty"`
	ReadyMaxStaleness        *time.Duration       `json:"readyMaxStaleness,omitempty"`
}

// FleetConfig is the centralized proxy fleet configuration stored in Redis under tutti:proxy-config.
type FleetConfig struct {
	Version   int64                       `json:"version"`
	UpdatedAt time.Time                   `json:"updatedAt,omitempty"`
	Defaults  SettingsOverride            `json:"defaults"`
	Groups    map[string]SettingsOverride `json:"groups,omitempty"`
}

// ValidateGroupName checks that name is a valid proxy group identifier.
func ValidateGroupName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: group name must not be empty", dom.ErrInvalidArg)
	}
	if len(name) > 64 {
		return fmt.Errorf("%w: group name %q exceeds 64 characters", dom.ErrInvalidArg, name)
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if i == 0 && !isAlphaNum {
			return fmt.Errorf("%w: group name %q must start with an alphanumeric character", dom.ErrInvalidArg, name)
		}
		if !isAlphaNum && c != '-' && c != '_' && c != '.' {
			return fmt.Errorf("%w: group name %q contains invalid character %q", dom.ErrInvalidArg, name, c)
		}
	}
	return nil
}

// Validate checks that all non-nil override fields have valid values.
func (o SettingsOverride) Validate() error {
	if r := o.Replication; r != nil {
		if r.OnEventError != nil {
			switch *r.OnEventError {
			case "log", "fail":
			default:
				return fmt.Errorf("%w: replication.onEventError must be \"log\" or \"fail\", got %q", dom.ErrInvalidArg, *r.OnEventError)
			}
		}
		if r.EventRetryBudget != nil && *r.EventRetryBudget < 0 {
			return fmt.Errorf("%w: replication.eventRetryBudget must be >= 0", dom.ErrInvalidArg)
		}
		if s := r.Sync; s != nil {
			if s.WriteQuorum != nil && *s.WriteQuorum < 0 {
				return fmt.Errorf("%w: replication.sync.writeQuorum must be >= 0", dom.ErrInvalidArg)
			}
			if s.Timeout != nil && *s.Timeout <= 0 {
				return fmt.Errorf("%w: replication.sync.timeout must be > 0", dom.ErrInvalidArg)
			}
			if s.OnQuorumFailure != nil {
				switch *s.OnQuorumFailure {
				case "fail", "accept":
				default:
					return fmt.Errorf("%w: replication.sync.onQuorumFailure must be \"fail\" or \"accept\", got %q", dom.ErrInvalidArg, *s.OnQuorumFailure)
				}
			}
		}
	}
	if h := o.Health; h != nil {
		if h.FailureThreshold != nil && *h.FailureThreshold < 1 {
			return fmt.Errorf("%w: health.failureThreshold must be >= 1", dom.ErrInvalidArg)
		}
		if h.OpenDuration != nil && *h.OpenDuration < MinFleetInterval {
			return fmt.Errorf("%w: health.openDuration must be >= %s", dom.ErrInvalidArg, MinFleetInterval)
		}
	}
	if f := o.Failover; f != nil {
		if f.CheckInterval != nil && *f.CheckInterval < MinFleetInterval {
			return fmt.Errorf("%w: failover.checkInterval must be >= %s", dom.ErrInvalidArg, MinFleetInterval)
		}
		if f.After != nil && *f.After <= 0 {
			return fmt.Errorf("%w: failover.after must be > 0", dom.ErrInvalidArg)
		}
		if f.MaxEventLag != nil && *f.MaxEventLag < 0 {
			return fmt.Errorf("%w: failover.maxEventLag must be >= 0", dom.ErrInvalidArg)
		}
	}
	if o.ReadHeaderTimeout != nil && *o.ReadHeaderTimeout <= 0 {
		return fmt.Errorf("%w: readHeaderTimeout must be > 0", dom.ErrInvalidArg)
	}
	if o.WriterLease != nil && *o.WriterLease < MinWriterLease {
		return fmt.Errorf("%w: writerLease must be >= %s", dom.ErrInvalidArg, MinWriterLease)
	}
	if o.DrainDelay != nil && *o.DrainDelay < 0 {
		return fmt.Errorf("%w: drainDelay must be >= 0", dom.ErrInvalidArg)
	}
	if o.ReadyMaxStaleness != nil && *o.ReadyMaxStaleness < MinFleetInterval {
		return fmt.Errorf("%w: readyMaxStaleness must be >= %s", dom.ErrInvalidArg, MinFleetInterval)
	}
	return nil
}

// Validate checks that defaults and all per-group overrides are valid.
func (fc FleetConfig) Validate() error {
	if err := fc.Defaults.Validate(); err != nil {
		return fmt.Errorf("defaults: %w", err)
	}
	for group, override := range fc.Groups {
		if err := ValidateGroupName(group); err != nil {
			return err
		}
		if err := override.Validate(); err != nil {
			return fmt.Errorf("group %q: %w", group, err)
		}
	}
	return nil
}

// Resolve computes the effective proxy settings for a proxy instance belonging to group,
// layering startup -> fc.Defaults -> fc.Groups[group].
// It also reports whether a proxy restart is required for non-hot-reloadable settings to take effect.
func Resolve(startup Settings, group string, fc *FleetConfig) (effective Settings, pendingRestart bool, reasons []string) {
	effective = startup
	if group == "" {
		group = DefaultGroup
	}
	if fc != nil {
		applyOverride(&effective, fc.Defaults)
		if grpOverride, ok := fc.Groups[group]; ok {
			applyOverride(&effective, grpOverride)
		}
	}
	if effective.ReadHeaderTimeout != startup.ReadHeaderTimeout {
		reasons = append(reasons, "readHeaderTimeout")
	}
	if effective.WriterLease != startup.WriterLease {
		reasons = append(reasons, "protectionGroups.writerLease")
	}
	pendingRestart = len(reasons) > 0
	return effective, pendingRestart, reasons
}

func applyOverride(dst *Settings, o SettingsOverride) {
	if r := o.Replication; r != nil {
		if r.OnEventError != nil {
			dst.Replication.OnEventError = *r.OnEventError
		}
		if r.EventRetryBudget != nil {
			dst.Replication.EventRetryBudget = *r.EventRetryBudget
		}
		if s := r.Sync; s != nil {
			if s.Enabled != nil {
				dst.Replication.Sync.Enabled = *s.Enabled
			}
			if s.WriteQuorum != nil {
				dst.Replication.Sync.WriteQuorum = *s.WriteQuorum
			}
			if s.Timeout != nil {
				dst.Replication.Sync.Timeout = *s.Timeout
			}
			if s.OnQuorumFailure != nil {
				dst.Replication.Sync.OnQuorumFailure = *s.OnQuorumFailure
			}
		}
	}
	if h := o.Health; h != nil {
		if h.FailureThreshold != nil {
			dst.Health.FailureThreshold = *h.FailureThreshold
		}
		if h.OpenDuration != nil {
			dst.Health.OpenDuration = *h.OpenDuration
		}
	}
	if f := o.Failover; f != nil {
		if f.Enabled != nil {
			dst.Failover.Enabled = *f.Enabled
		}
		if f.CheckInterval != nil {
			dst.Failover.CheckInterval = *f.CheckInterval
		}
		if f.After != nil {
			dst.Failover.After = *f.After
		}
		if f.MaxEventLag != nil {
			dst.Failover.MaxEventLag = *f.MaxEventLag
		}
	}
	if o.RequireSignedContentType != nil {
		dst.RequireSignedContentType = *o.RequireSignedContentType
	}
	if o.ReadHeaderTimeout != nil {
		dst.ReadHeaderTimeout = *o.ReadHeaderTimeout
	}
	if o.WriterLease != nil {
		dst.WriterLease = *o.WriterLease
	}
	if o.DrainDelay != nil {
		dst.DrainDelay = *o.DrainDelay
	}
	if o.ReadyMaxStaleness != nil {
		dst.ReadyMaxStaleness = *o.ReadyMaxStaleness
	}
}

var casSetFleetConfigScript = redis.NewScript(`
local cur = redis.call('GET', KEYS[1])
if (not cur and ARGV[1] ~= '') or (cur and cur ~= ARGV[1]) then
	return 0
end
redis.call('SET', KEYS[1], ARGV[2])
local v = redis.call('INCR', KEYS[2])
redis.call('PUBLISH', ARGV[3], tostring(v))
return 1
`)

// FleetConfigStore reads and updates the centralized proxy fleet configuration in Redis.
type FleetConfigStore struct {
	client redis.Cmdable
}

// NewFleetConfigStore creates a FleetConfigStore backed by Redis.
func NewFleetConfigStore(client redis.Cmdable) *FleetConfigStore {
	return &FleetConfigStore{client: client}
}

// Get returns the current FleetConfig, or an empty version-0 FleetConfig if none has been saved yet.
func (s *FleetConfigStore) Get(ctx context.Context) (*FleetConfig, error) {
	raw, err := s.client.Get(ctx, FleetConfigKey).Result()
	if errors.Is(err, redis.Nil) {
		return &FleetConfig{
			Version: 0,
			Groups:  map[string]SettingsOverride{},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg FleetConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", FleetConfigKey, err)
	}
	if cfg.Groups == nil {
		cfg.Groups = map[string]SettingsOverride{}
	}
	return &cfg, nil
}

// Set validates cfg, atomically increments Version, persists the FleetConfig under tutti:proxy-config,
// increments tutti:cfg:version, and publishes on tutti:cfg:changed.
func (s *FleetConfigStore) Set(ctx context.Context, cfg FleetConfig) (*FleetConfig, error) {
	_, next, err := s.SetExpected(ctx, cfg, nil)
	return next, err
}

// SetExpected is Set with optimistic concurrency: when expected is non-nil and differs from the stored version
// the update is rejected with dom.ErrFailedPrecondition. It returns the previous (version 0 when none) and the
// saved configuration.
func (s *FleetConfigStore) SetExpected(ctx context.Context, cfg FleetConfig, expected *int64) (prev, saved *FleetConfig, err error) {
	if err := cfg.Validate(); err != nil {
		return nil, nil, err
	}
	if cfg.Groups == nil {
		cfg.Groups = map[string]SettingsOverride{}
	}
	const maxAttempts = 16
	for attempt := 0; attempt < maxAttempts; attempt++ {
		raw, err := s.client.Get(ctx, FleetConfigKey).Result()
		cur := &FleetConfig{Groups: map[string]SettingsOverride{}}
		if errors.Is(err, redis.Nil) {
			raw = ""
		} else if err != nil {
			return nil, nil, err
		} else if err := json.Unmarshal([]byte(raw), cur); err != nil {
			return nil, nil, fmt.Errorf("unmarshal %s: %w", FleetConfigKey, err)
		}
		if expected != nil && *expected != cur.Version {
			return nil, nil, fmt.Errorf("%w: fleet config was changed by someone else (stored version %d, expected %d): reload and retry",
				dom.ErrFailedPrecondition, cur.Version, *expected)
		}
		next := FleetConfig{
			Version:   cur.Version + 1,
			UpdatedAt: time.Now().UTC(),
			Defaults:  cfg.Defaults,
			Groups:    cfg.Groups,
		}
		b, err := json.Marshal(next)
		if err != nil {
			return nil, nil, err
		}
		ok, err := casSetFleetConfigScript.Run(
			ctx,
			s.client,
			[]string{FleetConfigKey, ConfigVersionKey},
			raw,
			string(b),
			ConfigChangedChannel,
		).Int()
		if err != nil {
			return nil, nil, err
		}
		if ok == 1 {
			return cur, &next, nil
		}
	}
	return nil, nil, fmt.Errorf("concurrent modification of %s", FleetConfigKey)
}

// FleetAuditKey is the Redis list holding the most recent fleet configuration changes (newest first).
const FleetAuditKey = "tutti:proxy-config:audit"

// FleetAuditMax caps FleetAuditKey.
const FleetAuditMax = 200

// FleetAuditRecord is one audited fleet configuration change.
type FleetAuditRecord struct {
	At     time.Time    `json:"at"`
	Actor  string       `json:"actor"`
	Before *FleetConfig `json:"before"`
	After  *FleetConfig `json:"after"`
}

// RecordAudit appends rec to the capped audit list. The fleet configuration holds no secrets.
func (s *FleetConfigStore) RecordAudit(ctx context.Context, rec FleetAuditRecord) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.LPush(ctx, FleetAuditKey, string(b))
	pipe.LTrim(ctx, FleetAuditKey, 0, FleetAuditMax-1)
	_, err = pipe.Exec(ctx)
	return err
}

// ListAudit returns up to limit recent audit records, newest first.
func (s *FleetConfigStore) ListAudit(ctx context.Context, limit int) ([]FleetAuditRecord, error) {
	if limit <= 0 || limit > FleetAuditMax {
		limit = FleetAuditMax
	}
	items, err := s.client.LRange(ctx, FleetAuditKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]FleetAuditRecord, 0, len(items))
	for _, it := range items {
		var r FleetAuditRecord
		if err := json.Unmarshal([]byte(it), &r); err == nil {
			out = append(out, r)
		}
	}
	return out, nil
}
