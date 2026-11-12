package persistence

import (
	"github.com/Jinnrry/pmail/consts"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"xorm.io/xorm"
)

type EmailDeletionSession struct{ *xorm.Session }

func (r *PersistenceRepository) NewDeletionSession() app.EmailDeletionSession {
	return &EmailDeletionSession{r.db().NewSession()}
}
func (s *EmailDeletionSession) MarkDeleted(userID int, emailID int64) error {
	_, err := s.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Update(map[string]any{"status": consts.EmailStatusDel, "group_id": 0})
	return err
}
func (s *EmailDeletionSession) FindEmailByUID(userID, uid int) (int, bool, error) {
	var relation models.UserEmail
	found, err := s.Table("user_email").Where("id=? and user_id=?", uid, userID).Get(&relation)
	return relation.EmailID, found && relation.ID != 0, err
}
func (s *EmailDeletionSession) RemoveEmailOwner(userID int, emailID int64) error {
	_, err := s.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Delete(&models.UserEmail{})
	return err
}
func (s *EmailDeletionSession) RemoveEmailUID(userID, uid int) error {
	_, err := s.Table(&models.UserEmail{}).Where("id=? and user_id=?", uid, userID).Delete(&models.UserEmail{})
	return err
}
func (s *EmailDeletionSession) CountEmailOwners(emailID int64) (int, error) {
	var row struct {
		Num int `xorm:"num"`
	}
	_, err := s.Table(&models.UserEmail{}).Select("count(1) as num").Where("email_id=?", emailID).Get(&row)
	return row.Num, err
}
func (s *EmailDeletionSession) RemoveEmail(emailID int64) error {
	_, err := s.Table(&models.Email{}).Where("id=?", emailID).Delete(&models.Email{})
	return err
}
