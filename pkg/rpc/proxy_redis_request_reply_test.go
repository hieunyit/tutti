package rpc

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/testutil"
	pb "github.com/hieunyit/tutti/proto/gen/go/tutti"
)

var _ Proxy = &mockProxy{}

type mockProxy struct{}

func (m mockProxy) GetCredentials(_ context.Context) (*pb.GetProxyCredentialsResponse, error) {
	return &pb.GetProxyCredentialsResponse{Address: "address", Credentials: []*pb.Credential{{
		Alias:     "admin",
		AccessKey: "access",
		SecretKey: "secret",
	}}}, nil
}

// Hold only the SUBSCRIBE acknowledgement; the normal Redis handshake can finish.
type heldSubscribeConn struct {
	net.Conn
	waiting atomic.Bool
	written chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func (c *heldSubscribeConn) Write(p []byte) (int, error) {
	n, err := c.Conn.Write(p)
	if strings.Contains(strings.ToLower(string(p)), "subscribe") && c.waiting.CompareAndSwap(false, true) {
		close(c.written)
	}
	return n, err
}

func (c *heldSubscribeConn) Read(p []byte) (int, error) {
	if c.waiting.Load() {
		<-c.closed
		return 0, net.ErrClosed
	}
	return c.Conn.Read(p)
}

func (c *heldSubscribeConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func TestProxyServeCancelsWhileSubscriptionAckIsMissing(t *testing.T) {
	mr := miniredis.RunT(t)
	written := make(chan struct{})
	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return &heldSubscribeConn{Conn: conn, written: written, closed: make(chan struct{})}, nil
	}})
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- ProxyServe(ctx, client, mockProxy{}) }()
	select {
	case <-written:
	case <-time.After(2 * time.Second):
		t.Fatal("subscription was not sent")
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("server did not stop while waiting for subscription acknowledgement")
	}
}

func TestProxyClient_GetCredentials(t *testing.T) {
	c := testutil.SetupRedis(t)
	r := require.New(t)
	ctx := t.Context()
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = ProxyServe(ctx, c, &mockProxy{})
	}()
	t.Cleanup(func() {
		wg.Wait()
	})
	time.Sleep(50 * time.Millisecond)
	tst := ProxyClient{c}
	res, err := tst.GetCredentials(ctx)
	r.NoError(err)
	r.EqualValues("address", res.Address)
	r.Len(res.Credentials, 1)
	r.EqualValues("admin", res.Credentials[0].Alias)
	r.EqualValues("access", res.Credentials[0].AccessKey)
	r.EqualValues("secret", res.Credentials[0].SecretKey)
}

func TestProxyClient_ConcurrentRepliesAreNotLost(t *testing.T) {
	c := testutil.SetupRedis(t)
	ctx, cancel := context.WithCancel(t.Context())
	serverDone := make(chan error, 1)
	go func() { serverDone <- ProxyServe(ctx, c, mockProxy{}) }()
	t.Cleanup(func() { cancel(); <-serverDone })
	require.Eventually(t, func() bool {
		n, err := c.PubSubNumSub(ctx, proxyCreds).Result()
		return err == nil && n[proxyCreds] == 1
	}, time.Second, time.Millisecond)
	const requests = 32
	errs := make(chan error, requests)
	for range requests {
		go func() {
			res, err := NewProxyClient(c).GetCredentials(ctx)
			if err == nil && (res.Address != "address" || len(res.Credentials) != 1) {
				err = fmt.Errorf("unexpected credentials response: %v", res)
			}
			errs <- err
		}()
	}
	for range requests {
		require.NoError(t, <-errs)
	}
}
