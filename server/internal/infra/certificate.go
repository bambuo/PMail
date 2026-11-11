package infra

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"

	"github.com/Jinnrry/pmail/config"

	"os"
	"strings"
	"time"

	"github.com/Jinnrry/pmail/pkg/async"
	"github.com/Jinnrry/pmail/pkg/errors"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

type CertificateMyUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *CertificateMyUser) GetEmail() string {
	return u.Email
}
func (u CertificateMyUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *CertificateMyUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

func CertificateGetSSL() string {
	cfg, err := config.ReadConfig()
	if err != nil {
		panic(err)
	}
	if cfg.SSLType == "" {
		return config.SSLTypeAutoHTTP
	}

	return cfg.SSLType
}

func CertificateSetSSL(sslType, priKey, crtKey string) error {
	cfg, err := config.ReadConfig()
	if err != nil {
		panic(err)
	}
	if sslType == config.SSLTypeAutoHTTP || sslType == config.SSLTypeUser || sslType == config.SSLTypeAutoDNS {
		cfg.SSLType = sslType
	} else {
		return errors.New("SSL Type Error!")
	}

	if cfg.SSLType == config.SSLTypeUser {
		cfg.SSLPrivateKeyPath = priKey
		cfg.SSLPublicKeyPath = crtKey
		// 手动设置证书的情况下后台地址默认不启用https
		cfg.HttpsEnabled = 2
	}

	err = config.WriteConfig(cfg)
	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

func certificateRenewCertificate(privateKey *ecdsa.PrivateKey, cfg *config.Config) error {

	myUser := CertificateMyUser{
		Email: "SSLMaster@" + cfg.Domain,
		key:   privateKey,
	}

	conf := lego.NewConfig(&myUser)
	conf.UserAgent = "PMail"
	conf.Certificate.KeyType = certcrypto.RSA2048

	// A client facilitates communication with the CA server.
	client, err := lego.NewClient(conf)
	if err != nil {
		return errors.Wrap(err)
	}

	if cfg.SSLType == config.SSLTypeAutoHTTP {
		err = client.Challenge.SetHTTP01Provider(CertificateGetHttpChallengeInstance())
		if err != nil {
			return errors.Wrap(err)
		}
	} else if cfg.SSLType == config.SSLTypeAutoDNS {
		err = client.Challenge.SetDNS01Provider(CertificateGetDnsChallengeInstance(), dns01.AddDNSTimeout(60*time.Minute))
		if err != nil {
			return errors.Wrap(err)
		}

		log.Errorf("Please Set DNS Record/请将以下内容添加到DNS记录中:\n")
		for _, item := range CertificateGetDnsChallengeInstance().GetDNSSettings(nil) {
			log.Errorf("Type:%s\tHost:%s\tValue:%s\n", item.Type, item.Host, item.Value)
		}

	}

	var reg *registration.Resource

	reg, err = client.Registration.ResolveAccountByKey()
	if err != nil {
		return errors.Wrap(err)
	}

	myUser.Registration = reg

	domains := []string{cfg.WebDomain}
	for _, domain := range cfg.Domains {
		domains = append(domains, "smtp."+domain)
		domains = append(domains, "pop."+domain)
		domains = append(domains, "imap."+domain)
	}

	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	log.Infof("wait ssl renew")
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		panic(err)
	}
	err = os.WriteFile("./config/ssl/private.key", certificates.PrivateKey, 0666)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("./config/ssl/public.crt", certificates.Certificate, 0666)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("./config/ssl/issuerCert.crt", certificates.IssuerCertificate, 0666)
	if err != nil {
		panic(err)
	}

	return nil
}

func certificateGenerateCertificate(privateKey *ecdsa.PrivateKey, cfg *config.Config, newAccount bool) error {

	myUser := CertificateMyUser{
		Email: "i@" + cfg.Domain,
		key:   privateKey,
	}

	conf := lego.NewConfig(&myUser)
	conf.UserAgent = "PMail"
	conf.Certificate.KeyType = certcrypto.RSA2048

	// A client facilitates communication with the CA server.
	client, err := lego.NewClient(conf)
	if err != nil {
		return errors.Wrap(err)
	}

	if cfg.SSLType == config.SSLTypeAutoHTTP {
		err = client.Challenge.SetHTTP01Provider(CertificateGetHttpChallengeInstance())
		if err != nil {
			return errors.Wrap(err)
		}
	} else if cfg.SSLType == config.SSLTypeAutoDNS {
		err = client.Challenge.SetDNS01Provider(CertificateGetDnsChallengeInstance(), dns01.AddDNSTimeout(60*time.Minute))
		if err != nil {
			return errors.Wrap(err)
		}
	}

	var reg *registration.Resource

	if newAccount {
		reg, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return errors.Wrap(err)
		}
	} else {
		reg, err = client.Registration.ResolveAccountByKey()
		if err != nil {
			return errors.Wrap(err)
		}
	}

	myUser.Registration = reg

	domains := []string{cfg.WebDomain}
	for _, domain := range cfg.Domains {
		domains = append(domains, "smtp."+domain)
		domains = append(domains, "pop."+domain)
		domains = append(domains, "imap."+domain)
	}

	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	as := async.New(nil)

	as.Process(func(params any) {
		log.Infof("wait ssl")
		certificates, err := client.Certificate.Obtain(request)
		if err != nil {
			panic(err)
		}
		log.Infof("证书校验通过！")
		err = os.WriteFile("./config/ssl/private.key", certificates.PrivateKey, 0666)
		if err != nil {
			panic(err)
		}

		err = os.WriteFile("./config/ssl/public.crt", certificates.Certificate, 0666)
		if err != nil {
			panic(err)
		}

		err = os.WriteFile("./config/ssl/issuerCert.crt", certificates.IssuerCertificate, 0666)
		if err != nil {
			panic(err)
		}
		CertificateSetupFinished()

	}, nil)

	return nil
}

