package persistence

import (
	stderrors "errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/errors"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"modernc.org/sqlite"
	"xorm.io/xorm"
)

func InitDatabase(version string) (*xorm.Engine, error) {
	dsn := config.Instance.DbDSN
	var engine *xorm.Engine
	var err error
	if config.Instance.DbType == config.DBTypeSQLite {
		dsn, err = databaseSqliteDSNWithReliabilityOptions(dsn)
		if err != nil {
			return nil, errors.Wrap(err)
		}
	}

	switch config.Instance.DbType {
	case "mysql":
		engine, err = xorm.NewEngine("mysql", dsn)
	case "sqlite":
		engine, err = xorm.NewEngine("sqlite", dsn)
	case "postgres":
		engine, err = xorm.NewEngine("postgres", dsn)
	default:
		return nil, errors.New("Database Type Error!")
	}
	if err != nil {
		log.Errorf("DB init Error! %s", err.Error())
		return nil, errors.Wrap(err)
	}
	if config.Instance.DbType == config.DBTypeSQLite {
		engine.SetMaxOpenConns(1)
		engine.SetMaxIdleConns(1)
	} else {
		engine.SetMaxOpenConns(100)
		engine.SetMaxIdleConns(10)
	}

	engine.SetConnMaxLifetime(30 * time.Minute)
	engine.ShowSQL(config.Instance.LogLevel == "debug")
	// 同步表结构
	databaseSyncTables(engine)

	// 更新历史数据
	databaseFixHistoryData(engine)

	// 在数据库中记录程序版本
	var v models.Version
	_, err = engine.Get(&v)
	if err != nil {
		panic(err)
	}

	if version != "" && v.Info != version && version != "test" {
		v.Info = version
		if v.Id == 0 {
			engine.Insert(&v)
		} else {
			engine.Update(&v)
		}
	}

	//if config.Instance.LogLevel == "debug" {
	//	Instance.ShowSQL(true)
	//}

	return engine, nil
}

func DatabaseWithContext(ctx *context.Context, sql string) string {
	if ctx != nil {
		logId := ctx.GetValue(context.LogID)
		return fmt.Sprintf("/* %s */ %s", logId, sql)
	}
	return sql
}

func databaseSyncTables(engine *xorm.Engine) {
	err := engine.Sync2(&models.User{})
	if err != nil {
		panic(err)
	}
	err = engine.Sync2(&models.Email{})
	if err != nil {
		panic(err)
	}
	err = engine.Sync2(&models.Group{})
	if err != nil {
		panic(err)
	}
	err = engine.Sync2(&models.Rule{})
	if err != nil {
		panic(err)
	}
	err = engine.Sync2(&models.Sessions{})
	if err != nil {
		panic(err)
	}
	err = engine.Sync2(&models.UserEmail{})
	if err != nil {
		panic(err)
	}
	err = engine.Sync2(&models.Version{})
	if err != nil {
		panic(err)
	}
}

func databaseFixHistoryData(engine *xorm.Engine) {
	var ueNum int
	_, err := engine.Table(&models.UserEmail{}).Select("count(1)").Get(&ueNum)
	if err != nil {
		panic(err)
	}
	if ueNum > 0 {
		return
	}

	// 只有一个管理员用户
	var user []models.User
	err = engine.Table(&models.User{}).OrderBy("id asc").Find(&user)
	if err != nil {
		panic(err)
	}

	// 只有一个账号，且不是管理员账号，将账号提权为管理员
	if len(user) == 1 && user[0].IsAdmin == 0 {
		u := user[0]
		u.IsAdmin = 1
		_, err = engine.Update(&u)
		if err != nil {
			panic(err)
		}
	}

	if len(user) != 1 {
		return
	}

	// 以前有邮件
	var emails []*models.Email
	err = engine.Table(&models.Email{}).Select("id,status").OrderBy("id asc").Find(&emails)
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
		_, err = engine.Insert(&ue)
		if err != nil {
			log.Errorf("SQL Error: %v", err)
		}
	}
	log.Infof("Sync History Data Finished. Num: %d", len(emails))

}

const (
	databaseSqliteBusyTimeoutMilliseconds = 1000
	databaseTransactionAttempts           = 3
	databaseTransactionInitialBackoff     = 50 * time.Millisecond
)

// Transaction runs fn atomically. SQLite BUSY and LOCKED failures retry the
// entire transaction, so fn must contain only work that is safe to repeat.
func WithDatabaseTransaction(engine *xorm.Engine, fn func(*xorm.Session) error) error {
	return databaseTransactionWithRetry(engine, databaseTransactionAttempts, databaseTransactionInitialBackoff, fn)
}

func databaseTransactionWithRetry(engine *xorm.Engine, attempts int, initialBackoff time.Duration, fn func(*xorm.Session) error) error {
	if attempts < 1 {
		attempts = 1
	}

	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		var retrySafe bool
		err, retrySafe = databaseRunTransactionAttempt(engine, fn)
		if err == nil {
			return nil
		}
		if !retrySafe || engine.DriverName() != "sqlite" || !DatabaseIsSQLiteBusyOrLocked(err) || attempt == attempts {
			return err
		}
		time.Sleep(initialBackoff << (attempt - 1))
	}
	return err
}

