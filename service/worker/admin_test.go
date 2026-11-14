package worker

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAdminArgs(t *testing.T) {
	o, err := ParseAdminArgs([]string{"reset-password"}, io.Discard)
	require.NoError(t, err)
	require.Equal(t, ResetPasswordOptions{User: "admin", PasswordEnv: DefaultPasswordEnv}, o)

	o, err = ParseAdminArgs([]string{"reset-password", "--user", "ops", "--password-env", "X_PW"}, io.Discard)
	require.NoError(t, err)
	require.Equal(t, ResetPasswordOptions{User: "ops", PasswordEnv: "X_PW"}, o)

	_, err = ParseAdminArgs(nil, io.Discard)
	require.Error(t, err)
	_, err = ParseAdminArgs([]string{"nope"}, io.Discard)
	require.Error(t, err)
	_, err = ParseAdminArgs([]string{"reset-password", "extra"}, io.Discard)
	require.Error(t, err)
}
