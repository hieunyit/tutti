// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"iter"
	"slices"
	"strconv"
	"strings"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/minio/minio-go/v7"
)

// S3Resyncer reconciles current, unversioned objects while the group is frozen. The controller
// calls it only after the member caught up through replication with zero lag, so it normally
// lists both sides and copies nothing (see syncResyncBucket for the per-object cost).
// The caller must hold its coordinator lock, block client and worker admission,
// and drain every admitted writer before calling Resync. Direct writes to the
// underlying storages must also be stopped for the duration.
type S3Resyncer struct {
	open  func(context.Context, string, string) (resyncEndpoint, error)
	scope *accesspolicy.Scope
}

type resyncObjects interface {
	buckets(context.Context) ([]string, error)
	bucketExists(context.Context, string) (bool, error)
	createBucket(context.Context, string) error
	removeBucket(context.Context, string) error
	list(context.Context, string) iter.Seq2[objstore.CommonObjectInfo, error]
	exists(context.Context, string, string) (bool, error)
	remove(context.Context, string, string) error
	get(context.Context, string, string) (io.Reader, error)
	put(context.Context, string, string, io.Reader, uint64) error
}
type commonResyncObjects struct{ objstore.Common }

func (c commonResyncObjects) buckets(ctx context.Context) ([]string, error) {
	return c.ListBuckets(ctx)
}
func (c commonResyncObjects) bucketExists(ctx context.Context, b string) (bool, error) {
	return c.BucketExists(ctx, b)
}
func (c commonResyncObjects) createBucket(ctx context.Context, b string) error {
	return c.CreateBucket(ctx, b)
}
func (c commonResyncObjects) removeBucket(ctx context.Context, b string) error {
	return c.RemoveBucket(ctx, b)
}
func (c commonResyncObjects) list(ctx context.Context, b string) iter.Seq2[objstore.CommonObjectInfo, error] {
	return c.ListObjects(ctx, b)
}
func (c commonResyncObjects) exists(ctx context.Context, b, k string) (bool, error) {
	return c.ObjectExists(ctx, b, k)
}
func (c commonResyncObjects) remove(ctx context.Context, b, k string) error {
	return c.RemoveObject(ctx, b, k)
}
func (c commonResyncObjects) get(ctx context.Context, b, k string) (io.Reader, error) {
	return c.GetObject(ctx, b, k)
}
func (c commonResyncObjects) put(ctx context.Context, b, k string, r io.Reader, n uint64) error {
	return c.PutObject(ctx, b, k, r, n)
}

type resyncEndpoint struct {
	objects   resyncObjects
	versioned func(context.Context, string) (bool, error)
	metadata  func(context.Context, string, string) (minio.ObjectInfo, error)
	put       func(context.Context, string, string, io.Reader, uint64, minio.ObjectInfo) error
}

// NewS3Resyncer preserves object bytes, custom metadata and standard HTTP content
// metadata. Historical versions, bucket configuration, tags and ACLs are outside
// this current-object reconciliation; versioned/suspended buckets are refused.
func NewS3Resyncer(clients objstore.Clients, scope *accesspolicy.Scope) *S3Resyncer {
	return &S3Resyncer{scope: scope, open: func(ctx context.Context, storage, user string) (resyncEndpoint, error) {
		if clients == nil {
			return resyncEndpoint{}, fmt.Errorf("%w: storage clients are required", dom.ErrFailedPrecondition)
		}
		client, err := clients.AsS3(ctx, storage, user)
		if err != nil {
			return resyncEndpoint{}, err
		}
		s3 := client.S3()
		return resyncEndpoint{objects: commonResyncObjects{objstore.WrapS3common(client)},
			versioned: func(ctx context.Context, bucket string) (bool, error) {
				v, err := s3.GetBucketVersioning(ctx, bucket)
				return v.Enabled() || v.Suspended(), err
			},
			metadata: func(ctx context.Context, bucket, key string) (minio.ObjectInfo, error) {
				return s3.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
			},
			put: func(ctx context.Context, bucket, key string, reader io.Reader, size uint64, info minio.ObjectInfo) error {
				_, err := s3.PutObject(ctx, bucket, key, reader, int64(size), minio.PutObjectOptions{
					UserMetadata: info.UserMetadata, ContentType: info.ContentType, ContentEncoding: info.Metadata.Get("Content-Encoding"),
					ContentDisposition: info.Metadata.Get("Content-Disposition"), ContentLanguage: info.Metadata.Get("Content-Language"),
					CacheControl: info.Metadata.Get("Cache-Control"), Expires: info.Expires,
				})
				return err
			},
		}, nil
	}}
}

