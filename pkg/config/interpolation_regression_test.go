package config

import (
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"os"
	"strings"
	"testing"
)

func TestInterpolation_LiteralBcryptHashMustSurviveYAMLLoading(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("review-dummy-password"), bcrypt.MinCost)
	require.NoError(t, err)
	doc, err := readYAMLMap(strings.NewReader("auth:\n  passwordHash: '" + string(hash) + "'\n"))
	require.NoError(t, err)
	got := doc["auth"].(map[string]any)["passwordHash"]
	require.Equal(t, string(hash), got, "literal bcrypt dollar signs must not be interpreted as env placeholders")
}
func TestInterpolation_InterpolatedSecretMustRemainYAMLScalar(t *testing.T) {
	secret := `dummy"quote\slash`
	t.Setenv("TUTTI_REVIEW_DUMMY_SECRET", secret)
	doc, err := readYAMLMap(strings.NewReader("redis:\n  password: \"${TUTTI_REVIEW_DUMMY_SECRET}\"\n"))
	require.NoError(t, err, "valid secret must not become YAML source code")
	require.Equal(t, secret, doc["redis"].(map[string]any)["password"])
}

func TestInterpolation_MissingVariableFailsClosed(t *testing.T) {
	const name = "TUTTI_MISSING_INTERPOLATION_REGRESSION"
	t.Setenv(name, "temporary")
	require.NoError(t, os.Unsetenv(name))
	doc := "redis:\n  password: '" + "$" + "{" + name + "}'\n"
	_, err := readYAMLMap(strings.NewReader(doc))
	require.ErrorContains(t, err, "required environment variable "+name+" is not set")
}
