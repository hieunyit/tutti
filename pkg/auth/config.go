// Package auth implements authentication and role based authorization for the
// tutti management API: bcrypt user store, Redis-backed sessions, login/logout
// HTTP endpoints and gRPC interceptors.
package auth

import (
	"fmt"
	"strings"
	"time"
)

const (
	RoleAdmin  = "admin"
	RoleViewer = "viewer"

	DefaultSessionTTL = 12 * time.Hour
)

// Config is the `api.auth` section of the worker config.
type Config struct {
	// Enabled turns on session authentication for the management API.
	Enabled bool `yaml:"enabled"`
	// SessionTTL is the lifetime of a login session.
	SessionTTL time.Duration `yaml:"sessionTTL"`
	// Users is the list of local users. Passwords are bcrypt hashes.
	Users []User `yaml:"users"`
}

type User struct {
	Username     string `yaml:"username"`
	PasswordHash string `yaml:"passwordHash"`
	Role         string `yaml:"role"`
}

func validHash(h string) bool {
	for _, p := range []string{"$2a$", "$2b$", "$2y$"} {
		if strings.HasPrefix(h, p) {
			return true
		}
	}
	return false
}

func (c *Config) Validate() error {
	if c == nil || !c.Enabled {
		return nil
	}
	if c.SessionTTL < 0 {
		return fmt.Errorf("auth config: sessionTTL must not be negative")
	}
	if len(c.Users) == 0 {
		return fmt.Errorf("auth config: at least one user is required when auth is enabled")
	}
	seen := make(map[string]struct{}, len(c.Users))
	for i, u := range c.Users {
		if u.Username == "" {
			return fmt.Errorf("auth config: users[%d]: username is required", i)
		}
		if _, ok := seen[u.Username]; ok {
			return fmt.Errorf("auth config: duplicate username %q", u.Username)
		}
		seen[u.Username] = struct{}{}
		if u.Role != RoleAdmin && u.Role != RoleViewer {
			return fmt.Errorf("auth config: user %q: role must be %q or %q, got %q", u.Username, RoleAdmin, RoleViewer, u.Role)
		}
		if !validHash(u.PasswordHash) {
			return fmt.Errorf("auth config: user %q: passwordHash must be a bcrypt hash ($2a$, $2b$ or $2y$ prefix)", u.Username)
		}
	}
	return nil
}

func (c *Config) ttl() time.Duration {
	if c.SessionTTL <= 0 {
		return DefaultSessionTTL
	}
	return c.SessionTTL
}
