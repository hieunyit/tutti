package worker

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hieunyit/tutti/pkg/auth"
	"github.com/hieunyit/tutti/pkg/config"
	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/controlplane/pg"
	"github.com/hieunyit/tutti/pkg/util"
)

// DefaultPasswordEnv is the environment variable read by "admin reset-password" for an explicit password.
const DefaultPasswordEnv = "TUTTI_ADMIN_NEW_PASSWORD"

// ResetPasswordOptions are the parsed flags of "admin reset-password".
type ResetPasswordOptions struct {
	User        string
	PasswordEnv string
}

// ParseAdminArgs parses the arguments following the "admin" word. Only "reset-password" is supported.
func ParseAdminArgs(args []string, out io.Writer) (ResetPasswordOptions, error) {
	opts := ResetPasswordOptions{}
	if len(args) == 0 || args[0] != "reset-password" {
		return opts, fmt.Errorf("usage: worker admin reset-password [--user admin] [--password-env %s]", DefaultPasswordEnv)
	}
	fs := flag.NewFlagSet("admin reset-password", flag.ContinueOnError)
	fs.SetOutput(out)
	fs.StringVar(&opts.User, "user", "admin", "console user to reset (created as admin when missing)")
	fs.StringVar(&opts.PasswordEnv, "password-env", DefaultPasswordEnv,
		"environment variable holding the new password; a random one is generated and printed when it is empty or unset")
	if err := fs.Parse(args[1:]); err != nil {
		return opts, err
	}
	if fs.NArg() > 0 {
		return opts, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	return opts, nil
}

// RunAdminResetPassword connects to the same Redis and PostgreSQL as the server and resets a console user password.
func RunAdminResetPassword(ctx context.Context, conf *Config, opts ResetPasswordOptions, out io.Writer) error {
	if conf.Api == nil || !conf.Api.Auth.Enabled {
		return errors.New("console sign-in is disabled (api.auth.enabled=false): nothing to reset")
	}
	confRedis := util.NewRedisRole(conf.Redis, config.RedisRoleConfig)
	defer confRedis.Close()
	if err := confRedis.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("unable to reach redis: %w", err)
	}
	authOpts := []auth.Option{auth.WithManagedUsers(confRedis)}
	if conf.ControlPlane.WithDefaults().Backend == controlplane.BackendPostgres {
		pgStore, err := pg.OpenLazy(ctx, conf.ControlPlane)
		if err != nil {
			return fmt.Errorf("unable to open control-plane postgres store: %w", err)
		}
		defer pgStore.Close()
		if err := pgStore.TryReady(ctx); err != nil {
			return fmt.Errorf("control-plane postgres is not reachable: %w", err)
		}
		authOpts = append(authOpts, auth.WithControlPlane(pgStore))
	}
	svc, err := auth.NewService(conf.Api.Auth, confRedis, authOpts...)
	if err != nil {
		return err
	}
	password := strings.TrimSpace(os.Getenv(opts.PasswordEnv))
	generated, err := svc.ResetUserPassword(ctx, opts.User, password)
	if err != nil {
		return err
	}
	if generated != "" {
		auth.PrintPasswordBanner("NEW ADMIN PASSWORD", opts.User, generated)
	} else {
		_, _ = fmt.Fprintf(out, "password of console user %q reset; all its sessions were revoked\n", opts.User)
	}
	return nil
}
