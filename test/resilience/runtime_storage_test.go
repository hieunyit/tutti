// Copyright 2026 hieunyit
// SPDX-License-Identifier: Apache-2.0

package resilience

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/service/worker"
	"github.com/stretchr/testify/require"
)

func TestRuntimeStorageWithoutRestart(t *testing.T) {
	e := startEnv(t, envOpts{dynamicCreds: true,
		proxyHook:  func(c *proxy.Config) { delete(c.Storage.Storages, f2Key) },
		workerHook: func(c *worker.Config) { delete(c.Storage.Storages, f2Key) },
	})
	ctx := t.Context()
	var originalID any
	require.Eventually(t, func() bool {
		initial := e.proxyStatus(t)
		if initial != nil {
			originalID = initial["appId"]
		}
		return originalID != nil
	}, 5*time.Second, 100*time.Millisecond, "proxy must publish its initial process identity")
	config := map[string]any{"name": "archive", "address": storages[f2Key].Address, "provider": "Minio", "isSecure": false, "defaultRegion": "us-east-1", "httpTimeout": "5s", "healthCheckInterval": "1s"}
	credentials := []map[string]any{{"user": user, "accessKeyId": mainCreds.AccessKeyID, "secretAccessKey": mainCreds.SecretAccessKey}}
	test := e.restJSON(t, http.MethodPost, "/tutti/storages/test", map[string]any{"storage": config, "credential": credentials[0]})
	require.Equal(t, true, test["ok"])
	e.restJSON(t, http.MethodPost, "/tutti/storages", map[string]any{"storage": config, "credentials": credentials})
	listed := e.restJSON(t, http.MethodGet, "/tutti/storages", nil)
	raw, _ := json.Marshal(listed)
	require.NotContains(t, string(raw), "secretAccessKey")
	require.NotContains(t, string(raw), "secret_access_key")
	require.Eventually(t, func() bool {
		status := e.proxyStatus(t)
		if originalID != nil && status != nil {
			require.Equal(t, originalID, status["appId"], "proxy must not restart")
		}
		if status == nil {
			return false
		}
		list, _ := status["storages"].([]any)
		for _, item := range list {
			if entry, ok := item.(map[string]any); ok && entry["name"] == "archive" {
				return true
			}
		}
		return false
	}, 5*time.Second, 100*time.Millisecond, "new storage must reach proxy within 5s")
	bucket := bucketName("runtime-storage")
	e.replicateBucket(t, bucket, "archive")
	code, _, body := e.presignedPut(t, bucket, "object", "runtime storage payload")
	require.Equal(t, http.StatusOK, code, body)
	require.Eventually(t, func() bool {
		value, err := readObject(t, direct[f2Key], bucket, "object")
		return err == nil && value == "runtime storage payload"
	}, e.WaitLong, e.RetryLong)
	deletionBody, _ := json.Marshal(map[string]string{"name": "archive"})
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, e.UrlHttpApi+"/tutti/storages/delete", bytes.NewReader(deletionBody))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, response.StatusCode, string(data))
	require.Contains(t, string(data), "referenc")
	_, err = e.PolicyClient.DeleteReplication(ctx, replID(bucket, "archive"))
	require.NoError(t, err)
	e.restJSON(t, http.MethodPut, "/tutti/storages/delete", map[string]string{"name": "archive"})
}
