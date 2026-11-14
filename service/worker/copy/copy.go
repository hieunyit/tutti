// Copyright 2025 Clyso GmbH
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

package copy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"time"

	awss3 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/entity"
	"github.com/hieunyit/tutti/pkg/features"
	"github.com/hieunyit/tutti/pkg/metrics"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/hieunyit/tutti/pkg/systemsettings"
	"github.com/hieunyit/tutti/pkg/util"
	"github.com/hieunyit/tutti/pkg/workerstats"
)

// TODO:
// swift client metrics
const (
	CTuttiSourceVersionIDMetaHeader = "x-amz-meta-tutti-source-version-id"

	CRGWVersionMigrationQueryParam = "rgwx-version-id"
)

type Bucket struct {
	Storage string
	Bucket  string
}

func NewBucket(storage string, bucket string) Bucket {
	return Bucket{
		Storage: storage,
		Bucket:  bucket,
	}
}

type File struct {
	Storage string
	Bucket  string
	Name    string
	Version string
}

func NewVersionedFile(storage string, bucket string, name string, version string) File {
	return File{
		Storage: storage,
		Bucket:  bucket,
		Name:    name,
		Version: version,
	}
}

func NewFile(storage string, bucket string, name string) File {
	return File{
		Storage: storage,
		Bucket:  bucket,
		Name:    name,
	}
}

type ObjectInfo struct {
	LastModified time.Time
	Key          string
	VersionID    string
	Etag         string
	StorageClass string
	Size         uint64
}

type CopySvc interface {
	GetVersionInfo(ctx context.Context, user string, to File) ([]entity.ObjectVersionInfo, error)
	ClearDestination(ctx context.Context, user string, to File, versioned bool) error
	GetLastMigratedVersionInfo(ctx context.Context, user string, to File) (entity.ObjectVersionInfo, error)
	CopyObject(ctx context.Context, user string, from File, to File) error
	CopyACLs(ctx context.Context, user string, from File, to File) error
}

var _ CopySvc = (*S3CopySvc)(nil)

type S3CopySvc struct {
	credsSvc       objstore.CredsService
	clientRegistry objstore.Clients
	metricsSvc     metrics.WorkerService
}

func NewS3CopySvc(credsSvc objstore.CredsService, clientRegistry objstore.Clients, metricsSvc metrics.WorkerService) *S3CopySvc {
	return &S3CopySvc{
		credsSvc:       credsSvc,
		clientRegistry: clientRegistry,
		metricsSvc:     metricsSvc,
	}
}

func (r *S3CopySvc) GetVersionInfo(ctx context.Context, user string, to File) ([]entity.ObjectVersionInfo, error) {
	storageClient, err := r.clientRegistry.AsCommon(ctx, to.Storage, user)
	if err != nil {
		return nil, fmt.Errorf("unable to get %s storage client: %w", to.Storage, err)
	}
	objects := storageClient.ObjectVersions(ctx, to.Bucket, to.Name)

	result := []entity.ObjectVersionInfo{}
	for object, err := range objects {
		if err != nil {
			return nil, fmt.Errorf("unable to list versions: %w", err)
		}

		info := entity.ObjectVersionInfo{
			Version: object.VersionID,
			Size:    uint64(object.Size),
		}

		result = append(result, info)
	}

	return result, nil
}