type resyncBucket struct {
	name           string
	source, target bool
}

func (r *S3Resyncer) endpoints(ctx context.Context, g Group, target string) (resyncEndpoint, resyncEndpoint, error) {
	if target == g.Primary || !slices.Contains(g.Members, target) {
		return resyncEndpoint{}, resyncEndpoint{}, fmt.Errorf("%w: invalid resync target", dom.ErrInvalidArg)
	}
	src, err := r.open(ctx, g.Primary, g.User)
	if err != nil {
		return src, resyncEndpoint{}, err
	}
	dst, err := r.open(ctx, target, g.User)
	return src, dst, err
}

// Validate rejects unsupported bucket versioning before creating/fencing a group.
func (r *S3Resyncer) Validate(ctx context.Context, g Group) error {
	for _, target := range g.Members {
		if target == g.Primary {
			continue
		}
		src, dst, err := r.endpoints(ctx, g, target)
		if err != nil {
			return err
		}
		if _, err = r.plan(ctx, g, target, src, dst); err != nil {
			return err
		}
	}
	return nil
}

func (r *S3Resyncer) plan(ctx context.Context, g Group, target string, src, dst resyncEndpoint) ([]resyncBucket, error) {
	buckets := slices.Clone(g.Buckets)
	if len(buckets) == 0 {
		from, err := src.objects.buckets(ctx)
		if err != nil {
			return nil, err
		}
		to, err := dst.objects.buckets(ctx)
		if err != nil {
			return nil, err
		}
		from, err = r.scope.Filter(ctx, g.Primary, g.User, from)
		if err != nil {
			return nil, err
		}
		to, err = r.scope.Filter(ctx, target, g.User, to)
		if err != nil {
			return nil, err
		}
		buckets = append(from, to...)
		// Both endpoints must permit each bucket when either uses a shared account.
		buckets, err = r.scope.Filter(ctx, g.Primary, g.User, buckets)
		if err != nil {
			return nil, err
		}
		buckets, err = r.scope.Filter(ctx, target, g.User, buckets)
		if err != nil {
			return nil, err
		}
	} else {
		for _, storage := range []string{g.Primary, target} {
			allowed, err := r.scope.Filter(ctx, storage, g.User, buckets)
			if err != nil {
				return nil, err
			}
			if len(allowed) != len(buckets) {
				return nil, fmt.Errorf("%w: group buckets exceed user policy on %s", dom.ErrFailedPrecondition, storage)
			}
		}
	}
	slices.Sort(buckets)
	buckets = slices.Compact(buckets)
	var plan []resyncBucket
	for _, bucket := range buckets {
		a, err := src.objects.bucketExists(ctx, bucket)
		if err != nil {
			return nil, err
		}
		b, err := dst.objects.bucketExists(ctx, bucket)
		if err != nil {
			return nil, err
		}
		for _, ep := range []struct {
			endpoint resyncEndpoint
			exists   bool
			storage  string
		}{{src, a, g.Primary}, {dst, b, target}} {
			if !ep.exists {
				continue
			}
			versioned, err := ep.endpoint.versioned(ctx, bucket)
			if err != nil {
				return nil, fmt.Errorf("verify versioning of %s/%s: %w", ep.storage, bucket, err)
			}
			if versioned {
				return nil, fmt.Errorf("%w: exact group resync does not support enabled or suspended versioning (%s/%s)", dom.ErrFailedPrecondition, ep.storage, bucket)
			}
		}
		plan = append(plan, resyncBucket{bucket, a, b})
	}
	return plan, nil
}

func (r *S3Resyncer) Resync(ctx context.Context, g Group, target string) error {
	if !g.Frozen {
		return fmt.Errorf("%w: exact resync requires frozen group admission", dom.ErrFailedPrecondition)
	}
	src, dst, err := r.endpoints(ctx, g, target)
	if err != nil {
		return err
	}
	plan, err := r.plan(ctx, g, target, src, dst)
	if err != nil {
		return err
	}
	for _, bucket := range plan {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !bucket.source {
			if !bucket.target {
				continue
			}
			for object, err := range dst.objects.list(ctx, bucket.name) {
				if err != nil {
					return err
				}
				if err := dst.objects.remove(ctx, bucket.name, object.Key); err != nil {
					return err
				}
			}
			if err := dst.objects.removeBucket(ctx, bucket.name); err != nil {
				return err
			}
			exists, err := dst.objects.bucketExists(ctx, bucket.name)
			if err != nil {
				return err
			}
			if exists {
				return fmt.Errorf("%w: target-only bucket remains after deletion", dom.ErrFailedPrecondition)
			}
			continue
		}
		if !bucket.target {
			if err := dst.objects.createBucket(ctx, bucket.name); err != nil {
				return err
			}
			versioned, err := dst.versioned(ctx, bucket.name)
			if err != nil {
				return err
			}
			if versioned {
				return fmt.Errorf("%w: new target bucket is versioned", dom.ErrFailedPrecondition)
			}
		}
		if err := syncResyncBucket(ctx, src, dst, bucket.name); err != nil {
			return err
		}
	}
	return nil
}

