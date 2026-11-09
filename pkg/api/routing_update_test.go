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

package api

import (
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/policy"
	"github.com/hieunyit/tutti/pkg/store"
	"github.com/hieunyit/tutti/pkg/tasks"
	pb "github.com/hieunyit/tutti/proto/gen/go/chorus"
)

type userCreds struct {
	objstore.CredsService
	// storage -> users
	users map[string][]string
}

func (c userCreds) HasUser(storage, user string) error {
	for _, u := range c.users[storage] {
		if u == user {
			return nil
		}
	}
	return dom.ErrNotFound
}

func TestPolicy_UpdateRouting(t *testing.T) {
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	policySvc := policy.NewService(rc, &tasks.QueueServiceMock{Queues: map[string]int{}, Paused: map[string]bool{}}, "main")
	creds := userCreds{users: map[string][]string{"main": {"u"}, "a": {"u"}, "b": {"u"}}}
	h := PolicyHandlers(creds, nil, nil, policySvc, nil, nil, nil, nil, nil, store.NewUserLocker(rc, time.Second), nil)

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.UnaryInterceptor(ErrorInterceptor()))
	pb.RegisterPolicyServer(srv, h)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	c := pb.NewPolicyClient(conn)
	ctx := t.Context()
	bucket := "bk"

	// not existing routing -> NotFound
	_, err = c.UpdateRouting(ctx, &pb.UpdateRoutingRequest{User: "u", ToStorage: "b"})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = c.UpdateRouting(ctx, &pb.UpdateRoutingRequest{User: "u", Bucket: &bucket, ToStorage: "b"})
	require.Equal(t, codes.NotFound, status.Code(err))

	_, err = c.AddRouting(ctx, &pb.AddRoutingRequest{User: "u", ToStorage: "a"})
	require.NoError(t, err)
	_, err = c.AddRouting(ctx, &pb.AddRoutingRequest{User: "u", Bucket: &bucket, ToStorage: "a"})
	require.NoError(t, err)

	_, err = c.UpdateRouting(ctx, &pb.UpdateRoutingRequest{User: "u", ToStorage: "b"})
	require.NoError(t, err)
	_, err = c.UpdateRouting(ctx, &pb.UpdateRoutingRequest{User: "u", Bucket: &bucket, ToStorage: "main"})
	require.NoError(t, err)
	list, err := c.ListRoutings(ctx, &pb.RoutingsRequest{})
	require.NoError(t, err)
	require.Len(t, list.UserRoutings, 1)
	require.Equal(t, "b", list.UserRoutings[0].ToStorage)
	require.Len(t, list.BucketRoutings, 1)
	require.Equal(t, "main", list.BucketRoutings[0].ToStorage)

	// validation: unknown storage / user without creds there, missing args
	_, err = c.UpdateRouting(ctx, &pb.UpdateRoutingRequest{User: "u", ToStorage: "nope"})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = c.UpdateRouting(ctx, &pb.UpdateRoutingRequest{User: "u"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = c.UpdateRouting(ctx, &pb.UpdateRoutingRequest{ToStorage: "a"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	list, _ = c.ListRoutings(ctx, &pb.RoutingsRequest{})
	require.Equal(t, "b", list.UserRoutings[0].ToStorage)
}

func (c userCreds) MainStorage() string { return "main" }

func (c userCreds) ListUsers(storage string) []string { return c.users[storage] }
