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

// Package resilience contains end-to-end tests of the proxy resilience features: aws-chunked payload decoding,
// replication event recording failures, synchronous replication, circuit breakers and automatic failover.
// The storages are real MinIO containers; unreachable storages are simulated with an in-test TCP forwarder.
package resilience

import (
	"context"
	"fmt"
	"os"
	"testing"

	mclient "github.com/minio/minio-go/v7"
	mcredentials "github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/service/worker"
	"github.com/hieunyit/tutti/test/app"
	"github.com/hieunyit/tutti/test/env"
)

const (
	mainKey = "main"
	f1Key   = "f1"
	f2Key   = "f2"
	user    = "testuser"
)

var (
	workerConf *worker.Config
	proxyConf  *proxy.Config

	// storages holds the MinIO storages by their real (direct) addresses.
	storages map[string]s3.Storage
	// direct clients bypass the proxy.
	direct map[string]*mclient.Client

	mainCreds s3.CredentialsV4
)

func TestMain(m *testing.M) {
	os.Exit(setup(m))
}

func setup(m *testing.M) int {
	ctx := context.Background()

	testEnv, err := env.NewTestEnvironment(ctx, map[string]env.ComponentCreationConfig{
		mainKey: env.AsMinio(env.WithDisabledSTDOutLog(), env.WithDisabledSTDErrLog()),
		f1Key:   env.AsMinio(env.WithDisabledSTDOutLog(), env.WithDisabledSTDErrLog()),
		f2Key:   env.AsMinio(env.WithDisabledSTDOutLog(), env.WithDisabledSTDErrLog()),
	})
	if err != nil {
		panic(fmt.Sprintf("unable to create test environment: %v", err))
	}
	defer testEnv.Terminate(ctx)

	storages = map[string]s3.Storage{}
	direct = map[string]*mclient.Client{}
	for _, key := range []string{mainKey, f1Key, f2Key} {
		access, err := testEnv.GetMinioAccessConfig(key)
		if err != nil {
			panic(fmt.Sprintf("unable to get %s minio config: %v", key, err))
		}
		endpoint := fmt.Sprintf("%s:%d", access.Host.Local, access.S3Port.Forwarded)
		creds := s3.CredentialsV4{AccessKeyID: access.User, SecretAccessKey: access.Password}
		if key == mainKey {
			mainCreds = creds
		}
		storages[key] = s3.Storage{
			StorageAddress:    s3.StorageAddress{Address: "http://" + endpoint, Provider: s3.ProviderMinIO},
			Credentials:       map[string]s3.CredentialsV4{user: creds},
			ConnectionAccount: &creds,
		}
		direct[key], err = mclient.New(endpoint, &mclient.Options{
			Creds: mcredentials.NewStaticV4(creds.AccessKeyID, creds.SecretAccessKey, ""),
		})
		if err != nil {
			panic(fmt.Sprintf("unable to create %s client: %v", key, err))
		}
	}

	workerConf, err = worker.GetConfig()
	if err != nil {
		panic(fmt.Sprintf("unable to get worker config: %v", err))
	}
	workerConf.Log.Level = "warn"
	workerConf.Api.HttpPort = 0
	workerConf.Storage = app.WorkerS3Config(mainKey, storages)

	proxyConf, err = proxy.GetConfig()
	if err != nil {
		panic(fmt.Sprintf("unable to get proxy config: %v", err))
	}
	proxyConf.Log.Level = "warn"
	proxyConf.Storage = app.ProxyS3Config(mainKey, storages)
	proxyConf.Auth.UseStorage = mainKey

	return m.Run()
}
