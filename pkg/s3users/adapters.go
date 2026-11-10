package s3users

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	mclient "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/failover"
	"github.com/hieunyit/tutti/pkg/s3"
)

// UserReferencesFromRepos checks replications, routings, failover events and protection groups in the control-plane
// repositories for references to user. Can be called inside a controlplane.Tx.
func UserReferencesFromRepos(ctx context.Context, repos controlplane.Repos, user string) ([]string, error) {
	var reasons []string
	statuses, err := repos.Replications.ListStatuses(ctx, user)
	if err != nil {
		return nil, err
	}
	var repl []string
	for id := range statuses {
		if id.User != user {
			continue
		}
		if id.IsUserLevel() {
			repl = append(repl, fmt.Sprintf("%s->%s", id.FromStorage, id.ToStorage))
		} else {
			repl = append(repl, fmt.Sprintf("%s/%s->%s/%s", id.FromStorage, id.FromBucket, id.ToStorage, id.ToBucket))
		}
	}
	if len(repl) > 0 {
		sort.Strings(repl)
		reasons = append(reasons, fmt.Sprintf("%d replication(s) exist (%s): delete them first", len(repl), strings.Join(repl, ", ")))
	}
	userRoutes, err := repos.Routing.ListUserRoutes(ctx)
	if err != nil {
		return nil, err
	}
	if to, ok := userRoutes[user]; ok {
		reasons = append(reasons, fmt.Sprintf("a user routing to storage %q exists: delete it first", to))
	}
	bucketRoutes, err := repos.Routing.ListBucketRoutes(ctx, user)
	if err != nil {
		return nil, err
	}
	if len(bucketRoutes) > 0 {
		reasons = append(reasons, fmt.Sprintf("%d bucket routing(s) exist: delete them first", len(bucketRoutes)))
	}
	events, err := repos.FailoverEvents.List(ctx)
	if err != nil {
		return nil, err
	}
	n := 0
	for _, e := range events {
		if e.Value.User == user {
			n++
		}
	}
	if n > 0 {
		reasons = append(reasons, fmt.Sprintf("%d failover event(s) exist: clear them first", n))
	}
	groups, err := repos.ProtectionGroups.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, g := range groups {
		if g.Value.User == user {
			reasons = append(reasons, "protection group "+g.Value.ID)
		}
	}
	return reasons, nil
}

// VerifyTimeout bounds a single credential check.
const VerifyTimeout = 15 * time.Second

// MinioVerifier checks credentials with an authenticated ListBuckets call. addr resolves the storage address.
func MinioVerifier(addr func(storage string) (s3.StorageAddress, error)) Verifier {
	return func(ctx context.Context, storage string, cred s3.CredentialsV4) error {
		a, err := addr(storage)
		if err != nil {
			return err
		}
		host := strings.TrimPrefix(strings.TrimPrefix(a.Address, "http://"), "https://")
		mc, err := mclient.New(host, &mclient.Options{
			Creds:  credentials.NewStaticV4(cred.AccessKeyID, cred.SecretAccessKey, ""),
			Secure: a.IsSecure,
			Region: a.DefaultRegion,
		})
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(ctx, VerifyTimeout)
		defer cancel()
		if _, err := mc.ListBuckets(ctx); err != nil {
			if er := mclient.ToErrorResponse(err); er.Code != "" {
				return fmt.Errorf("%s: %s", er.Code, er.Message)
			}
			return err
		}
		return nil
	}
}

// PolicyService is the part of the policy service used to find references to a user.
type PolicyService interface {
	ListUserReplicationsInfo(ctx context.Context) (map[entity.UserReplicationPolicy]entity.ReplicationStatusExtended, error)
	ListBucketReplicationsInfo(ctx context.Context, user string) (map[entity.BucketReplicationPolicy]entity.ReplicationStatusExtended, error)
	GetUserRoutings(ctx context.Context) (map[string]string, error)
	GetBucketRoutings(ctx context.Context, user string) (map[string]string, error)
}

// PolicyReferences reports replications, routings and failover events that reference the user.
func PolicyReferences(pol PolicyService, events *failover.Store) References {
	return func(ctx context.Context, user string) ([]string, error) {
		var reasons []string
		userRepl, err := pol.ListUserReplicationsInfo(ctx)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, err
		}
		var repl []string
		for k := range userRepl {
			if k.User == user {
				repl = append(repl, fmt.Sprintf("%s->%s", k.FromStorage, k.ToStorage))
			}
		}
		bucketRepl, err := pol.ListBucketReplicationsInfo(ctx, user)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, err
		}
		for k := range bucketRepl {
			repl = append(repl, fmt.Sprintf("%s/%s->%s/%s", k.FromStorage, k.FromBucket, k.ToStorage, k.ToBucket))
		}
		if len(repl) > 0 {
			sort.Strings(repl)
			reasons = append(reasons, fmt.Sprintf("%d replication(s) exist (%s): delete them first", len(repl), strings.Join(repl, ", ")))
		}
		routings, err := pol.GetUserRoutings(ctx)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, err
		}
		if to, ok := routings[user]; ok {
			reasons = append(reasons, fmt.Sprintf("a user routing to storage %q exists: delete it first", to))
		}
		bucketRoutings, err := pol.GetBucketRoutings(ctx, user)
		if err != nil && !errors.Is(err, dom.ErrNotFound) {
			return nil, err
		}
		if len(bucketRoutings) > 0 {
			reasons = append(reasons, fmt.Sprintf("%d bucket routing(s) exist: delete them first", len(bucketRoutings)))
		}
		if events != nil {
			evs, err := events.ListFailoverEvents(ctx)
			if err != nil && !errors.Is(err, dom.ErrNotFound) {
				return nil, err
			}
			n := 0
			for _, e := range evs {
				if e.User == user {
					n++
				}
			}
			if n > 0 {
				reasons = append(reasons, fmt.Sprintf("%d failover event(s) exist: clear them first", n))
			}
		}
		return reasons, nil
	}
}
