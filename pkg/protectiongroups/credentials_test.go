// Copyright 2026 hieunyit. SPDX-License-Identifier: Apache-2.0
package protectiongroups

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hieunyit/tutti/pkg/dom"
	"github.com/hieunyit/tutti/pkg/objstore"
)

// fakeCreds implements the parts of objstore.CredsService used by CredentialValidator (the embedded nil
// interface panics on anything else).
type fakeCreds struct {
	objstore.CredsService
	identities []string
	overrides  map[string][]string
}

func (f fakeCreds) HasIdentity(u string) bool {
	for _, i := range f.identities {
		if i == u {
			return true
		}
	}
	return false
}
func (f fakeCreds) Identities() []string                      { return f.identities }
func (f fakeCreds) ListOverrideUsers(storage string) []string { return f.overrides[storage] }

// The validator has no implicit users: neither "default" nor any name when no identities exist.
func TestCredentialValidatorRequiresExistingUser(t *testing.T) {
	g := Group{ID: "g", Members: []string{"a", "b"}, Copies: 2}
	for _, tc := range []struct {
		name  string
		creds fakeCreds
		user  string
	}{
		{"default without identities", fakeCreds{}, "default"},
		{"any name without identities", fakeCreds{}, "whoever"},
		{"default with identities", fakeCreds{identities: []string{"alice"}}, "default"},
		{"unknown user", fakeCreds{identities: []string{"alice"}, overrides: map[string][]string{"a": {"bob"}}}, "carol"},
	} {
		g.User = tc.user
		err := CredentialValidator(tc.creds, nil)(t.Context(), g)
		require.ErrorIs(t, err, dom.ErrInvalidArg, tc.name)
	}
}
