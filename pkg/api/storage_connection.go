// Copyright 2026 hieunyit
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/storagetest"
	"github.com/hieunyit/tutti/pkg/store"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

// testTimeout bounds one full connection test.
const testTimeout = 3 * time.Minute

func connectionFromPb(c *pb.ConnectionAccount) s3.CredentialsV4 {
	if c == nil {
		return s3.CredentialsV4{}
	}
	return s3.CredentialsV4{AccessKeyID: c.AccessKeyId, SecretAccessKey: c.SecretAccessKey}
}

func (h *resilienceHandlers) connectionInfoPb(c RuntimeStorages, name string) *pb.ConnectionAccountInfo {
	ci, ok := c.ConnectionAccount(name)
	if !ok {
		return &pb.ConnectionAccountInfo{}
	}
	out := &pb.ConnectionAccountInfo{Configured: true, AccessKeyId: ci.Credentials.AccessKeyID, Source: ci.Source, PreviousAccessKeyId: ci.Previous.AccessKeyID}
	if !ci.RotatedAt.IsZero() {
		out.RotatedAt = timestamppb.New(ci.RotatedAt)
		out.SafeToRevokePreviousAfter = timestamppb.New(ci.SafeToRevokePreviousAfter())
	}
	return out
}

func testResultToPb(r *storagetest.Result) *pb.StorageTestResult {
	if r == nil {
		return nil
	}
	out := &pb.StorageTestResult{
		TestedAt: timestamppb.New(r.TestedAt), Ok: r.OK, HasWarnings: r.HasWarnings, ProbeBucket: r.ProbeBucket,
		Leftovers: r.Leftovers, DurationMs: r.DurationMs, AccessKeyId: r.AccessKeyID, Kind: r.Kind,
	}
	for _, s := range r.Steps {
		out.Steps = append(out.Steps, &pb.StorageTestStep{
			Id: s.ID, Number: int32(s.Number), Name: s.Name, Status: string(s.Status), Call: s.Call, LatencyMs: s.LatencyMs,
			Message: s.Message, MissingPermission: s.MissingPermission, Hint: s.Hint,
		})
	}
	return out
}

func testResultFromJSON(raw string) *pb.StorageTestResult {
	if raw == "" {
		return nil
	}
	var r storagetest.Result
	if json.Unmarshal([]byte(raw), &r) != nil {
		return nil
	}
	return testResultToPb(&r)
}

func (h *resilienceHandlers) saveTest(ctx context.Context, name string, r *storagetest.Result) {
	if r == nil || name == "" {
		return
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return
	}
	bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = h.runtimeStorages.SaveStorageTest(bg, name, raw)
}

func featuresFromPb(o *pb.StorageTestOptions) *storagetest.Features {
	f := o.GetFeatures()
	if f == nil {
		return nil
	}
	return &storagetest.Features{Versioning: f.Versioning, Tagging: f.Tagging, ACL: f.Acl, Lifecycle: f.Lifecycle, Policy: f.Policy}
}

// runStorageTest runs the full connection test. storage is the name of an existing storage ("" for unsaved settings):
// the buckets that routings, replications and access policies reference on it are checked in step 16.
func (h *resilienceHandlers) runStorageTest(ctx context.Context, storage string, addr s3.StorageAddress, cred s3.CredentialsV4, o *pb.StorageTestOptions) (*storagetest.Result, error) {
	if err := cred.Validate(); err != nil {
		return nil, fmt.Errorf("%w: connection account: %w", dom.ErrInvalidArg, err)
	}
	prefix := o.GetProbeBucketPrefix()
	if prefix != "" {
		if err := storagetest.ValidProbePrefix(prefix); err != nil {
			return nil, fmt.Errorf("%w: %w", dom.ErrInvalidArg, err)
		}
	}
	opts := storagetest.Options{
		Address: addr, Cred: cred, ProbeBucketPrefix: prefix, ProbeBucket: o.GetProbeBucket(),
		Features: featuresFromPb(o), Throughput: o.GetThroughput(),
	}
	if storage != "" {
		buckets, matcher, err := h.referencedBuckets(ctx, storage)
		if err != nil {
			return nil, err
		}
		opts.ExistingBuckets, opts.MatchBucket = buckets, matcher
	}
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()
	return storagetest.Run(ctx, opts), nil
}

func failedTestError(what string, r *storagetest.Result) error {
	return fmt.Errorf("%w: %s: connection test failed: %s (run TestStorageConnection for the details, or set force to save anyway)", dom.ErrFailedPrecondition, what, r.Failures())
}