func (r *S3CopySvc) GetLastMigratedVersionInfo(ctx context.Context, user string, to File) (entity.ObjectVersionInfo, error) {
	toClient, err := r.clientRegistry.AsS3(ctx, to.Storage, user)
	if err != nil {
		return entity.ObjectVersionInfo{}, fmt.Errorf("unable to get to client: %w", err)
	}
	objectStat, err := toClient.S3().StatObject(ctx, to.Bucket, to.Name, minio.StatObjectOptions{})
	if err != nil {
		var errorResp minio.ErrorResponse
		ok := errors.As(err, &errorResp)
		if !ok {
			return entity.ObjectVersionInfo{}, fmt.Errorf("unable to cast error to s3 error %w", err)
		}

		if errorResp.StatusCode != http.StatusNotFound {
			return entity.ObjectVersionInfo{}, fmt.Errorf("unexpected s3 error %w", err)
		}

		// Ceph has a bug, not returning delete marker header, check with list as last resort
		toStorage, err := r.credsSvc.GetS3Address(to.Storage)
		if err != nil {
			return entity.ObjectVersionInfo{}, fmt.Errorf("unable to get to storage address: %w", err)
		}
		if !objectStat.IsDeleteMarker && toStorage.Provider == s3.ProviderCeph {
			objectIter := toClient.S3().ListObjectsIter(ctx, to.Bucket, minio.ListObjectsOptions{
				Prefix:       to.Name,
				WithVersions: true,
				MaxKeys:      1,
			})

			objects := slices.Collect(objectIter)
			if len(objects) != 0 && objects[0].Err != nil {
				return entity.ObjectVersionInfo{}, fmt.Errorf("unable to list to object versions: %w", objects[0].Err)
			}
			// the prefix also matches objects with longer names listed after the object versions
			if len(objects) == 0 || objects[0].Key != to.Name {
				return entity.ObjectVersionInfo{}, dom.ErrNotFound
			}

			return entity.ObjectVersionInfo{
				DeleteMarker: true,
				// TODO address after Ceph fix
				// Version: objects[0].VersionID,
				// Size: uint64(objects[0].Size),
			}, nil
		}

		if !objectStat.IsDeleteMarker {
			return entity.ObjectVersionInfo{}, dom.ErrNotFound
		}

		return entity.ObjectVersionInfo{
			DeleteMarker: true,
			// TODO address after Ceph fix
			// Version: objectStat.VersionID,
		}, nil
	}

	sourceVersionID := objectStat.Metadata.Get(CTuttiSourceVersionIDMetaHeader)

	if sourceVersionID == "" {
		return entity.ObjectVersionInfo{}, dom.ErrInvalidArg
	}

	return entity.ObjectVersionInfo{
		Version: sourceVersionID,
		Size:    uint64(objectStat.Size),
	}, nil
}

// TODO this potentially can be a part of common client or its common application
// also, maybe split common client interface into client base (provider specific methods) and common client
// (implementing same logic with the use of base methods)
func (r *S3CopySvc) ClearDestination(ctx context.Context, user string, to File, versioned bool) error {
	toClient, err := r.clientRegistry.AsCommon(ctx, to.Storage, user)
	if err != nil {
		return fmt.Errorf("unable to get common to client: %w", err)
	}

	if !versioned {
		if err := toClient.RemoveObject(ctx, to.Bucket, to.Name); err != nil {
			return fmt.Errorf("unable to remove object: %w", err)
		}
		return nil
	}

	objectIter := toClient.ObjectVersions(ctx, to.Bucket, to.Name)

	listErrs := []error{}
	namesAndVersions := []objstore.NameAndVersion{}
	for object, err := range objectIter {
		if err != nil {
			listErrs = append(listErrs, fmt.Errorf("unable to list version: %w", err))
			continue
		}
		namesAndVersions = append(namesAndVersions, objstore.NameAndVersion{
			Name:    object.Key,
			Version: object.VersionID,
		})
	}

	removeErrs := toClient.RemoveVersionedObjects(ctx, to.Bucket, namesAndVersions)

	if len(listErrs) > 0 || len(removeErrs) > 0 {
		joinErr := errors.Join(errors.Join(listErrs...), errors.Join(removeErrs...))
		return fmt.Errorf("unable to delete object versions: %w", joinErr)
	}

	return nil
}

// EstimateCopyBuffer calculates the buffer budget needed for copying an object of the given size.
// For objects <= 5 MiB (non-multipart), the SDK uploads in a single PUT using the object size.
// For larger multipart objects, the SDK determines optimal part size via minio.OptimalPartInfo
// and allocates buffers according to that part size layout.
func EstimateCopyBuffer(size int64, customPartSize uint64) int64 {
	if size <= 0 {
		return 5 * 1024 * 1024
	}
	if size <= 5*1024*1024 {
		return size
	}
	_, partSize, _, err := minio.OptimalPartInfo(size, customPartSize)
	if err != nil || partSize <= 0 {
		return 64 * 1024 * 1024
	}
	return partSize
}