func CertificateGenSSL(update bool) error {

	cfg, err := config.ReadConfig()
	if err != nil {
		panic(err)
	}

	if !update {
		privateFile, errpi := os.ReadFile(cfg.SSLPrivateKeyPath)
		public, errpu := os.ReadFile(cfg.SSLPublicKeyPath)
		// 当前存在证书数据，就不生成了
		if errpi == nil && errpu == nil && len(privateFile) > 0 && len(public) > 0 {
			return nil
		}
	}

	privateKey, newAccount := config.ReadPrivateKey()

	if !update {
		return certificateGenerateCertificate(privateKey, cfg, newAccount)
	}

	return certificateRenewCertificate(privateKey, cfg)
}

// CheckSSLCrtInfo 返回证书过期剩余天数
func CertificateCheckSSLCrtInfo() (int, time.Time, bool, error) {

	cfg, err := config.ReadConfig()
	if err != nil {
		panic(err)
	}
	// load cert and key by tls.LoadX509KeyPair
	tlsCert, err := tls.LoadX509KeyPair(cfg.SSLPublicKeyPath, cfg.SSLPrivateKeyPath)
	if err != nil {
		return -1, time.Now(), true, errors.Wrap(err)
	}

	cert, err := x509.ParseCertificate(tlsCert.Certificate[0])

	if err != nil {
		return -1, time.Now(), true, errors.Wrap(err)
	}

	nameMatchFail := true
	for _, name := range cert.DNSNames {
		if strings.Contains(name, "imap") {
			nameMatchFail = false
			break
		}
	}

	// 检查过期时间
	hours := cert.NotAfter.Sub(time.Now()).Hours()

	if hours <= 0 {
		return -1, time.Now(), nameMatchFail, errors.New("Certificate has expired")
	}

	return cast.ToInt(hours / 24), cert.NotAfter, nameMatchFail, nil
}

func CertificateUpdate(needRestart bool) {
	if config.Instance != nil && config.Instance.IsInit && (config.Instance.SSLType == config.SSLTypeAutoHTTP || config.Instance.SSLType == config.SSLTypeAutoDNS) {
		days, _, nameMatchFail, err := CertificateCheckSSLCrtInfo()

		if days < 30 || err != nil || nameMatchFail {
			if err != nil {
				log.Errorf("SSL Check Error, Update SSL Certificate. Error Info :%+v", err)
			} else {
				log.Infof("SSL certificate remaining time is only %d days, renew SSL certificate.", days)
			}
			err = CertificateGenSSL(true)
			if err != nil {
				log.Errorf("SSL Update Error! %+v", err)
			}
			if needRestart {
				CertificateRestart <- // 更新完证书，重启服务
				true
			}
		} else {
			log.Debugf("SSL Check.")
		}
	}

}
