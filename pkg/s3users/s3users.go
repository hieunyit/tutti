// Package s3users manages S3 users: identities that have credentials on each storage (from the worker config or
// dynamic credentials), proxy access keys and an access policy.
package s3users

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/hieunyit/tutti/pkg/accesspolicy"
	"github.com/hieunyit/tutti/pkg/controlplane"
	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
	"github.com/hieunyit/tutti/pkg/proxyaccount"
	"github.com/hieunyit/tutti/pkg/proxystatus"
	"github.com/hieunyit/tutti/pkg/s3"
)

// Credentials is the part of objstore.CredsService used by the manager.
type Credentials interface {
	MainStorage() string
	Storages() map[string]dom.StorageType
	ListUsers(storage string) []string
	// StoragesWithOverrideUser is the set of storages on which the user has its own credentials (one cache pass).
	// AllOverrideUsers is the union of the users with own credentials over all storages (one cache pass).
	AllOverrideUsers() map[string]struct{}
	StoragesWithOverrideUser(user string) map[string]struct{}
	HasUser(storage, user string) error
	GetS3Credentials(storage, user string) (s3.CredentialsV4, error)
	ResolveS3Credentials(storage, user string) (s3.CredentialsV4, objstore.CredMode, error)
	SetS3Credentials(ctx context.Context, storage, user string, cred s3.CredentialsV4) error
	DeleteUserCredentials(ctx context.Context, storage, user string) error
	CredentialSource(storage, user string) string
	DynamicCredentialsEnabled() bool
	// ConnectionAccount reports whether the S3 storage has a connection account.
	ConnectionAccount(storage string) (objstore.ConnectionInfo, bool)
	// Identities are the users recorded in Tutti, including users without any storage credentials.
	Identities() []string
	HasIdentity(user string) bool
	AddIdentity(ctx context.Context, user string) error
	RemoveIdentity(ctx context.Context, user string) error
}

var _ Credentials = objstore.CredsService(nil)

// Verifier performs a cheap authenticated call against the storage with the credentials.
type Verifier func(ctx context.Context, storage string, cred s3.CredentialsV4) error

// References returns human-readable reasons why the user is still in use (replications, routings, failover events).
type References func(ctx context.Context, user string) ([]string, error)

// Manager implements the S3 user operations.
type Manager struct {
	Creds    Credentials
	Accounts *proxyaccount.Store
	Policies *accesspolicy.Store
	Redis    redis.Cmdable // proxy status documents (static key counts)
	CP       controlplane.Store
	Verify   Verifier
	Refs     References
}

func (m *Manager) controlPlane() controlplane.Store {
	if m.CP != nil {
		return m.CP
	}
	if cpHolder, ok := m.Creds.(interface{ ControlPlane() controlplane.Store }); ok {
		return cpHolder.ControlPlane()
	}
	return nil
}

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// ValidName checks a user name: 1-64 characters, letters, digits, '.', '_' and '-', starting with a letter or digit.
func ValidName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("%w: user name must be 1-64 characters: letters, digits, '.', '_' and '-', starting with a letter or digit", dom.ErrInvalidArg)
	}
	return nil
}

// StorageState is the credential state of a user on one storage.
type StorageState struct {
	Storage string
	// Configured is true if the user has its own credentials (override) on the storage.
	Configured  bool
	Source      string
	AccessKeyID string
	// Mode is how requests of the user are signed on the storage: objstore.ModeOverride, ModeConnection or ModeMissing.
	Mode objstore.CredMode
}

type KeyCounts struct{ Static, DynamicActive, DynamicDisabled int }

type PolicySummary struct {
	HasPolicy  bool
	Permission string
	Buckets    []string
}

// User is the listing view of an S3 user. It never contains secrets.
type User struct {
	Name     string
	Storages []StorageState
	// Complete is true if requests of the user resolve on every storage (override or connection account).
	Complete  bool
	KeyCounts KeyCounts
	Policy    PolicySummary
}

