package accesspolicy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hieunyit/tutti/pkg/s3"
	"github.com/redis/go-redis/v9"
)

type reviewPolicyRedis struct {
	redis.Cmdable
	started chan struct{}
	release chan struct{}
	err     error
}

func (r *reviewPolicyRedis) HGetAll(ctx context.Context, key string) *redis.MapStringStringCmd {
	if r.started != nil {
		close(r.started)
		<-r.release
	}
	cmd := redis.NewMapStringStringCmd(ctx)
	cmd.SetVal(map[string]string{"restricted": `{"permission":"read-only","buckets":["allowed"]}`})
	cmd.SetErr(r.err)
	return cmd
}

func TestReviewColdPolicyCacheMustNotReturnUnrestrictedDuringLoad(t *testing.T) {
	r := &reviewPolicyRedis{started: make(chan struct{}), release: make(chan struct{})}
	c := NewCache(NewStore(r), time.Second)
	done := make(chan struct{})
	go func() { defer close(done); c.Lookup(context.Background(), "restricted") }()
	<-r.started
	type lookupResult struct {
		policy Policy
		found  bool
	}
	result := make(chan lookupResult, 1)
	go func() { p, found := c.Lookup(context.Background(), "restricted"); result <- lookupResult{p, found} }()
	select {
	case got := <-result:
		if !got.found {
			t.Error("cold cache returned no policy while initial load was in progress; access middleware grants full access")
		}
		if err := got.policy.Authorize(Request{Method: s3.PutObject, Bucket: "forbidden"}); err == nil {
			t.Error("request during initial load must not bypass restrictions")
		}
	case <-time.After(50 * time.Millisecond):
		// Blocking until the initial load completes is also safe.
	}
	close(r.release)
	<-done
}

func TestReviewColdPolicyCacheMustNotReturnUnrestrictedOnRedisError(t *testing.T) {
	c := NewCache(NewStore(&reviewPolicyRedis{err: errors.New("redis unavailable")}), time.Second)
	p, found := c.Lookup(context.Background(), "restricted")
	if !found {
		t.Fatal("cold cache returned no policy after Redis failure; unknown authorization state is treated as unrestricted")
	}
	if err := p.Authorize(Request{Method: s3.GetObject, Bucket: "allowed"}); err == nil {
		t.Fatal("policy returned after Redis failure must deny access")
	}
}

func TestReviewPolicyCacheRecoversAfterInitialFailure(t *testing.T) {
	r := &reviewPolicyRedis{err: errors.New("redis unavailable")}
	c := NewCache(NewStore(r), time.Second)
	now := time.Now()
	c.now = func() time.Time { return now }
	c.Lookup(context.Background(), "restricted")
	r.err = nil
	now = now.Add(2 * time.Second)
	p, found := c.Lookup(context.Background(), "restricted")
	if !found || p.Permission != ReadOnly {
		t.Fatal("successful reload must restore the stored restriction")
	}
	if _, found := c.Lookup(context.Background(), "unrestricted"); found {
		t.Fatal("account known to have no policy must retain legacy access")
	}
}
