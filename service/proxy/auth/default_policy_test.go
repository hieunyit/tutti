package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDefaultPolicy(t *testing.T) {
	for _, v := range []string{"", "deny", "allow-legacy"} {
		require.NoError(t, (&Config{DefaultPolicy: v}).ValidateDefaultPolicy(), v)
	}
	require.ErrorContains(t, (&Config{DefaultPolicy: "allow"}).ValidateDefaultPolicy(), "defaultPolicy")
}
