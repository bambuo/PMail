package infra

import (
	"fmt"
	"testing"

	"github.com/Jinnrry/pmail/config"
)

func TestCheckSSLCrtInfo(t *testing.T) {
	config.Init()

	got, got1, match, err := CertificateCheckSSLCrtInfo()

	fmt.Println(got, got1, match, err)
}