func databaseRunTransaction(engine *xorm.Engine, fn func(*xorm.Session) error) error {
	err, _ := databaseRunTransactionAttempt(engine, fn)
	return err
}

func databaseRunTransactionAttempt(engine *xorm.Engine, fn func(*xorm.Session) error) (error, bool) {
	session := engine.NewSession()
	defer session.Close()

	if err := session.Begin(); err != nil {
		return fmt.Errorf("begin transaction: %w", err), true
	}
	if err := fn(session); err != nil {
		if rollbackErr := session.Rollback(); rollbackErr != nil {
			return stderrors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr)), false
		}
		return err, true
	}
	if err := session.Commit(); err != nil {
		// modernc.org/sqlite v1.46.1 and newer roll back internally when COMMIT
		// fails, leaving the connection clean and the transaction safe to retry.
		return fmt.Errorf("commit transaction: %w", err), true
	}
	return nil, false
}

// IsSQLiteBusyOrLocked classifies SQLite errors by result code, including
// extended BUSY and LOCKED codes. Error message text is intentionally ignored.
func DatabaseIsSQLiteBusyOrLocked(err error) bool {
	var sqliteErr *sqlite.Error
	if !stderrors.As(err, &sqliteErr) {
		return false
	}
	primaryCode := sqliteErr.Code() & 0xff
	return primaryCode == 5 || primaryCode == 6
}

func databaseSqliteDSNWithReliabilityOptions(dsn string) (string, error) {
	dsnPrefix, rawQuery, hasQuery := strings.Cut(dsn, "?")
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", fmt.Errorf("parse SQLite DSN: %w", err)
	}

	pragmas := query["_pragma"]
	timeoutIndex := -1
	for index, pragma := range pragmas {
		if databaseSqlitePragmaName(pragma) == "busy_timeout" {
			if timeoutIndex >= 0 {
				return "", fmt.Errorf("parse SQLite DSN: multiple busy_timeout pragmas are ambiguous")
			}
			timeoutIndex = index
		}
	}
	if timeoutIndex >= 0 {
		requested, err := databaseSqliteBusyTimeoutValue(pragmas[timeoutIndex])
		if err != nil {
			return "", fmt.Errorf("parse SQLite busy_timeout pragma %q: %w", pragmas[timeoutIndex], err)
		}
		if requested <= databaseSqliteBusyTimeoutMilliseconds {
			return dsn, nil
		}

		log.Warnf(
			"SQLite busy_timeout requested %dms exceeds the %dms maximum; using %dms",
			requested,
			databaseSqliteBusyTimeoutMilliseconds,
			databaseSqliteBusyTimeoutMilliseconds,
		)
		pragmas[timeoutIndex] = fmt.Sprintf("busy_timeout(%d)", databaseSqliteBusyTimeoutMilliseconds)
		query["_pragma"] = pragmas
		return dsnPrefix + "?" + query.Encode(), nil
	}

	separator := "?"
	if hasQuery {
		separator = "&"
		if strings.HasSuffix(dsn, "?") || strings.HasSuffix(dsn, "&") {
			separator = ""
		}
	}
	option := url.Values{"_pragma": {fmt.Sprintf("busy_timeout(%d)", databaseSqliteBusyTimeoutMilliseconds)}}.Encode()
	return dsn + separator + option, nil
}

func databaseSqliteBusyTimeoutValue(pragma string) (int64, error) {
	const name = "busy_timeout"

	normalized := strings.TrimSpace(pragma)
	if len(normalized) < len(name) || !strings.EqualFold(normalized[:len(name)], name) {
		return 0, fmt.Errorf("not a busy_timeout pragma")
	}
	remainder := strings.TrimSpace(normalized[len(name):])

	var valueText string
	switch {
	case strings.HasPrefix(remainder, "("):
		if !strings.HasSuffix(remainder, ")") {
			return 0, fmt.Errorf("expected busy_timeout(<milliseconds>)")
		}
		valueText = strings.TrimSpace(remainder[1 : len(remainder)-1])
	case strings.HasPrefix(remainder, "="):
		valueText = strings.TrimSpace(remainder[1:])
	default:
		return 0, fmt.Errorf("expected busy_timeout(<milliseconds>) or busy_timeout = <milliseconds>")
	}
	if valueText == "" {
		return 0, fmt.Errorf("timeout value is empty")
	}

	timeout, err := strconv.ParseInt(valueText, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout value %q: %w", valueText, err)
	}
	return timeout, nil
}

func databaseSqlitePragmaName(pragma string) string {
	normalized := strings.ToLower(strings.TrimSpace(pragma))
	if end := strings.IndexFunc(normalized, func(character rune) bool {
		return character == '(' || character == '=' || unicode.IsSpace(character)
	}); end >= 0 {
		normalized = normalized[:end]
	}
	return normalized
}

// PersistenceRepository 是持久化适配器，所有查询使用注入的引擎。
type PersistenceRepository struct {
	engine *xorm.Engine
}

func New(engine *xorm.Engine) *PersistenceRepository {
	return &PersistenceRepository{engine: engine}
}

func (r *PersistenceRepository) SetEngine(engine *xorm.Engine) {
	r.engine = engine
}

func (r *PersistenceRepository) db() *xorm.Engine {
	return r.engine
}
