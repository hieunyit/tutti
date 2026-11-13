package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildReplicationID_NoUser(t *testing.T) {
	r := require.New(t)
	id := buildReplicationID("", "A", "B", "", "")
	r.Equal("", id.User)
	r.Nil(id.FromBucket)
	id = buildReplicationID("", "A", "B", "b1", "")
	r.Equal("b1", *id.ToBucket)
}

// --user must not be required on any repl subcommand.
func TestReplCommandsUserFlagOptional(t *testing.T) {
	r := require.New(t)
	for _, name := range []string{"add", "buckets", "delete", "pause", "resume"} {
		sub, _, err := replCmd.Find([]string{name})
		r.NoError(err)
		r.Equal(name, sub.Name())
		f := sub.Flags().Lookup("user")
		r.NotNil(f, name)
		_, required := f.Annotations["cobra_annotation_bash_completion_one_required_flag"]
		r.False(required, name)
	}
	for _, name := range []string{"zero-downtime", "downtime", "delete", "status"} {
		sub, _, err := switchCmd.Find([]string{name})
		r.NoError(err)
		f := sub.Flags().Lookup("user")
		r.NotNil(f, name)
		_, required := f.Annotations["cobra_annotation_bash_completion_one_required_flag"]
		r.False(required, name)
	}
}

func TestRequireSwitchUser(t *testing.T) {
	r := require.New(t)
	err := requireSwitchUser("")
	r.Error(err)
	r.Contains(err.Error(), "protection group switchover")
	r.NoError(requireSwitchUser("alice"))
}
