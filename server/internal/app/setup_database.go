package app

import (
	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/errors"
	"github.com/Jinnrry/pmail/pkg/password"
)

func SetupGetDatabaseSettings(ctx *context.Context) (string, string, error) {
	configData, err := config.ReadConfig()
	if err != nil {
		return "", "", errors.Wrap(err)
	}

	if configData.DbType == "" && configData.DbDSN == "" {
		return config.DBTypeSQLite, config.ROOT_PATH + "./config/pmail.db", nil
	}

	return configData.DbType, configData.DbDSN, nil
}

func SetupGetAdminPassword(ctx *context.Context) (string, error) {

	users := []*models.User{}
	err := Persistence.ListSetupAccounts(&users)

	if err != nil {
		return "", errors.Wrap(err)
	}

	if len(users) > 0 {
		return users[0].Account, nil
	}

	return "", nil
}

func SetupSetAdminPassword(ctx *context.Context, account, pwd string) error {
	encodePwd := password.Encode(pwd)
	var user models.User = models.User{
		Account:  account,
		Name:     "admin",
		Password: encodePwd,
		IsAdmin:  1,
	}

	_, err := Persistence.InsertAdministrator(&user)

	if err != nil {
		return errors.Wrap(err)
	}

	return nil
}

func SetupSetDatabaseSettings(ctx *context.Context, dbType, dbDSN string) error {
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
	err = InitializeDatabase("")
	if err != nil {
		return errors.Wrap(err)
	}
	return nil
}
