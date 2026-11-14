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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/proxyaccount"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

type testAesSealer struct{}

func (testAesSealer) Seal(p []byte) (string, error) {
	b, _ := aes.NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	g, _ := cipher.NewGCM(b)
	nonce := make([]byte, g.NonceSize())
	_, _ = rand.Read(nonce)
	return base64.StdEncoding.EncodeToString(g.Seal(nonce, nonce, p, nil)), nil
}

func (testAesSealer) Open(v string) ([]byte, error) { return nil, errors.New("unused") }

func accountsClient(t *testing.T, opts ...ResilienceOption) (pb.ResilienceClient, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rc.Close() })
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer(grpc.UnaryInterceptor(ErrorInterceptor()))
	pb.RegisterResilienceServer(srv, ResilienceHandlers(rc, func(string) bool { return true }, opts...))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewResilienceClient(conn), mr
}

func TestProxyAccounts_Lifecycle(t *testing.T) {
	c, mr := accountsClient(t, WithProxyAccounts(
		func() (proxyaccount.Sealer, error) { return testAesSealer{}, nil },
		func(user string) bool { return user == "alice" }))
	ctx := t.Context()

	// unknown storage user
	_, err := c.CreateProxyAccount(ctx, &pb.CreateProxyAccountRequest{User: "nobody"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = c.CreateProxyAccount(ctx, &pb.CreateProxyAccountRequest{})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	created, err := c.CreateProxyAccount(ctx, &pb.CreateProxyAccountRequest{User: "alice", Description: "ci"})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(created.AccessKeyId, "TUT"))
	require.Len(t, created.SecretAccessKey, 40)
	require.NotContains(t, mr.Dump(), created.SecretAccessKey, "secret must be sealed in redis")

	// list never contains secrets
	list, err := c.ListProxyAccounts(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.Len(t, list.Accounts, 1)
	acc := list.Accounts[0]
	require.Equal(t, "alice", acc.User)
	require.Equal(t, []string{created.AccessKeyId}, acc.AccessKeyIds)
	require.Equal(t, []string{"dynamic"}, acc.Sources)
	require.Len(t, acc.DynamicKeys, 1)
	k := acc.DynamicKeys[0]
	require.Equal(t, "dynamic", k.Source)
	require.Equal(t, "active", k.Status)
	require.Equal(t, "ci", k.Description)
	require.NotNil(t, k.CreatedAt)
	require.Nil(t, k.RotatedAt)
	js := protojson.Format(list)
	require.NotContains(t, js, created.SecretAccessKey)
	require.NotContains(t, strings.ToLower(js), "secret")

	// rotate: default grace 1h, secret returned once
	rot, err := c.RotateProxyAccountSecret(ctx, &pb.RotateProxyAccountSecretRequest{AccessKeyId: created.AccessKeyId})
	require.NoError(t, err)
	require.NotEqual(t, created.SecretAccessKey, rot.SecretAccessKey)
	require.NotNil(t, rot.PreviousSecretExpiresAt)
	require.WithinDuration(t, time.Now().Add(time.Hour), rot.PreviousSecretExpiresAt.AsTime(), time.Minute)
	list, _ = c.ListProxyAccounts(ctx, &emptypb.Empty{})
	k = list.Accounts[0].DynamicKeys[0]
	require.NotNil(t, k.RotatedAt)
	require.NotNil(t, k.PreviousSecretExpiresAt)
	js = protojson.Format(list)
	require.NotContains(t, js, created.SecretAccessKey)
	require.NotContains(t, js, rot.SecretAccessKey)

	// immediate rotation
	rot, err = c.RotateProxyAccountSecret(ctx, &pb.RotateProxyAccountSecretRequest{AccessKeyId: created.AccessKeyId, GracePeriod: durationpb.New(0)})
	require.NoError(t, err)
	require.Nil(t, rot.PreviousSecretExpiresAt)
	list, _ = c.ListProxyAccounts(ctx, &emptypb.Empty{})
	require.Nil(t, list.Accounts[0].DynamicKeys[0].PreviousSecretExpiresAt)
	_, err = c.RotateProxyAccountSecret(ctx, &pb.RotateProxyAccountSecretRequest{AccessKeyId: "TUTNOPE"})
	require.Equal(t, codes.NotFound, status.Code(err))

	// status
	_, err = c.SetProxyAccountStatus(ctx, &pb.SetProxyAccountStatusRequest{AccessKeyId: created.AccessKeyId, Status: "disabled"})
	require.NoError(t, err)
	list, _ = c.ListProxyAccounts(ctx, &emptypb.Empty{})
	require.Equal(t, "disabled", list.Accounts[0].DynamicKeys[0].Status)
	_, err = c.SetProxyAccountStatus(ctx, &pb.SetProxyAccountStatusRequest{AccessKeyId: created.AccessKeyId, Status: "bogus"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = c.SetProxyAccountStatus(ctx, &pb.SetProxyAccountStatusRequest{AccessKeyId: "TUTNOPE", Status: "active"})
	require.Equal(t, codes.NotFound, status.Code(err))

	// delete
	_, err = c.DeleteProxyAccount(ctx, &pb.ProxyAccountKey{AccessKeyId: created.AccessKeyId})
	require.NoError(t, err)
	_, err = c.DeleteProxyAccount(ctx, &pb.ProxyAccountKey{AccessKeyId: created.AccessKeyId})
	require.Equal(t, codes.NotFound, status.Code(err))
	list, _ = c.ListProxyAccounts(ctx, &emptypb.Empty{})
	require.Empty(t, list.Accounts)
}

func TestProxyAccounts_NoMasterPassword(t *testing.T) {
	c, mr := accountsClient(t, WithProxyAccounts(
		func() (proxyaccount.Sealer, error) { return nil, dom.ErrFailedPrecondition },
		func(string) bool { return true }))
	_, err := c.CreateProxyAccount(t.Context(), &pb.CreateProxyAccountRequest{User: "alice"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.False(t, mr.Exists(proxyaccount.Key))

	// without the option accounts cannot be created either
	c2, _ := accountsClient(t)
	_, err = c2.CreateProxyAccount(t.Context(), &pb.CreateProxyAccountRequest{User: "alice"})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestListProxyAccounts_DynamicAccountsStatus(t *testing.T) {
	ctx := t.Context()
	c, _ := accountsClient(t) // not configured
	l, err := c.ListProxyAccounts(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.False(t, l.DynamicAccounts.Available)
	require.NotEmpty(t, l.DynamicAccounts.Reason)

	c, _ = accountsClient(t, WithProxyAccounts(func() (proxyaccount.Sealer, error) {
		return nil, errors.New("master password missing")
	}, nil))
	l, err = c.ListProxyAccounts(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.False(t, l.DynamicAccounts.Available)
	require.Contains(t, l.DynamicAccounts.Reason, "master password missing")

	c, _ = accountsClient(t, WithProxyAccounts(func() (proxyaccount.Sealer, error) { return testAesSealer{}, nil }, nil))
	l, err = c.ListProxyAccounts(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.True(t, l.DynamicAccounts.Available)
	require.Empty(t, l.DynamicAccounts.Reason)
}
