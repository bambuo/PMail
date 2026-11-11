package infra

import (
	"github.com/Jinnrry/pmail/config"
	"github.com/alexedwards/scs/mysqlstore"
	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"

	"time"
)

var SessionInstance *scs.SessionManager

func SessionInit() {
	SessionInstance = scs.New()
	SessionInstance.Lifetime = 7 * 24 * time.Hour
	// 使用db存储session数据，目前为了架构简单，
	// 暂不引入redis存储，如果日后性能存在瓶颈，可以将session迁移到redis

	switch config.Instance.DbType {
	case config.DBTypeMySQL:
		SessionInstance.Store = mysqlstore.New(Database.DB().DB)
	case config.DBTypeSQLite:
		SessionInstance.Store = sqlite3store.New(Database.DB().DB)
	case config.DBTypePostgres:
		SessionInstance.Store = postgresstore.New(Database.DB().DB)
	default:
		panic("Unsupported database type: " + config.Instance.DbType)
	}

}
