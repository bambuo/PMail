package app

import (
	"fmt"
	"strings"

	"github.com/Jinnrry/pmail/config"

	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/errors"
	"github.com/Jinnrry/pmail/pkg/ip"
)

type SetupDNSItem struct {
	Type  string `json:"type"`
	Host  string `json:"host"`
	Value string `json:"value"`
	TTL   int    `json:"ttl"`
	Tips  string `json:"tips"`
}

func (s *SetupService) DNSSettings(ctx *context.Context) (map[string][]*SetupDNSItem, error) {
	configData, err := config.ReadConfig()
	if err != nil {
		return nil, errors.Wrap(err)
	}

	ret := make(map[string][]*SetupDNSItem)

	for _, domain := range configData.Domains {
		ret[domain] = []*SetupDNSItem{
			{Type: "A", Host: strings.ReplaceAll(configData.WebDomain, "."+configData.Domain, ""), Value: ip.GetIp(), TTL: 3600, Tips: TextGetText(ctx.Lang, "ip_taps")},
			{Type: "A", Host: "smtp", Value: ip.GetIp(), TTL: 3600, Tips: TextGetText(ctx.Lang, "ip_taps")},
			{Type: "A", Host: "imap", Value: ip.GetIp(), TTL: 3600, Tips: TextGetText(ctx.Lang, "ip_taps")},
			{Type: "A", Host: "pop", Value: ip.GetIp(), TTL: 3600, Tips: TextGetText(ctx.Lang, "ip_taps")},
			{Type: "A", Host: "@", Value: ip.GetIp(), TTL: 3600, Tips: TextGetText(ctx.Lang, "ip_taps")},
			{Type: "MX", Host: "@", Value: fmt.Sprintf("smtp.%s", domain), TTL: 3600},
			{Type: "TXT", Host: "@", Value: "v=spf1 a mx ~all", TTL: 3600},
			{Type: "TXT", Host: "default._domainkey", Value: s.dkim(), TTL: 3600},
		}
	}

	return ret, nil
}
