// Copyright © 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/ratelimit"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3users"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

type RuntimeStorages interface {
	objstore.CredsService
	ManagedStoragesEnabled() bool
	StorageSource(string) string
	GetManaged(string) (objstore.ManagedStorage, bool)
	CreateManaged(context.Context, objstore.ManagedStorage) (objstore.ManagedStorage, error)
	UpdateManaged(context.Context, objstore.ManagedStorage) (objstore.ManagedStorage, error)
	DeleteManaged(context.Context, string) error
	RateLimits() map[string]ratelimit.RateLimit
}

type StorageReferences func(context.Context, string) ([]string, error)

func WithRuntimeStorages(creds RuntimeStorages, refs StorageReferences) ResilienceOption {
	return func(h *resilienceHandlers) { h.runtimeStorages = creds; h.runtimeRefs = refs }
}
func (h *resilienceHandlers) storageManager() (RuntimeStorages, error) {
	if h.runtimeStorages == nil {
		return nil, fmt.Errorf("%w: runtime storage management unavailable", dom.ErrFailedPrecondition)
	}
	return h.runtimeStorages, nil
}
func runtimeStorageFromPb(in *pb.RuntimeStorage) (objstore.ManagedStorage, error) {
	if in == nil {
		return objstore.ManagedStorage{}, fmt.Errorf("%w: storage is required", dom.ErrInvalidArg)
	}
	m := objstore.ManagedStorage{Name: in.Name, Address: s3.StorageAddress{Address: in.Address, Provider: s3.Provider(in.Provider), DefaultRegion: in.DefaultRegion, IsSecure: in.IsSecure}}
	if in.HttpTimeout != nil {
		if err := in.HttpTimeout.CheckValid(); err != nil {
			return m, fmt.Errorf("%w: httpTimeout: %v", dom.ErrInvalidArg, err)
		}
		m.Address.HttpTimeout = in.HttpTimeout.AsDuration()
	}
	if in.HealthCheckInterval != nil {
		if err := in.HealthCheckInterval.CheckValid(); err != nil {
			return m, fmt.Errorf("%w: healthCheckInterval: %v", dom.ErrInvalidArg, err)
		}
		m.Address.HealthCheckInterval = in.HealthCheckInterval.AsDuration()
	}
	if in.RateLimit != nil {
		m.RateLimit = ratelimit.RateLimit{Enabled: in.RateLimit.Enabled, RPM: int(in.RateLimit.Rpm), IncludeMetadataAPI: in.RateLimit.IncludeMetadataApi}
	}
	return m.Normalize()
}
func (h *resilienceHandlers) runtimeStoragePb(ctx context.Context, name string) (*pb.RuntimeStorage, error) {
	c := h.runtimeStorages
	a, err := c.GetS3Address(name)
	if err != nil {
		return nil, err
	}
	rate := c.RateLimits()[name]
	out := &pb.RuntimeStorage{Name: name, Address: a.Address, Provider: string(a.Provider), DefaultRegion: a.DefaultRegion, IsSecure: a.IsSecure, HttpTimeout: durationpb.New(a.HttpTimeout), HealthCheckInterval: durationpb.New(a.HealthCheckInterval), RateLimit: &pb.RuntimeStorageRateLimit{Enabled: rate.Enabled, Rpm: int32(rate.RPM), IncludeMetadataApi: rate.IncludeMetadataAPI}, Source: c.StorageSource(name), IsMain: c.MainStorage() == name}
	if m, ok := c.GetManaged(name); ok {
		out.CreatedAt = timestamppb.New(m.CreatedAt)
		out.UpdatedAt = timestamppb.New(m.UpdatedAt)
	}
	users := c.ListUsers(name)
	sort.Strings(users)
	for _, u := range users {
		cr, err := c.GetS3Credentials(name, u)
		if err != nil {
			return nil, err
		}
		out.Users = append(out.Users, &pb.RuntimeStorageUser{User: u, AccessKeyId: cr.AccessKeyID, Source: c.CredentialSource(name, u)})
	}
	if h.runtimeRefs != nil {
		out.References, err = h.runtimeRefs(ctx, name)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (h *resilienceHandlers) ListRuntimeStorages(ctx context.Context, _ *emptypb.Empty) (*pb.RuntimeStorageList, error) {
	c, err := h.storageManager()
	if err != nil {
		return nil, err
	}
	out := &pb.RuntimeStorageList{MainStorage: c.MainStorage(), ManagedStoragesEnabled: c.ManagedStoragesEnabled()}
	var names []string
	for n, t := range c.Storages() {
		if t == dom.S3 {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		s, err := h.runtimeStoragePb(ctx, n)
		if err != nil {
			return nil, err
		}
		out.Storages = append(out.Storages, s)
	}
	return out, nil
}
func verifyStorage(ctx context.Context, m objstore.ManagedStorage, credential s3.CredentialsV4) error {
	return s3users.MinioVerifier(func(string) (s3.StorageAddress, error) { return m.Address, nil })(ctx, m.Name, credential)
}
func (h *resilienceHandlers) storageCredentials(m objstore.ManagedStorage, in []*pb.RuntimeStorageCredential) (map[string]s3.CredentialsV4, error) {
	creds := map[string]s3.CredentialsV4{}
	for _, c := range in {
		if c == nil || strings.TrimSpace(c.User) == "" || strings.Contains(c.User, ":") {
			return nil, fmt.Errorf("%w: credential user is required and must not contain ':'", dom.ErrInvalidArg)
		}
		if _, exists := creds[c.User]; exists {
			return nil, fmt.Errorf("%w: duplicate credential user %q", dom.ErrInvalidArg, c.User)
		}
		cr := s3.CredentialsV4{AccessKeyID: c.AccessKeyId, SecretAccessKey: c.SecretAccessKey}
		if err := cr.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %v", dom.ErrInvalidArg, err)
		}
		if h.runtimeStorages.CredentialSource(m.Name, c.User) == objstore.CredSourceConfig {
			return nil, fmt.Errorf("%w: config credentials are read-only", dom.ErrFailedPrecondition)
		}
		creds[c.User] = cr
	}
	return creds, nil
}
func (h *resilienceHandlers) verifyStorageCredentials(ctx context.Context, m objstore.ManagedStorage, creds map[string]s3.CredentialsV4, force bool) error {
	if force {
		return nil
	}
	if len(creds) == 0 {
		return fmt.Errorf("%w: provide at least one credential to verify the storage", dom.ErrInvalidArg)
	}
	for user, c := range creds {
		if err := verifyStorage(ctx, m, c); err != nil {
			return fmt.Errorf("%w: verification failed for user %q: %v", dom.ErrFailedPrecondition, user, err)
		}
	}
	return nil
}
func (h *resilienceHandlers) CreateRuntimeStorage(ctx context.Context, req *pb.CreateRuntimeStorageRequest) (*pb.RuntimeStorage, error) {
	c, err := h.storageManager()
	if err != nil {
		return nil, err
	}
	m, err := runtimeStorageFromPb(req.GetStorage())
	if err != nil {
		return nil, err
	}
	if !c.ManagedStoragesEnabled() {
		return nil, fmt.Errorf("%w: enable encrypted dynamic credentials first", dom.ErrFailedPrecondition)
	}
	if _, exists := c.Storages()[m.Name]; exists {
		return nil, fmt.Errorf("%w: storage %q", dom.ErrAlreadyExists, m.Name)
	}
	credentials, err := h.storageCredentials(m, req.GetCredentials())
	if err != nil {
		return nil, err
	}
	if err = h.verifyStorageCredentials(ctx, m, credentials, req.Force); err != nil {
		return nil, err
	}
	_, err = c.CreateManaged(ctx, m)
	if err != nil {
		return nil, err
	}
	for user, cr := range credentials {
		if err = c.SetS3Credentials(ctx, m.Name, user, cr); err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			rollback := c.DeleteManaged(cleanup, m.Name)
			return nil, errors.Join(err, rollback)
		}
	}
	return h.runtimeStoragePb(ctx, m.Name)
}
func (h *resilienceHandlers) UpdateRuntimeStorage(ctx context.Context, req *pb.UpdateRuntimeStorageRequest) (*pb.RuntimeStorage, error) {
	c, err := h.storageManager()
	if err != nil {
		return nil, err
	}
	m, err := runtimeStorageFromPb(req.GetStorage())
	if err != nil {
		return nil, err
	}
	if c.StorageSource(m.Name) != objstore.StorageSourceManaged {
		return nil, fmt.Errorf("%w: only managed storages can be updated", dom.ErrFailedPrecondition)
	}
	if h.runtimeRefs != nil {
		refs, err := h.runtimeRefs(ctx, m.Name)
		if err != nil {
			return nil, err
		}
		if len(refs) > 0 && req.Force {
			return nil, fmt.Errorf("%w: referenced storage updates must pass verification; force is unavailable", dom.ErrFailedPrecondition)
		}
		if len(refs) > 0 && !req.Confirm {
			return nil, fmt.Errorf("%w: storage is referenced (%s); confirm the update", dom.ErrFailedPrecondition, strings.Join(refs, "; "))
		}
	}
	changes, err := h.storageCredentials(m, req.GetCredentials())
	if err != nil {
		return nil, err
	}
	verify := map[string]s3.CredentialsV4{}
	for _, u := range c.ListUsers(m.Name) {
		cr, err := c.GetS3Credentials(m.Name, u)
		if err != nil {
			return nil, err
		}
		verify[u] = cr
	}
	for u, cr := range changes {
		verify[u] = cr
	}
	if err = h.verifyStorageCredentials(ctx, m, verify, req.Force); err != nil {
		return nil, err
	}
	// Keep a rollback snapshot: credential writes can fail independently of the definition DB.
	old, _ := c.GetManaged(m.Name)
	oldCreds := map[string]s3.CredentialsV4{}
	for u := range changes {
		if cr, e := c.GetS3Credentials(m.Name, u); e == nil {
			oldCreds[u] = cr
		}
	}
	if _, err = c.UpdateManaged(ctx, m); err != nil {
		return nil, err
	}
	for u, cr := range changes {
		if err = c.SetS3Credentials(ctx, m.Name, u, cr); err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			var rollback []error
			_, e := c.UpdateManaged(cleanup, old)
			rollback = append(rollback, e)
			for user := range changes {
				if prev, ok := oldCreds[user]; ok {
					rollback = append(rollback, c.SetS3Credentials(cleanup, m.Name, user, prev))
				} else {
					e := c.DeleteUserCredentials(cleanup, m.Name, user)
					if !errors.Is(e, dom.ErrNotFound) {
						rollback = append(rollback, e)
					}
				}
			}
			return nil, errors.Join(append([]error{err}, rollback...)...)
		}
	}
	return h.runtimeStoragePb(ctx, m.Name)
}
func (h *resilienceHandlers) DeleteRuntimeStorage(ctx context.Context, req *pb.RuntimeStorageName) (*emptypb.Empty, error) {
	c, err := h.storageManager()
	if err != nil {
		return nil, err
	}
	if req.GetName() == "" {
		return nil, fmt.Errorf("%w: storage name is required", dom.ErrInvalidArg)
	}
	if req.Name == c.MainStorage() {
		return nil, fmt.Errorf("%w: main storage cannot be deleted", dom.ErrFailedPrecondition)
	}
	if h.runtimeRefs == nil {
		return nil, fmt.Errorf("%w: reference protection unavailable", dom.ErrFailedPrecondition)
	}
	refs, err := h.runtimeRefs(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	if len(refs) > 0 {
		return nil, fmt.Errorf("%w: storage is referenced: %s", dom.ErrFailedPrecondition, strings.Join(refs, "; "))
	}
	if err = c.DeleteManaged(ctx, req.Name); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
func (h *resilienceHandlers) TestRuntimeStorage(ctx context.Context, req *pb.TestRuntimeStorageRequest) (*pb.TestS3UserCredentialsResponse, error) {
	c, err := h.storageManager()
	if err != nil {
		return nil, err
	}
	m, err := runtimeStorageFromPb(req.GetStorage())
	if err != nil {
		return nil, err
	}
	var cr s3.CredentialsV4
	if in := req.Credential; in != nil && in.SecretAccessKey != "" {
		cr = s3.CredentialsV4{AccessKeyID: in.AccessKeyId, SecretAccessKey: in.SecretAccessKey}
	} else {
		user := req.GetCredential().GetUser()
		if user == "" {
			us := c.ListUsers(m.Name)
			sort.Strings(us)
			if len(us) > 0 {
				user = us[0]
			}
		}
		cr, err = c.GetS3Credentials(m.Name, user)
		if err != nil {
			return nil, fmt.Errorf("%w: supply credentials for this connection test", dom.ErrInvalidArg)
		}
	}
	if err = cr.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", dom.ErrInvalidArg, err)
	}
	start := time.Now()
	err = verifyStorage(ctx, m, cr)
	out := &pb.TestS3UserCredentialsResponse{Ok: err == nil, LatencyMs: time.Since(start).Milliseconds()}
	if err != nil {
		out.Error = err.Error()
	}
	return out, nil
}

// ScanStorageUsers finds users even if their storage credentials have been removed.
func ScanStorageUsers(ctx context.Context, rc redis.UniversalClient) ([]string, error) {
	users := map[string]struct{}{}
	for _, prefix := range []string{"p:route_bucket:", "p:repl_bucket_status_index:"} {
		var cursor uint64
		for {
			keys, next, err := rc.Scan(ctx, cursor, prefix+"*", 100).Result()
			if err != nil {
				return nil, err
			}
			for _, k := range keys {
				users[strings.TrimPrefix(k, prefix)] = struct{}{}
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	out := make([]string, 0, len(users))
	for u := range users {
		out = append(out, u)
	}
	return out, nil
}
