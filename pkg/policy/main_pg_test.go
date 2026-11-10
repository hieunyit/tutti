package policy

import (
	"os"
	"testing"

	"github.com/hieunyit/tutti/pkg/controlplane/pgtest"
)

func TestMain(m *testing.M) { os.Exit(pgtest.Run(m)) }
