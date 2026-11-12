package certificate

import (
	"fmt"
	"testing"

	"github.com/Jinnrry/pmail/internal/config"
)

func TestCheckSSLCrtInfo(t *testing.T) {
	config.Init()

	got, got1, match, err := CheckSSLCrtInfo()

	fmt.Println(got, got1, match, err)
}
