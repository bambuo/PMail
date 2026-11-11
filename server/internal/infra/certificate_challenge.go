package infra

import (
	"time"

	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/go-acme/lego/v4/challenge/dns01"
	log "github.com/sirupsen/logrus"
)

type certificateAuthInfo struct {
	Domain  string
	Token   string
	KeyAuth string
}

type CertificateHttpChallenge struct {
	AuthInfo map[string]*certificateAuthInfo
}

var certificateInstance *CertificateHttpChallenge

func (h *CertificateHttpChallenge) Present(domain, token, keyAuth string) error {
	h.AuthInfo[token] = &certificateAuthInfo{
		Domain:  domain,
		Token:   token,
		KeyAuth: keyAuth,
	}

	return nil
}

func (h *CertificateHttpChallenge) CleanUp(domain, token, keyAuth string) error {
	delete(h.AuthInfo, token)
	return nil
}

func CertificateGetHttpChallengeInstance() *CertificateHttpChallenge {
	if certificateInstance == nil {
		certificateInstance = &CertificateHttpChallenge{
			AuthInfo: map[string]*certificateAuthInfo{},
		}
	}
	return certificateInstance
}

type CertificateDNSChallenge struct {
	AuthInfo map[string]*certificateAuthInfo
}

var certificateDnsInstance *CertificateDNSChallenge

func CertificateGetDnsChallengeInstance() *CertificateDNSChallenge {
	if certificateDnsInstance == nil {
		certificateDnsInstance = &CertificateDNSChallenge{
			AuthInfo: map[string]*certificateAuthInfo{},
		}
	}
	return certificateDnsInstance
}

func (h *CertificateDNSChallenge) Present(domain, token, keyAuth string) error {
	info := dns01.GetChallengeInfo(domain, keyAuth)
	log.Infof("Preparing DNS challenge for %s", domain)
	h.AuthInfo[token] = &certificateAuthInfo{
		Domain:  info.FQDN,
		Token:   token,
		KeyAuth: info.Value,
	}
	log.Infof("DNS challenge prepared for %s", domain)
	return nil
}

func (h *CertificateDNSChallenge) CleanUp(domain, token, keyAuth string) error {
	delete(h.AuthInfo, token)
	return nil
}

func (h *CertificateDNSChallenge) Timeout() (timeout, interval time.Duration) {
	return 60 * time.Minute, 5 * time.Second
}

type CertificateDNSItem struct {
	Type  string `json:"type"`
	Host  string `json:"host"`
	Value string `json:"value"`
	TTL   int    `json:"ttl"`
	Tips  string `json:"tips"`
}

func (h *CertificateDNSChallenge) GetDNSSettings(ctx *context.Context) []*CertificateDNSItem {
	ret := []*CertificateDNSItem{}
	for _, info := range h.AuthInfo {
		ret = append(ret, &CertificateDNSItem{
			Type:  "TXT",
			Host:  info.Domain,
			Value: info.KeyAuth,
			TTL:   3600,
		})
	}

	return ret
}