// syncResyncBucket makes the target bucket equal to the source in ONE merge-join pass over both
// listings (S3 lists keys in ascending binary order; any other order fails closed). Per key:
//   - target only: deleted after the pass (only if both listings were in order), deletion verified;
//   - source only: copied, verified from the digest streamed during the copy;
//   - both: compared by size, then ETag when the ETags decide it, and only otherwise by
//     content hash (each side read at most once); copied when different.
//
// The cost of an already caught-up bucket is therefore two listings, not two full reads of
// every object, which bounds the frozen window by listing time plus the changed bytes.
func syncResyncBucket(ctx context.Context, src, dst resyncEndpoint, bucket string) error {
	ctx, cancel := context.WithCancel(ctx) // stops the listing goroutines on early return
	defer cancel()
	srcNext, srcStop := iter.Pull2(src.objects.list(ctx, bucket))
	defer srcStop()
	dstNext, dstStop := iter.Pull2(dst.objects.list(ctx, bucket))
	defer dstStop()
	pull := func(next func() (objstore.CommonObjectInfo, error, bool), last *string, side string) (objstore.CommonObjectInfo, bool, error) {
		o, err, ok := next()
		if !ok {
			return o, false, nil
		}
		if err != nil {
			return o, false, err
		}
		if *last != "" && o.Key <= *last {
			return o, false, fmt.Errorf("%w: %s listing of %s is not in ascending key order (%q after %q)", dom.ErrFailedPrecondition, side, bucket, o.Key, *last)
		}
		*last = o.Key
		return o, true, nil
	}
	var srcLast, dstLast string
	// Deletions are applied only after both listings were consumed in order, so an out-of-order
	// listing can never delete a target object that the source lists later.
	var targetOnly []string
	s, sok, err := pull(srcNext, &srcLast, "source")
	if err != nil {
		return err
	}
	d, dok, err := pull(dstNext, &dstLast, "target")
	if err != nil {
		return err
	}
	for sok || dok {
		if err := ctx.Err(); err != nil {
			return err
		}
		switch {
		case dok && (!sok || d.Key < s.Key):
			targetOnly = append(targetOnly, d.Key)
			if d, dok, err = pull(dstNext, &dstLast, "target"); err != nil {
				return err
			}
		case sok && (!dok || s.Key < d.Key):
			if err := copyResyncObject(ctx, src, dst, bucket, s); err != nil {
				return err
			}
			if s, sok, err = pull(srcNext, &srcLast, "source"); err != nil {
				return err
			}
		default:
			same, err := sameResyncObject(ctx, src, dst, bucket, s, d)
			if err != nil {
				return err
			}
			if !same {
				if err := copyResyncObject(ctx, src, dst, bucket, s); err != nil {
					return err
				}
			}
			if s, sok, err = pull(srcNext, &srcLast, "source"); err != nil {
				return err
			}
			if d, dok, err = pull(dstNext, &dstLast, "target"); err != nil {
				return err
			}
		}
	}
	for _, key := range targetOnly {
		if err := removeResyncObject(ctx, dst, bucket, key); err != nil {
			return err
		}
	}
	return nil
}

func removeResyncObject(ctx context.Context, dst resyncEndpoint, bucket, key string) error {
	if err := dst.objects.remove(ctx, bucket, key); err != nil {
		return err
	}
	remains, err := dst.objects.exists(ctx, bucket, key)
	if err != nil {
		return err
	}
	if remains {
		return fmt.Errorf("%w: target-only object remains after deletion", dom.ErrFailedPrecondition)
	}
	return nil
}

// md5ETag normalizes an S3 ETag and reports whether it has an MD5-derived form: "<32 hex>" for
// single-part uploads or "<32 hex>-<parts>" for multipart uploads.
func md5ETag(etag string) (string, bool) {
	e := strings.ToLower(strings.Trim(strings.TrimSpace(etag), `"`))
	digest, parts, multipart := strings.Cut(e, "-")
	if len(digest) != 32 {
		return e, false
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return e, false
	}
	if multipart {
		if n, err := strconv.Atoi(parts); err != nil || n < 1 {
			return e, false
		}
	}
	return e, true
}