// EstimateParallelCopyBuffer is the buffer reservation for a multipart upload with the given number of
// parts in flight: threads * partSize, but never more than the object itself.
func EstimateParallelCopyBuffer(size int64, partSize uint64, threads int) int64 {
	if size <= 0 || partSize == 0 || threads < 1 {
		return EstimateCopyBuffer(size, partSize)
	}
	est := int64(partSize) * int64(threads)
	if size < est {
		// still at least one part
		est = size
	}
	return est
}

// CopyObject copies one object and records the throughput counters (in-flight, result, bytes, duration).
func (r *S3CopySvc) CopyObject(ctx context.Context, user string, from File, to File) error {
	start := time.Now()
	workerstats.Default.Start()
	metrics.CopyStarted()
	defer metrics.CopyFinished()
	var copiedBytes int64 = -1 // stays -1 when nothing was transferred (skipped as already present)
	err := r.copyObject(ctx, user, from, to, &copiedBytes)
	d := time.Since(start)
	switch {
	case err != nil:
		workerstats.Default.FinishFailure()
		metrics.ObserveCopy(false, 0, d)
	case copiedBytes >= 0:
		workerstats.Default.FinishSuccess(copiedBytes, d)
		metrics.ObserveCopy(true, copiedBytes, d)
	default:
		workerstats.Default.Skip()
	}
	return err
}

