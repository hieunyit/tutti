package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseReplicationRef(t *testing.T) {
	r := require.New(t)
	id, err := parseReplicationRef("alice:main:backup")
	r.NoError(err)
	r.Equal("alice", id.User)
	r.Nil(id.FromBucket)

	id, err = parseReplicationRef(":main:backup:photos:photos2")
	r.NoError(err)
	r.Equal("", id.User)
	r.Equal("photos", id.GetFromBucket())
	r.Equal("photos2", id.GetToBucket())

	for _, bad := range []string{"", "a:b", "a:b:c:d", ":main::", "a:b:c:d:"} {
		_, err = parseReplicationRef(bad)
		r.Error(err, bad)
	}
}
