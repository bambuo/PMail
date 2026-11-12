package app

import (
	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/errors"
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