func (r *S3CopySvc) copyObject(ctx context.Context, user string, from File, to File, copiedBytes *int64) error {
	fromClient, err := r.clientRegistry.AsS3(ctx, from.Storage, user)
	if err != nil {
		return fmt.Errorf("unable to get from client: %w", err)
	}
	toClient, err := r.clientRegistry.AsS3(ctx, to.Storage, user)
	if err != nil {
		return fmt.Errorf("unable to get to client: %w", err)
	}

	// Request saving. In this minio-go GetObject issues a HEAD of its own and Object.Stat() is then free,
	// so for an unversioned source that has to be copied the separate source HEAD is a duplicate.
	// The source stat is only needed up front to decide whether an already present destination
	// makes the copy a no-op. Hence for an unversioned source the destination is checked first:
	//   - destination absent: nothing can be skipped, the source stat is taken from GetObject (HEAD + GET)
	//   - destination present: the source is stat-ed as before and compared (skip stays intact)
	// Versioned sources keep the original sequence (delete markers answer GET and HEAD differently).
	var fromObject *minio.Object
	var fromObjectStat, toObjectStat minio.ObjectInfo
	toObjectPresent := true
	toStatDone := false
	sourceStatDone := false
	if from.Version == "" {
		toObjectStat, err = toClient.S3().StatObject(ctx, to.Bucket, to.Name, minio.GetObjectOptions{
			VersionID: to.Version,
		})
		toStatDone = true
		if err != nil {
			if !isNotFound(err) {
				return fmt.Errorf("unable to get to object stat: %w", err)
			}
			toObjectPresent = false
			fromObject, err = fromClient.S3().GetObject(ctx, from.Bucket, from.Name, minio.GetObjectOptions{})
			if err != nil {
				return fmt.Errorf("unable to get from object: %w", err)
			}
			defer fromObject.Close()
			fromObjectStat, err = fromObject.Stat()
			if err != nil {
				if isNotFound(err) {
					return dom.ErrNotFound
				}
				return fmt.Errorf("unable to get from object stat: %w", err)
			}
			sourceStatDone = true
		}
	}

	if !sourceStatDone {
		fromObjectStat, err = fromClient.S3().StatObject(ctx, from.Bucket, from.Name, minio.GetObjectOptions{
			VersionID: from.Version,
		})
		if err != nil {
			switch {
			case from.Version != "" && fromObjectStat.IsDeleteMarker:
				// delete marker version is replicated below
			case from.Version != "" && isNotFound(err):
				// Ceph RGW answers a delete marker version with 404 and without the delete marker header:
				// tell it apart from a missing version by the object versions list.
				isDeleteMarker, listErr := r.isDeleteMarkerVersion(ctx, user, from)
				if listErr != nil {
					return fmt.Errorf("unable to list from object versions: %w", listErr)
				}
				if !isDeleteMarker {
					return dom.ErrNotFound
				}
				fromObjectStat.IsDeleteMarker = true
			case isNotFound(err):
				return dom.ErrNotFound
			case !fromObjectStat.IsDeleteMarker:
				return fmt.Errorf("unable to get from object stat: %w", err)
			}
		}
	}

	if !toStatDone {
		toObjectStat, err = toClient.S3().StatObject(ctx, to.Bucket, to.Name, minio.GetObjectOptions{
			VersionID: to.Version,
		})
		if err != nil {
			if !isNotFound(err) {
				return fmt.Errorf("unable to get to object stat: %w", err)
			}
			toObjectPresent = false
		}
	}

	if features.Tagging(ctx) {
		if len(fromObjectStat.UserTags) == 0 && fromObjectStat.UserTagCount > 0 {
			tags, err := fromClient.S3().GetObjectTagging(ctx, from.Bucket, from.Name, minio.GetObjectTaggingOptions{VersionID: from.Version})
			if err != nil {
				return fmt.Errorf("unable to get from object tagging: %w", err)
			}
			if tags != nil {
				fromObjectStat.UserTags = tags.ToMap()
			}
		}
		if toObjectPresent && len(toObjectStat.UserTags) == 0 && toObjectStat.UserTagCount > 0 {
			tags, err := toClient.S3().GetObjectTagging(ctx, to.Bucket, to.Name, minio.GetObjectTaggingOptions{VersionID: to.Version})
			if err != nil && !isNotFound(err) {
				return fmt.Errorf("unable to get to object tagging: %w", err)
			}
			if tags != nil {
				toObjectStat.UserTags = tags.ToMap()
			}
		}
	}

	if toObjectPresent &&
		fromObjectStat.ETag == toObjectStat.ETag &&
		fromObjectStat.Size == toObjectStat.Size &&
		sameMetadata(fromObjectStat, toObjectStat, features.Tagging(ctx)) {
		return nil
	}

	if fromObjectStat.IsDeleteMarker {
		if err := toClient.S3().RemoveObject(ctx, to.Bucket, to.Name, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("unable to replicate delete marker: %w", err)
		}
		return nil
	}

	fromObjectSize := fromObjectStat.Size

	ctx, span := otel.Tracer("").Start(ctx, "copy.CopyToWithVersion")
	span.SetAttributes(attribute.String("bucket", from.Bucket), attribute.String("object", from.Name),
		attribute.String("version", from.Version),
		attribute.String("from", from.Storage), attribute.String("to", to.Storage), attribute.Int64("size", fromObjectSize))
	defer span.End()

	toStorage, err := r.credsSvc.GetS3Address(to.Storage)
	if err != nil {
		return fmt.Errorf("unable to get to storage address: %w", err)
	}
	provider := toStorage.Provider

	putObjectOpts := minio.PutObjectOptions{
		UserMetadata:       fromObjectStat.UserMetadata,
		ContentType:        fromObjectStat.ContentType,
		ContentEncoding:    fromObjectStat.ContentEncoding,
		ContentDisposition: fromObjectStat.Metadata.Get("Content-Disposition"),
		ContentLanguage:    fromObjectStat.Metadata.Get("Content-Language"),
		CacheControl:       fromObjectStat.Metadata.Get("Cache-Control"),
		Expires:            fromObjectStat.Expires,
	}

	if features.Tagging(ctx) {
		putObjectOpts.UserTags = fromObjectStat.UserTags
	}

	if to.Version != "" {
		putObjectOpts.UserMetadata[CTuttiSourceVersionIDMetaHeader] = to.Version

		switch provider {
		case s3.ProviderCeph:
			putObjectOpts.Internal.CustomQueryParams = url.Values{
				CRGWVersionMigrationQueryParam: []string{to.Version},
			}
		case s3.ProviderMinIO:
			// minio will accept only uuid as a version id
			if _, err := uuid.Parse(to.Version); err == nil {
				putObjectOpts.Internal.SourceVersionID = to.Version
			}
		default:
		}
	}

	zerolog.Ctx(ctx).
		Debug().
		Str("file_size", util.ByteCountSI(fromObjectSize)).
		Msg("starting obj copy")

	r.metricsSvc.WorkerInProgressBytesInc(ctx, fromObjectSize)
	defer r.metricsSvc.WorkerInProgressBytesDec(ctx, fromObjectSize)
	if fromObject == nil {
		fromObject, err = fromClient.S3().GetObject(ctx, from.Bucket, from.Name, minio.GetObjectOptions{
			VersionID: from.Version,
		})
		if err != nil {
			return fmt.Errorf("unable to get from object: %w", err)
		}
		defer fromObject.Close()
	}
	// the source object can be deleted after the initial stat
	stat, err := fromObject.Stat()
	if err != nil {
		if isNotFound(err) {
			return dom.ErrNotFound
		}
		return fmt.Errorf("unable to stat from object: %w", err)
	}
	if stat.Size == 0 {
		// workaround for e2e test test/object_test.go TestApi_Object_Folder
		putObjectOpts.DisableContentSha256 = true
	}
	if !toStorage.IsSecure {
		// workaround for big objects in e2e tests TODO: investigate
		putObjectOpts.DisableContentSha256 = true
	}

	// Multipart tuning: objects above the part size are uploaded with several parts in flight.
	// Defaults (1 thread) keep the sequential single-stream upload.
	threads, partSize, partExplicit := systemsettings.EffectiveMultipart()
	parallel := threads > 1 && stat.Size > int64(partSize)
	if parallel {
		putObjectOpts.NumThreads = uint(threads)
		putObjectOpts.PartSize = partSize
	} else if partExplicit && stat.Size > int64(partSize) {
		putObjectOpts.PartSize = partSize
	}

	// Buffer budget guardrail:
	// Calculate actual buffer allocation needed by the SDK according to part layout (F2).
	// Small files (<= 5 MiB) allocate file size; multipart files allocate buffers based on minio.OptimalPartInfo.
	// With parallel parts, threads parts are in flight at once, so the reservation scales with threads.
	estimatedBuffer := EstimateCopyBuffer(stat.Size, putObjectOpts.PartSize)
	if parallel {
		estimatedBuffer = EstimateParallelCopyBuffer(stat.Size, putObjectOpts.PartSize, threads)
	}
	releaseBuffer, err := systemsettings.AcquireBufferBudget(ctx, estimatedBuffer)
	if err != nil {
		var oversize *systemsettings.BufferExceedsBudgetError
		if errors.As(err, &oversize) {
			return fmt.Errorf("unable to acquire buffer budget: %w: %w", err, asynq.SkipRetry)
		}
		return fmt.Errorf("unable to acquire buffer budget: %w", err)
	}
	defer releaseBuffer()

	var info minio.UploadInfo
	if parallel {
		// minio-go uploads parts concurrently only from an io.ReaderAt. minio.Object.ReadAt is serialized by
		// a mutex, so parallelSource serves each part with its own ranged GET instead. The whole-object GET
		// opened for the stat is not needed any more.
		fromObject.Close()
		psrc := newParallelSource(ctx, stat.Size, int64(partSize), systemsettings.BandwidthLimiter(),
			minioObjectOpener(func(c context.Context, o minio.GetObjectOptions) (*minio.Object, error) {
				return fromClient.S3().GetObject(c, from.Bucket, from.Name, o)
			}, from.Version, stat.ETag))
		defer psrc.Close()
		info, err = toClient.S3().PutObject(ctx, to.Bucket, to.Name, psrc, stat.Size, putObjectOpts)
		if err != nil {
			if isNotFound(psrc.Err()) {
				return dom.ErrNotFound
			}
			return fmt.Errorf("unable to upload object: %w", err)
		}
	} else {
		source := &sourceReader{obj: fromObject}
		throttledSource := systemsettings.NewThrottledReader(ctx, source, systemsettings.BandwidthLimiter())
		info, err = toClient.S3().PutObject(ctx, to.Bucket, to.Name, throttledSource, stat.Size, putObjectOpts)
		if err != nil {
			if isNotFound(source.err) {
				return dom.ErrNotFound
			}
			return fmt.Errorf("unable to upload object: %w", err)
		}
	}
	if info.VersionID != "" {
		to.Version = info.VersionID
	}
	*copiedBytes = stat.Size

	if features.ACL(ctx) {
		if err := r.CopyACLs(ctx, user, from, to); err != nil {
			return fmt.Errorf("unable to copy acl %w", err)
		}
	}

	return nil
}

