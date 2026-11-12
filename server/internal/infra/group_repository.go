package infra

import (
	"fmt"

	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/errors"
)

func (r PersistenceRepository) RemoveGroups(ctx *context.Context, allGroupIds []int) (bool, error) {
	// 开启一个事务
	trans := Database.NewSession()

	res, err := trans.Exec(DatabaseWithContext(ctx, fmt.Sprintf("delete from `group` where id in (%s) and user_id =?", array.Join(allGroupIds, ","))), ctx.UserID)
	if err != nil {
		trans.Rollback()
		return false, errors.Wrap(err)
	}
	num, err := res.RowsAffected()
	if err != nil {
		trans.Rollback()
		return false, errors.Wrap(err)
	}

	_, err = trans.Exec(DatabaseWithContext(ctx, fmt.Sprintf("update user_email set group_id=0 where group_id in (%s)", array.Join(allGroupIds, ","))))
	if err != nil {
		trans.Rollback()
		return false, errors.Wrap(err)
	}

	trans.Commit()

	return num > 0, nil
}
