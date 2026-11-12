package app

import (
	"fmt"
	"strings"

	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/errors"
	"github.com/Jinnrry/pmail/pkg/ip"
	"github.com/Jinnrry/pmail/pkg/password"
)

// SetupService 初始化引导用例：数据库、管理员、域名与证书完成标记。
type SetupService struct {
	users      UserPersistence
	initialize func(string) error
	dkim       func() string
}

func NewSetupService(users UserPersistence, initialize func(string) error, dkim func() string) *SetupService {
	return &SetupService{users: users, initialize: initialize, dkim: dkim}
}

// SetupSvc 初始化引导服务实例，由 bootstrap.BindDependencies 装配
var SetupSvc *SetupService

func (s *SetupService) DatabaseSettings(ctx *context.Context) (string, string, error) {
	configData, err := config.ReadConfig()
	if err != nil {
		return "", "", errors.Wrap(err)
	}

	if configData.DbType == "" && configData.DbDSN == "" {
		return config.DBTypeSQLite, config.ROOT_PATH + "./config/pmail.db", nil
	}

	return configData.DbType, configData.DbDSN, nil
}

func (s *SetupService) AdminPassword(ctx *context.Context) (string, error) {

	users := []*models.User{}
	err := s.users.ListSetupAccounts(&users)

	if err != nil {
		return "", errors.Wrap(err)
	}

	if len(users) > 0 {
		return users[0].Account, nil
	}

	return "", nil
}

func (s *SetupService) SetAdminPassword(ctx *context.Context, account, pwd string) error {
	encodePwd := password.Hash(pwd)
	var user models.User = models.User{
		Account:  account,
		Name:     "admin",
		Password: encodePwd,
		IsAdmin:  1,
	}

	_, err := s.users.InsertAdministrator(&user)

	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

func (s *SetupService) SetDatabaseSettings(ctx *context.Context, dbType, dbDSN string) error {
	configData, err := config.ReadConfig()
	if err != nil {
		return errors.Wrap(err)
	}

	if !array.InArray(dbType, config.DBTypes) {
		return errors.New("dbtype error")
	}

	if dbDSN == "" {
		return errors.New("DSN error")
	}

	configData.DbType = dbType
	configData.DbDSN = dbDSN

	err = config.WriteConfig(configData)
	if err != nil {
		return errors.Wrap(err)
	}
	config.Init()
	// 检查数据库是否能正确连接
	err = s.initialize("")
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}

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

func (s *SetupService) DomainSettings() (string, string, []string, error) {
	configData, err := config.ReadConfig()
	if err != nil {
		return "", "", []string{}, errors.Wrap(err)
	}

	return configData.Domain, configData.WebDomain, array.Difference(configData.Domains, []string{configData.Domain}), nil
}

func (s *SetupService) SetDomainSettings(smtpDomain, webDomain, multiDomains string) error {
	configData, err := config.ReadConfig()
	if err != nil {
		return errors.Wrap(err)
	}

	if smtpDomain == "" {
		return errors.New("domain must not empty!")
	}

	if webDomain == "" {
		return errors.New("web domain must not empty!")
	}

	configData.Domains = []string{}

	if multiDomains != "" {
		domains := strings.Split(multiDomains, ",")
		configData.Domains = domains
	}

	if !array.InArray(smtpDomain, configData.Domains) {
		configData.Domains = append(configData.Domains, smtpDomain)
	}

	configData.Domain = smtpDomain
	configData.WebDomain = webDomain

	// 检查域名是否指向本机 todo

	err = config.WriteConfig(configData)
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}

// Finish 标记初始化完成
func (s *SetupService) Finish() error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return errors.Wrap(err)
	}
	cfg.IsInit = true

	err = config.WriteConfig(cfg)
	if err != nil {
		return errors.Wrap(err)
	}
	LifecycleInitChan <- // 初始化完成
	true
	return nil
}