// Listing is the result of List.
type Listing struct {
	Users       []User
	MainStorage string
	Storages    []string
	Dynamic     bool
}

func (m *Manager) storageNames() []string {
	types := m.Creds.Storages()
	names := make([]string, 0, len(types))
	for n := range types {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (m *Manager) hasConnectionAccount(storage string) bool {
	if m.Creds.Storages()[storage] != dom.S3 {
		return false
	}
	_, ok := m.Creds.ConnectionAccount(storage)
	return ok
}

func (m *Manager) accessKeyID(storage, user string) string {
	if m.Creds.Storages()[storage] != dom.S3 {
		return ""
	}
	c, err := m.Creds.GetS3Credentials(storage, user)
	if err != nil {
		return ""
	}
	return c.AccessKeyID
}

// SharedStorages lists the S3 storages on which requests of the user are signed with the shared connection account
// (the user has no credentials of its own there). Such a user is denied everything by the proxy unless it has an
// access policy.
func (m *Manager) SharedStorages(user string) []string {
	var res []string
	for _, st := range m.storageNames() {
		if m.Creds.Storages()[st] != dom.S3 {
			continue
		}
		if _, mode, _ := m.Creds.ResolveS3Credentials(st, user); mode == objstore.ModeConnection {
			res = append(res, st)
		}
	}
	return res
}

// known reports whether the user exists in Tutti (identity or credentials on any storage).
func (m *Manager) known(user string) bool {
	if m.Creds.HasIdentity(user) {
		return true
	}
	for _, st := range m.storageNames() {
		if m.Creds.CredentialSource(st, user) != "" {
			return true
		}
	}
	return false
}

// CheckPolicyDeletable refuses to delete the policy of a user that would lose all access: a user on a shared
// connection account without a policy is denied by the proxy.
func (m *Manager) CheckPolicyDeletable(user string) error {
	if !m.known(user) {
		return nil
	}
	if shared := m.SharedStorages(user); len(shared) > 0 {
		return fmt.Errorf("%w: user %q uses the shared connection account of storage(s) %s: without an access policy the user would lose all access; set another policy instead or delete the user",
			dom.ErrFailedPrecondition, user, strings.Join(shared, ", "))
	}
	return nil
}

// List returns the union of users over all storages.
func (m *Manager) List(ctx context.Context) (*Listing, error) {
	storages := m.storageNames()
	names := map[string]struct{}{}
	// every identity is added below, so the connection-account part of ListUsers adds nothing: one cache pass
	for u := range m.Creds.AllOverrideUsers() {
		names[u] = struct{}{}
	}
	for _, u := range m.Creds.Identities() {
		names[u] = struct{}{}
	}
	static, err := m.staticKeyCounts(ctx)
	if err != nil {
		return nil, err
	}
	dyn, err := m.Accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	policies, err := m.Policies.List(ctx)
	if err != nil {
		return nil, err
	}
	pol := make(map[string]accesspolicy.Policy, len(policies))
	for _, p := range policies {
		pol[p.User] = p
	}
	counts := map[string]*KeyCounts{}
	for _, a := range dyn {
		c := counts[a.User]
		if c == nil {
			c = &KeyCounts{}
			counts[a.User] = c
		}
		if a.Status == proxyaccount.StatusActive {
			c.DynamicActive++
		} else {
			c.DynamicDisabled++
		}
	}
	res := &Listing{MainStorage: m.Creds.MainStorage(), Storages: storages, Dynamic: m.Creds.DynamicCredentialsEnabled()}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	for _, n := range sorted {
		u := User{Name: n, Complete: true}
		for _, st := range storages {
			src := m.Creds.CredentialSource(st, n)
			state := StorageState{Storage: st, Configured: src != "", Source: src, Mode: objstore.ModeMissing}
			switch {
			case state.Configured:
				state.AccessKeyID = m.accessKeyID(st, n)
				state.Mode = objstore.ModeOverride
			case m.hasConnectionAccount(st):
				state.Mode = objstore.ModeConnection
			default:
				u.Complete = false
			}
			u.Storages = append(u.Storages, state)
		}
		if c := counts[n]; c != nil {
			u.KeyCounts = *c
		}
		u.KeyCounts.Static = static[n]
		if p, ok := pol[n]; ok {
			u.Policy = PolicySummary{HasPolicy: true, Permission: p.Permission, Buckets: p.Buckets}
		} else {
			d := accesspolicy.Default(n)
			u.Policy = PolicySummary{Permission: d.Permission, Buckets: d.Buckets}
		}
		res.Users = append(res.Users, u)
	}
	return res, nil
}

// staticKeyCounts counts the distinct config-defined access keys per user reported by live proxies.
func (m *Manager) staticKeyCounts(ctx context.Context) (map[string]int, error) {
	res := map[string]int{}
	if m.Redis == nil {
		return res, nil
	}
	statuses, err := proxystatus.List(ctx, m.Redis)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, st := range statuses {
		for _, a := range st.Accounts {
			if a.Source == "dynamic" {
				continue
			}
			k := a.User + "\x00" + a.AccessKeyID
			if _, ok := seen[k]; ok {
				continue
			}
			seen[k] = struct{}{}
			res[a.User]++
		}
	}
	return res, nil
}

// Get returns one user of the listing.
func (m *Manager) Get(ctx context.Context, name string) (User, error) {
	l, err := m.List(ctx)
	if err != nil {
		return User{}, err
	}
	for _, u := range l.Users {
		if u.Name == name {
			return u, nil
		}
	}
	return User{}, fmt.Errorf("%w: s3 user %q", dom.ErrNotFound, name)
}

// Credential is the credential of a user on one storage.
type Credential struct {
	Storage         string
	AccessKeyID     string
	SecretAccessKey string
}

// VerifyResult is the outcome of checking one credential.
type VerifyResult struct {
	Storage   string
	OK        bool
	Error     string
	LatencyMS int64
}

// CreateRequest is the input of Create.
type CreateRequest struct {
	Name            string
	Credentials     []Credential
	Verify          bool
	CreateAccessKey bool
	Description     string
	Policy          *accesspolicy.Policy
}

// CreateResult is the output of Create.
type CreateResult struct {
	User         User
	AccessKey    *proxyaccount.Account
	AccessSecret string
	Verification []VerifyResult
}

func (m *Manager) checkS3Storage(storage string) error {
	t, ok := m.Creds.Storages()[storage]
	if !ok {
		return fmt.Errorf("%w: storage %q not found", dom.ErrInvalidArg, storage)
	}
	if t != dom.S3 {
		return fmt.Errorf("%w: storage %q is not an S3 storage: only S3 storages are supported", dom.ErrInvalidArg, storage)
	}
	return nil
}

func (m *Manager) requireDynamic() error {
	if !m.Creds.DynamicCredentialsEnabled() {
		return fmt.Errorf("%w: dynamic credentials are not enabled (storage.dynamicCredentials.enabled with a masterPassword): S3 users cannot be created at runtime", dom.ErrFailedPrecondition)
	}
	return nil
}

// verifyAll checks the credentials and returns the results; the error lists all failures.
func (m *Manager) verifyAll(ctx context.Context, creds []Credential) ([]VerifyResult, error) {
	res := make([]VerifyResult, 0, len(creds))
	var failed []string
	for _, c := range creds {
		r := m.verifyOne(ctx, c.Storage, s3.CredentialsV4{AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey})
		res = append(res, r)
		if !r.OK {
			failed = append(failed, fmt.Sprintf("storage %q: %s", c.Storage, r.Error))
		}
	}
	if len(failed) > 0 {
		return res, fmt.Errorf("%w: credential verification failed: %s", dom.ErrInvalidArg, strings.Join(failed, "; "))
	}
	return res, nil
}

func (m *Manager) verifyOne(ctx context.Context, storage string, cred s3.CredentialsV4) VerifyResult {
	r := VerifyResult{Storage: storage}
	if m.Verify == nil {
		r.Error = "credential verification is not available on this server"
		return r
	}
	start := time.Now()
	err := m.Verify(ctx, storage, cred)
	r.LatencyMS = time.Since(start).Milliseconds()
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.OK = true
	return r
}

// Create creates a user with dynamic credentials. Steps: validate, verify, store credentials, set policy, create
// the access key. A failing step rolls back the previous ones (best effort).
func (m *Manager) Create(ctx context.Context, req CreateRequest) (*CreateResult, error) {
	if err := ValidName(req.Name); err != nil {
		return nil, err
	}
	var result *CreateResult
	err := m.withUserLock(ctx, req.Name, func(ctx context.Context) error {
		var err error
		result, err = m.createLocked(ctx, req)
		return err
	})
	return result, err
}

func (m *Manager) createLocked(ctx context.Context, req CreateRequest) (*CreateResult, error) {
	if err := m.requireDynamic(); err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, c := range req.Credentials {
		if err := m.checkS3Storage(c.Storage); err != nil {
			return nil, err
		}
		if _, dup := seen[c.Storage]; dup {
			return nil, fmt.Errorf("%w: duplicate credentials for storage %q", dom.ErrInvalidArg, c.Storage)
		}
		seen[c.Storage] = struct{}{}
		if err := (s3.CredentialsV4{AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey}).Validate(); err != nil {
			return nil, fmt.Errorf("%w: storage %q: accessKeyId and secretAccessKey are required", dom.ErrInvalidArg, c.Storage)
		}
	}
	if _, ok := seen[m.Creds.MainStorage()]; !ok && !m.hasConnectionAccount(m.Creds.MainStorage()) {
		return nil, fmt.Errorf("%w: credentials for the main storage %q are required (it has no connection account)", dom.ErrInvalidArg, m.Creds.MainStorage())
	}
	if len(req.Credentials) == 0 {
		// a user without any storage credentials must resolve on every S3 storage through its connection account
		var missing []string
		for _, st := range m.storageNames() {
			if m.Creds.Storages()[st] == dom.S3 && !m.hasConnectionAccount(st) {
				missing = append(missing, st)
			}
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("%w: storage(s) %s have no connection account: give the user credentials for them or add a connection account to the storage", dom.ErrInvalidArg, strings.Join(missing, ", "))
		}
	}
	if req.CreateAccessKey {
		if err := m.Accounts.SealingAvailable(); err != nil {
			return nil, err
		}
	}
	// same result as scanning ListUsers(st) of every storage, without one credential-cache scan per storage: the user
	// is listed on a storage when it has own credentials there, or is a Tutti identity and the storage has a
	// connection account.
	withCreds := m.Creds.StoragesWithOverrideUser(req.Name)
	isIdentity := m.Creds.HasIdentity(req.Name)
	for _, st := range m.storageNames() {
		_, listed := withCreds[st]
		if !listed && isIdentity {
			_, listed = m.Creds.ConnectionAccount(st)
		}
		if listed {
			return nil, fmt.Errorf("%w: s3 user %q already exists (storage %q)", dom.ErrAlreadyExists, req.Name, st)
		}
	}
	if isIdentity {
		return nil, fmt.Errorf("%w: s3 user %q already exists", dom.ErrAlreadyExists, req.Name)
	}
	if req.Policy == nil {
		var shared []string
		for _, st := range m.storageNames() {
			if _, has := seen[st]; !has && m.hasConnectionAccount(st) {
				shared = append(shared, st)
			}
		}
		if len(shared) > 0 {
			return nil, fmt.Errorf("%w: a policy is required: the user uses the shared connection account of storage(s) %s and would have no access without one (set permission and bucket patterns)",
				dom.ErrInvalidArg, strings.Join(shared, ", "))
		}
	}
	var pol *accesspolicy.Policy
	if req.Policy != nil {
		p := *req.Policy
		p.User = req.Name
		if err := p.Validate(); err != nil {
			return nil, err
		}
		pol = &p
	}

	res := &CreateResult{}
	if req.Verify {
		v, err := m.verifyAll(ctx, req.Credentials)
		res.Verification = v
		if err != nil {
			return res, err
		}
	}

	var setStorages []string
	identityAdded := false
	rollback := func() {
		bg := context.WithoutCancel(ctx)
		for _, st := range setStorages {
			_ = m.Creds.DeleteUserCredentials(bg, st, req.Name)
		}
		if identityAdded {
			_ = m.Creds.RemoveIdentity(bg, req.Name)
		}
	}
	// the user record first: a user without storage credentials exists only as this record
	if err := m.Creds.AddIdentity(ctx, req.Name); err != nil {
		return nil, err
	}
	identityAdded = true
	for _, c := range req.Credentials {
		err := m.Creds.SetS3Credentials(ctx, c.Storage, req.Name, s3.CredentialsV4{AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey})
		if err != nil {
			// the failing call may have stored the entry before the cache sync failed
			setStorages = append(setStorages, c.Storage)
			rollback()
			return nil, err
		}
		setStorages = append(setStorages, c.Storage)
	}
	if pol != nil {
		if err := m.Policies.Set(ctx, *pol); err != nil {
			rollback()
			return nil, err
		}
	}
	if req.CreateAccessKey {
		acc, secret, err := m.Accounts.Create(ctx, req.Name, req.Description)
		if err != nil {
			if pol != nil {
				_ = m.Policies.Delete(context.WithoutCancel(ctx), req.Name)
			}
			rollback()
			return nil, err
		}
		res.AccessKey, res.AccessSecret = &acc, secret
	}
	u, err := m.Get(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	res.User = u
	return res, nil
}

// UpdateCredentials sets or replaces the dynamic credentials of an existing user on one storage.
func (m *Manager) UpdateCredentials(ctx context.Context, user string, c Credential, verify bool) error {
	if err := ValidName(user); err != nil {
		return err
	}
	return m.withUserLock(ctx, user, func(ctx context.Context) error {
		return m.updateCredentialsLocked(ctx, user, c, verify)
	})
}

func (m *Manager) updateCredentialsLocked(ctx context.Context, user string, c Credential, verify bool) error {
	if err := m.requireDynamic(); err != nil {
		return err
	}
	if err := m.checkS3Storage(c.Storage); err != nil {
		return err
	}
	if err := (s3.CredentialsV4{AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey}).Validate(); err != nil {
		return fmt.Errorf("%w: accessKeyId and secretAccessKey are required", dom.ErrInvalidArg)
	}
	exists := m.Creds.HasIdentity(user)
	for _, st := range m.storageNames() {
		if m.Creds.CredentialSource(st, user) != "" {
			exists = true
		}
	}
	if !exists {
		return fmt.Errorf("%w: s3 user %q", dom.ErrNotFound, user)
	}
	if m.Creds.CredentialSource(c.Storage, user) == objstore.CredSourceConfig {
		return fmt.Errorf("%w: credentials of user %q on storage %q are defined in config and cannot be changed", dom.ErrFailedPrecondition, user, c.Storage)
	}
	if verify {
		if _, err := m.verifyAll(ctx, []Credential{c}); err != nil {
			return err
		}
	}
	return m.Creds.SetS3Credentials(ctx, c.Storage, user, s3.CredentialsV4{AccessKeyID: c.AccessKeyID, SecretAccessKey: c.SecretAccessKey})
}

// DeleteCredentials removes the dynamic per-user override of one storage. Config credentials cannot be removed.
// The removal is refused when the user would be left without credentials on the storage (no override, no
// connection account) while replications, routings or failover events still reference the user.
func (m *Manager) DeleteCredentials(ctx context.Context, user, storage string) error {
	if err := ValidName(user); err != nil {
		return err
	}
	return m.withUserLock(ctx, user, func(ctx context.Context) error {
		return m.deleteCredentialsLocked(ctx, user, storage)
	})
}

func (m *Manager) deleteCredentialsLocked(ctx context.Context, user, storage string) error {
	if err := m.requireDynamic(); err != nil {
		return err
	}
	if err := m.checkS3Storage(storage); err != nil {
		return err
	}
	switch m.Creds.CredentialSource(storage, user) {
	case "":
		return fmt.Errorf("%w: user %q has no credentials on storage %q", dom.ErrNotFound, user, storage)
	case objstore.CredSourceConfig:
		return fmt.Errorf("%w: credentials of user %q on storage %q are defined in config and cannot be removed", dom.ErrFailedPrecondition, user, storage)
	}
	if !m.hasConnectionAccount(storage) && m.Refs != nil {
		refs, err := m.Refs(ctx, user)
		if err != nil {
			return err
		}
		if len(refs) > 0 {
			return fmt.Errorf("%w: user %q would be left without credentials on storage %q (no connection account) while still in use: %s",
				dom.ErrFailedPrecondition, user, storage, strings.Join(refs, "; "))
		}
	}
	if err := m.Creds.DeleteUserCredentials(ctx, storage, user); err != nil && !errors.Is(err, dom.ErrNotFound) {
		return err
	}
	return nil
}

// DeleteResult is the output of Delete.
type DeleteResult struct {
	DeletedStorages   []string
	DeletedAccessKeys int
	DeletedPolicy     bool
}

// Delete removes the dynamic credentials, dynamic access keys and the access policy of a user. It refuses if any
// credentials come from config or the user is referenced by replications, routings or failover events; active
// access keys block deletion unless force is set.
func (m *Manager) Delete(ctx context.Context, name string, force bool) (*DeleteResult, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: user name is required", dom.ErrInvalidArg)
	}
	var result *DeleteResult
	err := m.withUserLock(ctx, name, func(ctx context.Context) error {
		var err error
		result, err = m.deleteLocked(ctx, name, force)
		return err
	})
	return result, err
}

func (m *Manager) deleteLocked(ctx context.Context, name string, force bool) (*DeleteResult, error) {
	var dynamicStorages, configStorages []string
	for _, st := range m.storageNames() {
		switch m.Creds.CredentialSource(st, name) {
		case objstore.CredSourceConfig:
			configStorages = append(configStorages, st)
		case objstore.CredSourceDynamic:
			dynamicStorages = append(dynamicStorages, st)
		}
	}
	if len(configStorages)+len(dynamicStorages) == 0 && !m.Creds.HasIdentity(name) {
		return nil, fmt.Errorf("%w: s3 user %q", dom.ErrNotFound, name)
	}
	var reasons []string
	if len(configStorages) > 0 {
		reasons = append(reasons, fmt.Sprintf("credentials are defined in the worker config on storage(s) %s and must be removed there", strings.Join(configStorages, ", ")))
	}
	if m.Refs != nil {
		refs, err := m.Refs(ctx, name)
		if err != nil {
			return nil, err
		}
		reasons = append(reasons, refs...)
	}
	accounts, err := m.Accounts.List(ctx)
	if err != nil {
		return nil, err
	}
	var keys []proxyaccount.Account
	active := 0
	for _, a := range accounts {
		if a.User == name {
			keys = append(keys, a)
			if a.Status == proxyaccount.StatusActive {
				active++
			}
		}
	}
	if active > 0 && !force {
		reasons = append(reasons, fmt.Sprintf("%d active proxy access key(s) exist: disable them or set force=true to delete them", active))
	}
	if len(reasons) > 0 {
		return nil, fmt.Errorf("%w: s3 user %q cannot be deleted: %s", dom.ErrFailedPrecondition, name, strings.Join(reasons, "; "))
	}

	res := &DeleteResult{}
	if cp := m.controlPlane(); cp != nil {
		if err := cp.Tx(ctx, func(repos controlplane.Repos) error {
			txRefs, err := UserReferencesFromRepos(ctx, repos, name)
			if err != nil {
				return err
			}
			txAccounts, err := repos.ProxyAccounts.List(ctx)
			if err != nil {
				return err
			}
			var txKeys []controlplane.ProxyAccountRecord
			txActive := 0
			for _, a := range txAccounts {
				if a.Value.User == name {
					txKeys = append(txKeys, a.Value)
					if a.Value.Status == proxyaccount.StatusActive {
						txActive++
					}
				}
			}
			if txActive > 0 && !force {
				txRefs = append(txRefs, fmt.Sprintf("%d active proxy access key(s) exist: disable them or set force=true to delete them", txActive))
			}
			if len(txRefs) > 0 {
				return fmt.Errorf("%w: s3 user %q cannot be deleted: %s", dom.ErrFailedPrecondition, name, strings.Join(txRefs, "; "))
			}
			for _, k := range txKeys {
				if err := repos.ProxyAccounts.Delete(ctx, k.AccessKeyID); err != nil && !errors.Is(err, dom.ErrNotFound) {
					return err
				}
				res.DeletedAccessKeys++
			}
			if err := repos.AccessPolicies.Delete(ctx, name); err == nil {
				res.DeletedPolicy = true
			} else if !errors.Is(err, dom.ErrNotFound) {
				return err
			}
			for _, st := range dynamicStorages {
				if err := repos.DynamicCredentials.Delete(ctx, st, name); err != nil && !errors.Is(err, dom.ErrNotFound) {
					return err
				}
				res.DeletedStorages = append(res.DeletedStorages, st)
			}
			if err := repos.S3Identities.Delete(ctx, name); err != nil && !errors.Is(err, dom.ErrNotFound) {
				return err
			}
			return nil
		}); err != nil {
			return nil, controlplane.WrapWriteError(err)
		}
		if syncer, ok := m.Creds.(interface{ SyncAfterTx(context.Context) error }); ok {
			if err := syncer.SyncAfterTx(ctx); err != nil {
				return res, err
			}
		}
		return res, nil
	}
	// cut access first (keys), then the policy, then the credentials: a retry after a failure finishes the job
	for _, k := range keys {
		if err := m.Accounts.Delete(ctx, k.AccessKeyID); err != nil && !errors.Is(err, dom.ErrNotFound) {
			return res, err
		}
		res.DeletedAccessKeys++
	}
	if err := m.Policies.Delete(ctx, name); err == nil {
		res.DeletedPolicy = true
	} else if !errors.Is(err, dom.ErrNotFound) {
		return res, err
	}
	for _, st := range dynamicStorages {
		if err := m.Creds.DeleteUserCredentials(ctx, st, name); err != nil && !errors.Is(err, dom.ErrNotFound) {
			return res, err
		}
		res.DeletedStorages = append(res.DeletedStorages, st)
	}
	if err := m.Creds.RemoveIdentity(ctx, name); err != nil {
		return res, err
	}
	return res, nil
}

// Test checks the given credentials, or the stored ones if cred is nil, against the storage.
func (m *Manager) Test(ctx context.Context, user, storage string, cred *Credential) (VerifyResult, error) {
	if err := m.checkS3Storage(storage); err != nil {
		return VerifyResult{}, err
	}
	var c s3.CredentialsV4
	if cred != nil {
		c = s3.CredentialsV4{AccessKeyID: cred.AccessKeyID, SecretAccessKey: cred.SecretAccessKey}
		if err := c.Validate(); err != nil {
			return VerifyResult{}, fmt.Errorf("%w: accessKeyId and secretAccessKey are required", dom.ErrInvalidArg)
		}
	} else {
		if user == "" {
			return VerifyResult{}, fmt.Errorf("%w: user is required when no credentials are given", dom.ErrInvalidArg)
		}
		var err error
		c, _, err = m.Creds.ResolveS3Credentials(storage, user)
		if err != nil {
			return VerifyResult{}, fmt.Errorf("%w: user %q has no credentials on storage %q", dom.ErrNotFound, user, storage)
		}
	}
	return m.verifyOne(ctx, storage, c), nil
}
