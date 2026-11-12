/*
 * Copyright © 2024 Clyso GmbH
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

package router

import (
	"net/http"
	"strconv"
	"strings"

	mclient "github.com/minio/minio-go/v7"
	"github.com/rs/zerolog"

	xctx "github.com/hieunyit/tutti/pkg/ctx"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/s3client"
	"github.com/hieunyit/tutti/pkg/tasks"
)

// uploadedSize returns the payload size of a PUT request when it is known from the request:
// x-amz-decoded-content-length for aws-chunked uploads, otherwise Content-Length.
func uploadedSize(req *http.Request) (int64, bool) {
	if v := req.Header.Get("X-Amz-Decoded-Content-Length"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			return n, true
		}
		return 0, false
	}
	if strings.Contains(strings.ToLower(req.Header.Get("Content-Encoding")), "aws-chunked") {
		return 0, false
	}
	if req.ContentLength >= 0 {
		return req.ContentLength, true
	}
	return 0, false
}

func (r *s3Router) putObject(req *http.Request) (resp *http.Response, taskList []tasks.ReplicationTask, storage string, isApiErr bool, err error) {
	ctx := req.Context()
	user, bucket, object := xctx.GetUser(ctx), xctx.GetBucket(ctx), xctx.GetObject(ctx)

	storage = xctx.GetRoutingPolicy(ctx)
	ctx = xctx.SetStorage(ctx, storage)
	req = req.WithContext(ctx)

	client, err := r.clients.AsS3(ctx, storage, user)
	if err != nil {
		return nil, nil, "", false, err
	}
	resp, isApiErr, err = r.forward(req, storage, client)
	if err != nil || isApiErr {
		return
	}
	zerolog.Ctx(ctx).Debug().Str("etag", resp.Header.Get("ETag")).Msg("uploaded object with etag")

	obj := dom.Object{Bucket: bucket,
		Name:    object,
		Version: "", // versionID not supported for obj PUT
	}
	objSize, known := uploadedSize(req)
	if !known {
		objInfo, statErr := client.S3().StatObject(ctx, bucket, object, mclient.StatObjectOptions{})
		if statErr != nil {
			zerolog.Ctx(ctx).Err(statErr).Msg("unable to get uploaded object size")
		} else {
			objSize = objInfo.Size
		}
	}

	taskList = append(taskList, &tasks.ObjectSyncPayload{
		Object:  obj,
		ObjSize: objSize,
	})
	if hasACLChanged(req) {
		taskList = append(taskList, &tasks.ObjSyncACLPayload{
			Object: obj,
		})
	}
	if hasTagsChanged(req) {
		taskList = append(taskList, &tasks.ObjSyncTagsPayload{
			Object: obj,
		})
	}

	return
}

func (r *s3Router) deleteObjects(req *http.Request) (resp *http.Response, taskList []tasks.ReplicationTask, storage string, isApiErr bool, err error) {
	ctx := req.Context()
	user := xctx.GetUser(ctx)
	bucket := xctx.GetBucket(ctx)

	storage = xctx.GetRoutingPolicy(ctx)
	ctx = xctx.SetStorage(ctx, storage)
	req = req.WithContext(ctx)

	client, err := r.clients.AsS3(ctx, storage, user)
	if err != nil {
		return nil, nil, "", false, err
	}
	reqBody := deleteObjectsRequest{}
	err = s3client.ExtractReqBody(req, &reqBody)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("unable to unmarshal deleteObjectsRequest request body")
		return
	}

	resp, isApiErr, err = r.forward(req, storage, client)
	if err != nil || isApiErr {
		return
	}

	respBody := multiDeleteResult{}
	err = s3client.ExtractRespBody(resp, &respBody)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("unable to unmarshal multiDeleteResult response body")
		return
	}

	if reqBody.Quiet {
		// quiet mode: filter request objects by response errors
		errSet := make(map[objectID]struct{}, len(respBody.Error))
		for _, delErr := range respBody.Error {
			errSet[objectID{Key: delErr.Key, VersionID: delErr.VersionID}] = struct{}{}
		}
		for _, object := range reqBody.Objects {
			if _, ok := errSet[object]; ok {
				continue
			}
			taskList = append(taskList, &tasks.ObjectSyncPayload{
				Object:  object.toDom(bucket),
				Deleted: true,
			})
		}
	} else {
		// normal mode: use deleted obj list from response
		taskList = make([]tasks.ReplicationTask, len(respBody.Deleted))
		for i, object := range respBody.Deleted {
			taskList[i] = &tasks.ObjectSyncPayload{
				Object:  object.toDom(bucket),
				Deleted: true,
			}
		}
	}
	return
}
