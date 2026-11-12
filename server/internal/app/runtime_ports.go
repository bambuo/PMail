package app

import (
	"context"
	"net/http"

	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

type SessionPort interface {
	Put(context.Context, string, any)
	Get(context.Context, string) any
	Clear(context.Context) error
	LoadAndSave(http.Handler) http.Handler
}

var Sessions SessionPort

// Certificate operations are bound by bootstrap, keeping certificate libraries outside use cases.
var GetCertificateType func() string
var SetCertificate func(string, string, string) error
var GenerateCertificate func(bool) error
var HTTPChallenge func(string) (string, bool)

type DNSChallengeRecord struct {
	Type  string `json:"type"`
	Host  string `json:"host"`
	Value string `json:"value"`
	TTL   int    `json:"ttl"`
	Tips  string `json:"tips"`
}

var GetDNSChallengeRecords func(*pcontext.Context) []*DNSChallengeRecord
