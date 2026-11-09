/*
 * Copyright © 2026 Clyso GmbH
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package accesspolicy implements per-account authorization for proxy S3 accounts.
//
// A policy is keyed by the Chorus proxy user name (the key of the credentials map, not the access key id):
// storage credentials map access keys to users, and custom credentials (auth.custom) are keyed by user as well.
package accesspolicy

import (
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/s3"
)

// Permission values.
const (
	ReadWrite = "read-write"
	ReadOnly  = "read-only"
	WriteOnly = "write-only"
)

// Policy restricts one proxy account.
type Policy struct {
	// User is the proxy user name.
	User       string `json:"user"`
	Permission string `json:"permission"`
	// Buckets are glob patterns (path.Match). Empty or ["*"] means all buckets.
	Buckets     []string `json:"buckets"`
	Description string   `json:"description,omitempty"`
}

// Default is the effective policy of an account without a stored policy.
func Default(user string) Policy {
	return Policy{User: user, Permission: ReadWrite, Buckets: []string{"*"}}
}

// Validate checks and normalizes the policy (empty bucket list becomes ["*"]).
func (p *Policy) Validate() error {
	if strings.TrimSpace(p.User) == "" {
		return fmt.Errorf("%w: access policy: user is required", dom.ErrInvalidArg)
	}
	switch p.Permission {
	case ReadWrite, ReadOnly, WriteOnly:
	default:
		return fmt.Errorf("%w: access policy: permission must be %q, %q or %q, got %q",
			dom.ErrInvalidArg, ReadWrite, ReadOnly, WriteOnly, p.Permission)
	}
	for _, b := range p.Buckets {
		if b == "" {
			return fmt.Errorf("%w: access policy: empty bucket pattern", dom.ErrInvalidArg)
		}
		if _, err := path.Match(b, "x"); err != nil {
			return fmt.Errorf("%w: access policy: invalid bucket pattern %q: %v", dom.ErrInvalidArg, b, err)
		}
	}
	if len(p.Buckets) == 0 {
		p.Buckets = []string{"*"}
	}
	return nil
}

// AllBuckets reports whether the policy does not restrict buckets.
func (p Policy) AllBuckets() bool {
	if len(p.Buckets) == 0 {
		return true
	}
	for _, b := range p.Buckets {
		if b == "*" {
			return true
		}
	}
	return false
}

// BucketAllowed reports whether the bucket matches the allow-list.
func (p Policy) BucketAllowed(bucket string) bool {
	if p.AllBuckets() {
		return true
	}
	if bucket == "" {
		return false
	}
	for _, pat := range p.Buckets {
		if ok, err := path.Match(pat, bucket); err == nil && ok {
			return true
		}
	}
	return false
}

// Class is the kind of access an S3 method needs.
type Class uint8

const (
	// ClassUnknown is not classified: denied for restricted accounts.
	ClassUnknown Class = iota
	// ClassRead needs read permission.
	ClassRead
	// ClassWrite needs write permission.
	ClassWrite
	// ClassExists is an existence check (HeadBucket/HeadObject), allowed with any permission.
	ClassExists
	// ClassManage is bucket management and needs read-write.
	ClassManage
)

var classes = map[s3.Method]Class{}

func init() {
	set := func(c Class, ms ...s3.Method) {
		for _, m := range ms {
			classes[m] = c
		}
	}
	set(ClassExists, s3.HeadBucket, s3.HeadObject)
	set(ClassRead,
		s3.ListBuckets, s3.GetBucketLocation, s3.GetBucketTagging, s3.GetBucketLifecycle, s3.GetBucketPolicy,
		s3.GetBucketPolicyStatus, s3.GetBucketAcl, s3.GetBucketVersioning, s3.GetBucketWebsite,
		s3.GetBucketReplication, s3.GetBucketNotification, s3.GetBucketEncryption, s3.GetBucketRequestPayment,
		s3.GetBucketMetricsConfiguration, s3.ListBucketMetricsConfiguration,
		s3.GetBucketAnalyticsConfiguration, s3.ListBucketAnalyticsConfiguration,
		s3.GetBucketIntelligentTieringConfiguration, s3.ListBucketIntelligentTieringConfiguration,
		s3.GetBucketInventoryConfiguration, s3.ListBucketInventoryConfiguration,
		s3.GetBucketAccelerateConfiguration, s3.GetBucketLogging, s3.GetBucketOwnershipControls, s3.GetBucketCors,
		s3.GetObject, s3.ListObjects, s3.ListObjectsV2, s3.GetObjectAttributes, s3.ListObjectVersions,
		s3.SelectObjectContent, s3.ListMultipartUploads, s3.ListParts,
		s3.GetObjectTagging, s3.GetObjectAcl, s3.GetPublicAccessBlock, s3.GetObjectRetention,
		s3.GetObjectLegalHold, s3.GetObjectLockConfiguration, s3.GetObjectTorrent,
	)
	set(ClassWrite,
		s3.PutObject, s3.DeleteObject, s3.DeleteObjects, s3.CopyObject, s3.RestoreObject,
		s3.CreateMultipartUpload, s3.UploadPart, s3.CompleteMultipartUpload, s3.AbortMultipartUpload, s3.UploadPartCopy,
		s3.PutObjectTagging, s3.DeleteObjectTagging, s3.PutObjectAcl, s3.PutObjectRetention, s3.PutObjectLegalHold,
	)
	set(ClassManage,
		s3.CreateBucket, s3.DeleteBucket,
		s3.PutBucketTagging, s3.DeleteBucketTagging,
		s3.DeleteBucketLifecycle, s3.PutBucketLifecycle,
		s3.PutBucketPolicy, s3.DeleteBucketPolicy, s3.PutBucketAcl, s3.PutBucketVersioning,
		s3.PutBucketWebsite, s3.DeleteBucketWebsite,
		s3.PutBucketReplication, s3.DeleteBucketReplication,
		s3.DeleteBucketNotification, s3.PutBucketNotification,
		s3.PutBucketEncryption, s3.DeleteBucketEncryption, s3.PutBucketRequestPayment,
		s3.PutBucketMetricsConfiguration, s3.DeleteBucketMetricsConfiguration,
		s3.PutBucketAnalyticsConfiguration, s3.DeleteBucketAnalyticsConfiguration,
		s3.PutBucketIntelligentTieringConfiguration, s3.DeleteBucketIntelligentTieringConfiguration,
		s3.PutBucketInventoryConfiguration, s3.DeleteBucketInventoryConfiguration,
		s3.PutBucketAccelerateConfiguration, s3.PutBucketLogging,
		s3.PutBucketOwnershipControls, s3.DeleteBucketOwnershipControls,
		s3.PutBucketCors, s3.DeleteBucketCors,
		s3.PutPublicAccessBlock, s3.DeletePublicAccessBlock, s3.PutObjectLockConfiguration,
		s3.WriteGetObjectResponse,
	)
}

// Classify returns the access class of the method; ClassUnknown if it is not in the table.
func Classify(m s3.Method) Class { return classes[m] }

// Request is what Authorize needs to know about a request.
type Request struct {
	Method s3.Method
	Bucket string
	// CopySourceBucket is the bucket of x-amz-copy-source for CopyObject/UploadPartCopy.
	CopySourceBucket string
	// CopySourceInvalid is set if the header is present but could not be parsed (denied for restricted accounts).
	CopySourceInvalid bool
}

// Authorize returns nil if the request is allowed, otherwise an error wrapping dom.ErrAuth.
func (p Policy) Authorize(r Request) error {
	switch p.Permission {
	case ReadWrite, ReadOnly, WriteOnly:
	default: // fail closed on a corrupt policy
		return fmt.Errorf("%w: invalid access policy", dom.ErrAuth)
	}
	class := Classify(r.Method)
	switch class {
	case ClassUnknown:
		return fmt.Errorf("%w: method %s is not allowed by the access policy", dom.ErrAuth, r.Method)
	case ClassRead:
		if p.Permission == WriteOnly {
			return fmt.Errorf("%w: account has write-only access", dom.ErrAuth)
		}
	case ClassWrite:
		if p.Permission == ReadOnly {
			return fmt.Errorf("%w: account has read-only access", dom.ErrAuth)
		}
	case ClassManage:
		if p.Permission != ReadWrite {
			return fmt.Errorf("%w: bucket management requires read-write access", dom.ErrAuth)
		}
	case ClassExists:
	}
	if r.Method != s3.ListBuckets && !p.BucketAllowed(r.Bucket) {
		return fmt.Errorf("%w: access to bucket %q is not allowed", dom.ErrAuth, r.Bucket)
	}
	if r.Method == s3.CopyObject || r.Method == s3.UploadPartCopy {
		if r.CopySourceInvalid {
			return fmt.Errorf("%w: unable to parse copy source", dom.ErrAuth)
		}
		if !p.BucketAllowed(r.CopySourceBucket) {
			return fmt.Errorf("%w: access to copy source bucket %q is not allowed", dom.ErrAuth, r.CopySourceBucket)
		}
	}
	return nil
}

// SortPolicies sorts by user.
func SortPolicies(ps []Policy) {
	sort.Slice(ps, func(i, j int) bool { return ps[i].User < ps[j].User })
}
