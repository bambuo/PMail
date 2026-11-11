package app

import (
	"context"
	"net/http"

	"github.com/Jinnrry/pmail/dto/plugin"
	"github.com/Jinnrry/pmail/hooks/contract"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

type SessionPort interface {
	Put(context.Context, string, any)
	Get(context.Context, string) any
	Clear(context.Context) error
	LoadAndSave(http.Handler) http.Handler
}

var Sessions SessionPort
var AllHooks func() []contract.EmailHook
var HookNames func() []string
var GetHook func(string) (contract.EmailHook, bool)
var InitializeDatabase func(string) error
var GenerateDKIM func() string

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

var RemoveGroups func(*pcontext.Context, []int) (bool, error)

// WASM 插件管理端口，由 bootstrap 绑定到 infra 实现。
var PluginList func() []plugin.Info
var PluginApprove func(ctx *pcontext.Context, id string, hooks, capabilities []string) error
var PluginEnable func(ctx *pcontext.Context, id string) error
var PluginDisable func(ctx *pcontext.Context, id string) error
var PluginReject func(ctx *pcontext.Context, id string) error
var PluginReload func(ctx *pcontext.Context, id string) error
