package pop3

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Jinnrry/pmail/internal/config"

	"github.com/Jinnrry/pmail/internal/testutil/apitest"
)

func TestMain(m *testing.M) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	config.ROOT_PATH = filepath.Clean(filepath.Join(wd, "../../..")) + string(os.PathSeparator)
	apitest.BindDependencies()
	os.Exit(m.Run())
}
