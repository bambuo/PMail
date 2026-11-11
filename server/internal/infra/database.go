package infra

import (
	"fmt"
	"time"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/errors"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"xorm.io/xorm"
)

var Database *xorm.Engine

func InitDatabase(version string) error {
	dsn := config.Instance.DbDSN
	var err error
	if config.Instance.DbType == config.DBTypeSQLite {
		dsn, err = databaseSqliteDSNWithReliabilityOptions(dsn)
		if err != nil {
			return errors.Wrap(err)
		}
	}

	switch config.Instance.DbType {
	case "mysql":
		Database, err = xorm.NewEngine("mysql", dsn)
	case "sqlite":
		Database, err = xorm.NewEngine("sqlite", dsn)
	case "postgres":
		Database, err = xorm.NewEngine("postgres", dsn)
	default:
		return errors.New("Database Type Error!")
	}
	if err != nil {
		log.Errorf("DB init Error! %s", err.Error())
		return errors.Wrap(err)
	}
	if config.Instance.DbType == config.DBTypeSQLite {
		Database.SetMaxOpenConns(1)
		Database.SetMaxIdleConns(1)
	} else {
		Database.SetMaxOpenConns(100)
		Database.SetMaxIdleConns(10)
	}

	Database.SetConnMaxLifetime(30 * time.Minute)
	Database.ShowSQL(config.Instance.LogLevel == "debug")
	// 同步表结构
	databaseSyncTables()

	// 更新历史数据
	databaseFixHistoryData()

	// 在数据库中记录程序版本
	var v models.Version
	_, err = Database.Get(&v)
	if err != nil {
		panic(err)
	}

	if version != "" && v.Info != version && version != "test" {
		v.Info = version
		if v.Id == 0 {
			Database.Insert(&v)
		} else {
			Database.Update(&v)
		}
	}

	//if config.Instance.LogLevel == "debug" {
	//	Instance.ShowSQL(true)
	//}

	return nil
}

func DatabaseWithContext(ctx *context.Context, sql string) string {
	if ctx != nil {
		logId := ctx.GetValue(context.LogID)
		return fmt.Sprintf("/* %s */ %s", logId, sql)
	}
	return sql
}

func databaseSyncTables() {
	err := Database.Sync2(&models.User{})
	if err != nil {
		panic(err)
	}
	err = Database.Sync2(&models.Email{})
	if err != nil {
		panic(err)
	}
	err = Database.Sync2(&models.Group{})
	if err != nil {
		panic(err)
	}
	err = Database.Sync2(&models.Rule{})
	if err != nil {
		panic(err)
	}
	err = Database.Sync2(&models.Sessions{})
	if err != nil {
		panic(err)
	}
	err = Database.Sync2(&models.UserEmail{})
	if err != nil {
		panic(err)
	}
	err = Database.Sync2(&models.Version{})
	if err != nil {
		panic(err)
	}
}

func databaseFixHistoryData() {
	var ueNum int
	_, err := Database.Table(&models.UserEmail{}).Select("count(1)").Get(&ueNum)
	if err != nil {
		panic(err)
	}
	if ueNum > 0 {
		return
	}

	// 只有一个管理员用户
	var user []models.User
	err = Database.Table(&models.User{}).OrderBy("id asc").Find(&user)
	if err != nil {
		panic(err)
	}

	// 只有一个账号，且不是管理员账号，将账号提权为管理员
	if len(user) == 1 && user[0].IsAdmin == 0 {
		u := user[0]
		u.IsAdmin = 1
		_, err = Database.Update(&u)
		if err != nil {
			panic(err)
		}
	}

	if len(user) != 1 {
		return
	}

	// 以前有邮件
	var emails []*models.Email
	err = Database.Table(&models.Email{}).Select("id,status").OrderBy("id asc").Find(&emails)
	if err != nil {
		panic(err)
	}
	if len(emails) == 0 {
		return
	}

	log.Infof("Sync History Data！Please Wait！")

	// 把以前的邮件，全部分到管理员账号下面去
	for _, email := range emails {
		ue := models.UserEmail{
			UserID:  user[0].ID,
			EmailID: email.Id,
			Status:  email.Status,
		}
		_, err = Database.Insert(&ue)
		if err != nil {
			log.Errorf("SQL Error: %v", err)
		}
	}
	log.Infof("Sync History Data Finished. Num: %d", len(emails))

}
