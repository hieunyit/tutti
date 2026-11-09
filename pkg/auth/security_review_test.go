package auth

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestReviewSessionMustHonorRestartedUserConfiguration(t *testing.T) {
	for _, change := range []string{"deleted", "demoted"} {
		t.Run(change, func(t *testing.T) {
			mr := miniredis.RunT(t)
			rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = rdb.Close() })
			conf := testConf(t, true)
			first, err := NewService(conf, rdb)
			require.NoError(t, err)
			login, err := first.Login(context.Background(), "admin", "adminpw", "127.0.0.1")
			require.NoError(t, err)
			if change == "deleted" {
				conf.Users = conf.Users[1:]
			} else {
				conf.Users[0].Role = RoleViewer
			}
			restarted, err := NewService(conf, rdb)
			require.NoError(t, err)
			sess, err := restarted.Authenticate(context.Background(), login.Token)
			if change == "deleted" {
				require.Error(t, err, "deleted user's session must be rejected after restart")
			} else {
				require.NoError(t, err)
				require.Equal(t, RoleViewer, sess.Role, "demoted user must not retain admin from stored session")
			}
		})
	}
}
