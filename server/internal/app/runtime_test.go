package app_test

import (
	"os"
	"testing"

	"github.com/Jinnrry/pmail/internal/bootstrap"
)

func TestMain(m *testing.M) { bootstrap.BindDependencies(); os.Exit(m.Run()) }