// referencedBuckets returns the buckets that bucket routings and bucket replications place on the storage, and a
// matcher for the buckets of the access policies of the users that reach the storage (users with a bucket routing or
// replication to it; every user for the main storage).
func (h *resilienceHandlers) referencedBuckets(ctx context.Context, storage string) ([]string, func(string) bool, error) {
	if h.redis == nil {
		return nil, nil, nil
	}
	users, err := ScanStorageUsers(ctx, h.redis)
	if err != nil {
		return nil, nil, err
	}
	sort.Strings(users)
	buckets := map[string]struct{}{}
	reaching := map[string]struct{}{}
	for _, u := range users {
		members, err := h.redis.SMembers(ctx, bucketReplIndexPrefix+u).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, nil, err
		}
		for _, m := range members {
			id, err := store.StringToBucketReplicationPolicyConverter(m)
			if err != nil {
				continue
			}
			if id.FromStorage == storage {
				buckets[id.FromBucket] = struct{}{}
				reaching[u] = struct{}{}
			}
			if id.ToStorage == storage {
				buckets[id.ToBucket] = struct{}{}
				reaching[u] = struct{}{}
			}
		}
		routes, err := h.redis.HGetAll(ctx, bucketRoutePrefix+u).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return nil, nil, err
		}
		for b, target := range routes {
			if target == storage {
				buckets[b] = struct{}{}
				reaching[u] = struct{}{}
			}
		}
	}
	var matcher func(string) bool
	if h.policies != nil {
		pols, err := h.policies.List(ctx)
		if err != nil {
			return nil, nil, err
		}
		isMain := h.runtimeStorages != nil && h.runtimeStorages.MainStorage() == storage
		var relevant []string
		for _, p := range pols {
			if p.AllBuckets() {
				continue
			}
			if _, ok := reaching[p.User]; ok || isMain {
				relevant = append(relevant, p.User)
				pp := p
				prev := matcher
				matcher = func(b string) bool { return pp.BucketAllowed(b) || (prev != nil && prev(b)) }
			}
		}
	}
	out := make([]string, 0, len(buckets))
	for b := range buckets {
		out = append(out, b)
	}
	sort.Strings(out)
	return out, matcher, nil
}

// GetRuntimeStorage returns one storage.
func (h *resilienceHandlers) GetRuntimeStorage(ctx context.Context, req *pb.RuntimeStorageName) (*pb.RuntimeStorage, error) {
	c, err := h.storageManager()
	if err != nil {
		return nil, err
	}
	if _, ok := c.Storages()[req.GetName()]; !ok {
		return nil, fmt.Errorf("%w: storage %q", dom.ErrNotFound, req.GetName())
	}
	return h.runtimeStoragePb(ctx, req.Name)
}

// TestStorageConnection runs the connection checks.
func (h *resilienceHandlers) TestStorageConnection(ctx context.Context, req *pb.TestStorageConnectionRequest) (*pb.StorageTestResult, error) {
	c, err := h.storageManager()
	if err != nil {
		return nil, err
	}
	var (
		name string
		addr s3.StorageAddress
		cred = connectionFromPb(req.ConnectionAccount)
	)
	switch {
	case req.GetName() != "":
		name = req.Name
		if t, ok := c.Storages()[name]; !ok || t != dom.S3 {
			return nil, fmt.Errorf("%w: S3 storage %q", dom.ErrNotFound, name)
		}
		if addr, err = c.GetS3Address(name); err != nil {
			return nil, err
		}
		if cred == (s3.CredentialsV4{}) {
			ci, ok := c.ConnectionAccount(name)
			if !ok {
				return nil, fmt.Errorf("%w: storage %q has no connection account: pass connection_account to test one", dom.ErrInvalidArg, name)
			}
			cred = ci.Credentials
		}
	case req.GetStorage() != nil:
		m, err := runtimeStorageFromPb(req.Storage)
		if err != nil {
			return nil, err
		}
		addr = m.Address
	default:
		return nil, fmt.Errorf("%w: pass the name of an existing storage or the storage settings", dom.ErrInvalidArg)
	}
	res, err := h.runStorageTest(ctx, name, addr, cred, req.Options)
	if err != nil {
		return nil, err
	}
	if name != "" {
		// only the result of the stored account becomes the "last test" of the storage
		if ci, ok := c.ConnectionAccount(name); ok && ci.Credentials == cred {
			h.saveTest(ctx, name, res)
		}
	}
	return testResultToPb(res), nil
}

// RotateStorageConnectionAccount tests the new account, then switches to it atomically.
func (h *resilienceHandlers) RotateStorageConnectionAccount(ctx context.Context, req *pb.RotateStorageConnectionAccountRequest) (out *pb.RuntimeStorage, err error) {
	c, err := h.storageManager()
	if err != nil {
		return nil, err
	}
	name := req.GetName()
	if name == "" {
		return nil, fmt.Errorf("%w: storage name is required", dom.ErrInvalidArg)
	}
	next := connectionFromPb(req.NewAccount)
	if err := next.Validate(); err != nil {
		return nil, fmt.Errorf("%w: new_account: %w", dom.ErrInvalidArg, err)
	}
	err = h.storageLocker.With(ctx, []string{name}, func(ctx context.Context) error {
		if c.StorageSource(name) != objstore.StorageSourceManaged {
			return fmt.Errorf("%w: only managed storages can be rotated through the API; change connectionAccount in the config file for config storages", dom.ErrFailedPrecondition)
		}
		addr, err := c.GetS3Address(name)
		if err != nil {
			return err
		}
		res, err := h.runStorageTest(ctx, name, addr, next, req.Options)
		if err != nil {
			return err
		}
		forcedBy := ""
		if !res.OK {
			if !req.Force {
				return failedTestError("rotation refused", res)
			}
			forcedBy = actorName(ctx)
			if forcedBy == "" {
				forcedBy = "unknown"
			}
		}
		if _, err := c.RotateConnectionAccount(ctx, name, next, forcedBy); err != nil {
			return err
		}
		h.saveTest(ctx, name, res)
		out, err = h.runtimeStoragePb(ctx, name)
		return err
	})
	return out, err
}
