/*
 * Copyright © 2023 Clyso GmbH
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

package standalone

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hieunyit/tutti/pkg/dom"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

func getFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

func TestStandaloneDefaultConfig(t *testing.T) {
	r := require.New(t)
	conf, err := GetConfig()
	r.NoError(err)
	r.EqualValues(5*time.Second, conf.ShutdownTimeout)
	conf.ShutdownTimeout = 500 * time.Millisecond
	conf.Proxy.Port = getFreePort(t)
	conf.Api.GrpcPort = getFreePort(t)
	conf.Api.HttpPort = getFreePort(t)
	conf.UIPort = getFreePort(t)
	for k, s := range conf.Storage.Storages {
		if s.S3 != nil {
			s.S3.Address = fmt.Sprintf(":%d", getFreePort(t))
			conf.Storage.Storages[k] = s
		}
	}
	for k, s := range conf.Proxy.Storage.Storages {
		if s.S3 != nil {
			s.S3.Address = conf.Storage.Storages[k].S3.Address
			conf.Proxy.Storage.Storages[k] = s
		}
	}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	var done atomic.Bool
	go func() {
		_ = Start(ctx, dom.AppInfo{}, conf)
		done.Store(true)
	}()

	grpcConn, err := grpc.DialContext(ctx, fmt.Sprintf("localhost:%d", conf.Api.GrpcPort),
		grpc.WithInsecure(),
		grpc.WithBackoffMaxDelay(time.Second),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	r.NoError(err)
	apiClient := pb.NewChorusClient(grpcConn)

	r.Eventually(func() bool {
		res, err := apiClient.GetStorages(ctx, &emptypb.Empty{})
		if err != nil {
			return false
		}
		return len(res.Storages) == 2
	}, 2*time.Second, 10*time.Millisecond, "standalone is not up")

	cancel()
	r.Eventually(func() bool {
		return done.Load()
	}, 2*time.Second, 10*time.Millisecond, "unable to shutdown standalone")
}
