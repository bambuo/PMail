package infra

import (
	"database/sql"
	stderrors "errors"
	"fmt"

	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"xorm.io/builder"
	"xorm.io/xorm"
)

func PersistDeliveryResult(ctx *context.Context, emailID int, status int8, errorText any) error {
	return WithDatabaseTransaction(Database, func(session *xorm.Session) error {
		result, err := session.Exec(DatabaseWithContext(ctx, `update email
			set status=?, error=?, update_time=CURRENT_TIMESTAMP
			where id=?`), status, errorText, emailID)
		if err != nil {
			return err
		}
		if err = smtpRequireOneAffected(result, "update email delivery result"); err != nil {
			return err
		}

		result, err = session.Exec(DatabaseWithContext(ctx, `update user_email
			set status=? where email_id=?`), status, emailID)
		if err != nil {
			return err
		}
		return smtpRequireOneAffected(result, "update user_email delivery result")
	})
}

func smtpRequireOneAffected(result sql.Result, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: read affected rows: %w", operation, err)
	}
	if affected != 1 {
		return fmt.Errorf("%s: affected %d rows, want 1", operation, affected)
	}
	return nil
}

func PersistInitialAudit(ctx *context.Context, modelEmail models.Email, userIDs []int) (*models.Email, error) {
	var saved models.Email
	err := WithDatabaseTransaction(Database, func(session *xorm.Session) error {
		candidate := modelEmail
		candidate.Id = 0
		if _, err := session.Insert(&candidate); err != nil {
			return err
		}
		if candidate.Id <= 0 {
			return stderrors.New("email insert returned no id")
		}
		for _, userID := range userIDs {
			relation := models.UserEmail{EmailID: candidate.Id, UserID: userID, Status: candidate.Status}
			if _, err := session.Insert(&relation); err != nil {
				return err
			}
		}
		saved = candidate
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func FindRecipientAccounts(accounts []string, users *[]*models.User) error {
	where, params, err := builder.ToSQL(builder.In("LOWER(account)", accounts))
	if err != nil {
		return err
	}
	return Database.Table(&models.User{}).Where(where, params...).Find(users)
}