// sourceReader keeps the source read error to tell it apart from a destination write error.
// It hides minio.Object.ReadAt so PutObject reads sequentially.
type sourceReader struct {
	obj *minio.Object
	err error
}

func (r *sourceReader) Read(p []byte) (int, error) {
	n, err := r.obj.Read(p)
	r.setErr(err)
	return n, err
}

// Seek allows PutObject to retry the upload.
func (r *sourceReader) Seek(offset int64, whence int) (int64, error) {
	n, err := r.obj.Seek(offset, whence)
	r.setErr(err)
	return n, err
}

func (r *sourceReader) setErr(err error) {
	if err != nil && !errors.Is(err, io.EOF) {
		r.err = err
	}
}

func (r *S3CopySvc) isDeleteMarkerVersion(ctx context.Context, user string, file File) (bool, error) {
	client, err := r.clientRegistry.AsCommon(ctx, file.Storage, user)
	if err != nil {
		return false, fmt.Errorf("unable to get %s storage client: %w", file.Storage, err)
	}
	// the listing is read to the end: stopping it early leaks the minio-go listing goroutine
	isDeleteMarker := false
	for object, err := range client.ObjectVersions(ctx, file.Bucket, file.Name) {
		if errors.Is(err, objstore.ErrBucketDoesntExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if object.VersionID == file.Version {
			isDeleteMarker = object.IsDeleteMarker
		}
	}
	return isDeleteMarker, nil
}

func isNotFound(err error) bool {
	var errorResp minio.ErrorResponse
	return errors.As(err, &errorResp) && (errorResp.Code == minio.NoSuchKey || errorResp.StatusCode == http.StatusNotFound)
}

func (r *S3CopySvc) CopyACLs(ctx context.Context, user string, from File, to File) error {
	fromClient, err := r.clientRegistry.AsS3(ctx, from.Storage, user)
	if err != nil {
		return fmt.Errorf("unable to get from client: %w", err)
	}
	toClient, err := r.clientRegistry.AsS3(ctx, to.Storage, user)
	if err != nil {
		return fmt.Errorf("unable to get to client: %w", err)
	}
	// check if copy ACL supported for versioned objects with the same version ID
	toStorage, err := r.credsSvc.GetS3Address(to.Storage)
	if err != nil {
		return fmt.Errorf("unable to get to storage address: %w", err)
	}
	toProvider := toStorage.Provider
	isVersioned := from.Version != ""
	sameVersion := from.Version == to.Version
	if isVersioned && sameVersion && !toProvider.SupportsRetainVersionID() {
		// TODO: iterate over from/to versions and find "to" version with the same index(or eTag?) as "from" version???
		zerolog.Ctx(ctx).Warn().Msgf("skip object ACL sync: same version IDs on different storages not supported for provider %q", toProvider)
		return nil
	}

	fromACL, err := fromClient.AWS().GetObjectAclWithContext(ctx, &awss3.GetObjectAclInput{
		Bucket:    &from.Bucket,
		Key:       &from.Name,
		VersionId: &from.Version,
	})
	if err != nil {
		if s3client.AwsErrRetry(err) {
			return fmt.Errorf("unable to get from acl: %w", err)
		}
		zerolog.Ctx(ctx).Err(err).Msg("skip object ACL sync due to get ACL err")
		return nil
	}

	toACL, err := toClient.AWS().GetObjectAclWithContext(ctx, &awss3.GetObjectAclInput{
		Bucket:    &to.Bucket,
		Key:       &to.Name,
		VersionId: &to.Version,
	})
	if err != nil {
		if s3client.AwsErrRetry(err) {
			return fmt.Errorf("unable to get to acl: %w", err)
		}
		zerolog.Ctx(ctx).Err(err).Msg("skip object ACL sync due to get ACL err")
		return nil
	}

	var toOwnerID *string
	if toACL != nil && toACL.Owner != nil {
		toOwnerID = toACL.Owner.ID
	}

	_, err = toClient.AWS().PutObjectAclWithContext(ctx, &awss3.PutObjectAclInput{
		AccessControlPolicy: MapOwnersACL(fromACL.Owner, fromACL.Grants, toOwnerID, features.PreserveACLGrants(ctx)),
		Bucket:              &to.Bucket,
		Key:                 &to.Name,
		VersionId:           &to.Version,
	})
	if err != nil {
		if s3client.AwsErrRetry(err) {
			return fmt.Errorf("unable to put to acl: %w", err)
		}
		zerolog.Ctx(ctx).Err(err).Msg("skip object ACL sync due to put ACL err")
		return nil
	}

	return nil
}

func MapOwnersACL(srcOwner *awss3.Owner, srcGrants []*awss3.Grant, dstOwner *string, preserveACLGrants bool) *awss3.AccessControlPolicy {
	destGrants := make([]*awss3.Grant, 0, len(srcGrants))
	for _, grant := range srcGrants {
		var destID *string
		if preserveACLGrants {
			destID = grant.Grantee.ID
		} else {
			destID = srcOwnerToDestOwner(grant.Grantee.ID, srcOwner.ID, dstOwner)
		}

		destGrant := &awss3.Grant{
			Grantee: &awss3.Grantee{
				ID:           destID,
				EmailAddress: grant.Grantee.EmailAddress,
				Type:         grant.Grantee.Type,
				URI:          grant.Grantee.URI,
				DisplayName:  grant.Grantee.DisplayName,
			},
			Permission: grant.Permission,
		}
		destGrants = append(destGrants, destGrant)
	}

	res := &awss3.AccessControlPolicy{
		Grants: destGrants,
	}
	if srcOwner != nil {
		res.Owner = &awss3.Owner{
			ID:          srcOwnerToDestOwner(srcOwner.ID, srcOwner.ID, dstOwner),
			DisplayName: srcOwner.DisplayName,
		}
	}

	return res
}

func srcOwnerToDestOwner(owner, srcBucketOwner, dstBucketOwner *string) *string {
	if owner == nil || *owner != *srcBucketOwner {
		return owner
	}
	return dstBucketOwner
}

func sameMetadata(from, to minio.ObjectInfo, checkTags bool) bool {
	if from.ContentType != to.ContentType {
		return false
	}
	if from.ContentEncoding != to.ContentEncoding {
		return false
	}
	if from.Metadata.Get("Content-Disposition") != to.Metadata.Get("Content-Disposition") {
		return false
	}
	if from.Metadata.Get("Content-Language") != to.Metadata.Get("Content-Language") {
		return false
	}
	if from.Metadata.Get("Cache-Control") != to.Metadata.Get("Cache-Control") {
		return false
	}
	if !from.Expires.Equal(to.Expires) {
		return false
	}
	if !maps.Equal(from.UserMetadata, to.UserMetadata) {
		return false
	}
	if checkTags && !maps.Equal(from.UserTags, to.UserTags) {
		return false
	}
	return true
}
