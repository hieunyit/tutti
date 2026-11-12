// Package auth implements authentication and role based authorization for the
// tutti management API: bcrypt user store, Redis-backed sessions, login/logout
// HTTP endpoints and gRPC interceptors.
package auth

import (
	"fmt"
	"net"
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
	// AllowManagedUsers allows running without static config users when managed users (PostgreSQL/Redis) are enabled.
	AllowManagedUsers bool `yaml:"allowManagedUsers"`
	// BootstrapAdminPassword is the initial password for the seeded admin user when the database is empty.
	// Can also be set via TUTTI_BOOTSTRAP_ADMIN_PASSWORD environment variable.
	BootstrapAdminPassword string `yaml:"bootstrapAdminPassword"`
	// TrustedProxies lists CIDRs or IPs of reverse proxies (e.g. nginx serving
	// the web UI) whose X-Forwarded-For / X-Real-IP headers are trusted when
	// resolving the client address for login rate limiting. Empty by default:
	// the TCP peer address is used.
	TrustedProxies []string `yaml:"trustedProxies"`
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

// parseTrustedProxies parses CIDRs and bare IPs.
func parseTrustedProxies(entries []string) ([]*net.IPNet, error) {
	var nets []*net.IPNet
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if _, n, err := net.ParseCIDR(e); err == nil {
			nets = append(nets, n)
			continue
		}
		ip := net.ParseIP(e)
		if ip == nil {
			return nil, fmt.Errorf("auth config: trustedProxies: %q is not a valid IP or CIDR", e)
		}
		bits := 8 * net.IPv6len
		if v4 := ip.To4(); v4 != nil {
			ip, bits = v4, 8*net.IPv4len
		}
		nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
	}
	return nets, nil
}

func (c *Config) Validate() error {
	if c == nil || !c.Enabled {
		return nil
	}
	if _, err := parseTrustedProxies(c.TrustedProxies); err != nil {
		return err
	}
	if c.SessionTTL < 0 {
		return fmt.Errorf("auth config: sessionTTL must not be negative")
	}
	if len(c.Users) == 0 && !c.AllowManagedUsers {
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