// sameResyncObject decides equality from listing metadata when it is conclusive:
//   - different sizes: different;
//   - equal MD5-form ETags (single-part or same multipart layout): same content;
//   - two different single-part ETags: different (with SSE the ETag may not be an MD5, so this
//     can only cause an unnecessary copy, never a missed one);
//   - otherwise (multipart with different part layouts, missing/opaque ETags): inconclusive, so
//     both sides are hashed, each exactly once.
//
// Versioned buckets are refused by plan, so version IDs never need to be compared.
func sameResyncObject(ctx context.Context, src, dst resyncEndpoint, bucket string, s, d objstore.CommonObjectInfo) (bool, error) {
	if s.Size != d.Size {
		return false, nil
	}
	se, sok := md5ETag(s.Etag)
	de, dok := md5ETag(d.Etag)
	if sok && dok {
		if se == de {
			return true, nil
		}
		if !strings.Contains(se, "-") && !strings.Contains(de, "-") {
			return false, nil
		}
	}
	a, an, err := hashResyncObject(ctx, src.objects, bucket, s.Key)
	if err != nil {
		return false, err
	}
	b, bn, err := hashResyncObject(ctx, dst.objects, bucket, d.Key)
	if err != nil {
		return false, err
	}
	return an == bn && bytes.Equal(a, b), nil
}

func copyResyncObject(ctx context.Context, src, dst resyncEndpoint, bucket string, object objstore.CommonObjectInfo) error {
	info := minio.ObjectInfo{}
	if src.metadata != nil {
		var err error
		info, err = src.metadata(ctx, bucket, object.Key)
		if err != nil {
			return err
		}
		if info.Size < 0 || uint64(info.Size) != object.Size {
			return fmt.Errorf("%w: source changed during resync", dom.ErrFailedPrecondition)
		}
	}
	reader, err := src.objects.get(ctx, bucket, object.Key)
	if err != nil {
		return err
	}
	if closer, ok := reader.(io.Closer); ok {
		defer closer.Close()
	}
	// The copied bytes are hashed while streaming, so the source is never read a second time to verify.
	sum, md := sha256.New(), md5.New()
	counted := &countingReader{r: io.TeeReader(reader, io.MultiWriter(sum, md))}
	if dst.put != nil {
		err = dst.put(ctx, bucket, object.Key, counted, object.Size, info)
	} else {
		err = dst.objects.put(ctx, bucket, object.Key, counted, object.Size)
	}
	if err != nil {
		return fmt.Errorf("resync %s/%s: %w", bucket, object.Key, err)
	}
	if uint64(counted.n) != object.Size {
		return fmt.Errorf("%w: source size changed during resync of %s/%s", dom.ErrFailedPrecondition, bucket, object.Key)
	}
	return verifyCopiedObject(ctx, dst, bucket, object.Key, object.Size, sum.Sum(nil), hex.EncodeToString(md.Sum(nil)))
}

// verifyCopiedObject checks the written object against the streamed digests: by its ETag when the
// target reports a single-part MD5 ETag equal to the streamed MD5, otherwise by reading it once.
func verifyCopiedObject(ctx context.Context, dst resyncEndpoint, bucket, key string, size uint64, sum []byte, md5Hex string) error {
	if dst.metadata != nil {
		info, err := dst.metadata(ctx, bucket, key)
		if err != nil {
			return err
		}
		if info.Size < 0 || uint64(info.Size) != size {
			return fmt.Errorf("%w: content mismatch after resync of %s/%s", dom.ErrFailedPrecondition, bucket, key)
		}
		if e, ok := md5ETag(info.ETag); ok && e == md5Hex {
			return nil
		}
	}
	got, n, err := hashResyncObject(ctx, dst.objects, bucket, key)
	if err != nil {
		return err
	}
	if uint64(n) != size || !bytes.Equal(got, sum) {
		return fmt.Errorf("%w: content mismatch after resync of %s/%s", dom.ErrFailedPrecondition, bucket, key)
	}
	return nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func hashResyncObject(ctx context.Context, client resyncObjects, bucket, key string) ([]byte, int64, error) {
	reader, err := client.get(ctx, bucket, key)
	if err != nil {
		return nil, 0, err
	}
	hash := sha256.New()
	n, err := io.Copy(hash, reader)
	if closer, ok := reader.(io.Closer); ok {
		err = errors.Join(err, closer.Close())
	}
	return hash.Sum(nil), n, err
}
