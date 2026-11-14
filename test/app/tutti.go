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

package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/testutil"
	"github.com/hieunyit/tutti/pkg/util"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
	"github.com/hieunyit/tutti/service/proxy"
	"github.com/hieunyit/tutti/service/worker"
	"github.com/hieunyit/tutti/service/worker/handler"
	"github.com/rs/xid"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

var (
	asynqRetry = 100 * time.Millisecond
)

var WorkerRetryConf = handler.Config{
	SwiftRetryInterval:       asynqRetry * 5,
	PauseRetryInterval:       asynqRetry * 2,
	SwitchRetryInterval:      asynqRetry * 2,
	QueueUpdateInterval:      asynqRetry * 2,
	TaskCheckInterval:        asynqRetry,
	DelayedTaskCheckInterval: asynqRetry * 2,
	CustomErrRetryInterval:   &asynqRetry,
}

type Tutti struct {
	PolicyClient  pb.PolicyClient
	TuttiClient  pb.TuttiClient
	WebhookClient pb.WebhookClient

	UrlHttpApi string
	ProxyAddr  string
	ProxyPort  int

	WaitShort  time.Duration
	RetryShort time.Duration
	WaitLong   time.Duration
	RetryLong  time.Duration
}

func SetupTutti(t testing.TB, workerConf *worker.Config, proxyConf *proxy.Config) Tutti {
	t.Helper()
	return SetupTuttiRedis(t, workerConf, proxyConf, testutil.SetupRedisAddr(t))
}

// SetupTuttiRedis is SetupTutti on a caller-provided Redis, so a test can control it (e.g. inject errors into miniredis).
func SetupTuttiRedis(t testing.TB, workerConf *worker.Config, proxyConf *proxy.Config, redisAddr string) Tutti {
	t.Helper()
	var err error
	workerConf, err = DeepCopyStruct(workerConf)
	if err != nil {
		t.Fatal(err)
	}
	workerConf.Worker = &WorkerRetryConf
	e := Tutti{
		WaitShort:  waitShort,
		RetryShort: retryShort,
		WaitLong:   waitLong,
		RetryLong:  retryLong,
	}

	workerConf.Redis.Address = redisAddr

	if proxyConf != nil {
		proxyConf, err = DeepCopyStruct(proxyConf)
		if err != nil {
			t.Fatal(err)
		}
		proxyConf.Redis.Address = redisAddr
		proxyConf.Port, e.ProxyAddr = getRandomPort()
		e.ProxyPort = proxyConf.Port
		proxyConf.Address = e.ProxyAddr
		if err := proxyConf.Validate(); err != nil {
			t.Error("invalid proxy config", err)
		}
	}

	workerConf.Api.Enabled = true
	workerConf.Api.Webhook.Enabled = true
	grpcAddr := ""
	workerConf.Api.GrpcPort, grpcAddr = getRandomPort()
	// HttpPort may be pre-set when the caller needs to know the port before
	// starting the worker (e.g. to build webhook baseURL for container→host push).
	if workerConf.Api.HttpPort == 0 {
		workerConf.Api.HttpPort, e.UrlHttpApi = getRandomPort()
	} else {
		e.UrlHttpApi = fmt.Sprintf("127.0.0.1:%d", workerConf.Api.HttpPort)
	}
	e.UrlHttpApi = "http://" + e.UrlHttpApi

	webhookGrpcAddr := grpcAddr
	if workerConf.Api.Webhook.GrpcPort > 0 {
		workerConf.Api.Webhook.GrpcPort, webhookGrpcAddr = getRandomPort()
		workerConf.Api.Webhook.HttpPort, _ = getRandomPort()
	}

	ctx := t.Context()
	if err := workerConf.Validate(); err != nil {
		t.Error("invalid worker config", err)
	}

	wg, ctx := errgroup.WithContext(ctx)
	if proxyConf != nil {
		wg.Go(func() error {
			app := dom.AppInfo{
				Version: "test",
				App:     "proxy",
				AppID:   xid.New().String(),
			}
			if err := proxy.Start(ctx, app, proxyConf); err != nil {
				return fmt.Errorf("proxy error: %w", err)
			}
			return nil
		})
	}
	wg.Go(func() error {
		app := dom.AppInfo{
			Version: "test",
			App:     "worker",
			AppID:   xid.New().String(),
		}
		if err := worker.Start(ctx, app, workerConf); err != nil {
			return fmt.Errorf("worker error: %w", err)
		}
		return nil
	})
	t.Cleanup(func() {
		err := wg.Wait()
		if util.IsServerError(err) {
			t.Error("embedded env services exited with error:", err)
		}
	})

	grpcConn, err := grpc.DialContext(ctx, grpcAddr,
		grpc.WithInsecure(),
		grpc.WithBackoffMaxDelay(time.Second),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatal("failed to dial grpc api:", err)
	}
	e.PolicyClient = pb.NewPolicyClient(grpcConn)
	e.TuttiClient = pb.NewTuttiClient(grpcConn)

	if webhookGrpcAddr != grpcAddr {
		webhookConn, err := grpc.DialContext(ctx, webhookGrpcAddr,
			grpc.WithInsecure(),
			grpc.WithBackoffMaxDelay(time.Second),
			grpc.WithBlock(),
		)
		if err != nil {
			t.Fatal("failed to dial webhook grpc api:", err)
		}
		e.WebhookClient = pb.NewWebhookClient(webhookConn)
	} else {
		e.WebhookClient = pb.NewWebhookClient(grpcConn)
	}
	return e
}
