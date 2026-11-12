package persistence

import "github.com/Jinnrry/pmail/models"

func (r *PersistenceRepository) FindLocalForwardAccount(account string, user *models.User) (bool, error) {
	return r.db().Table(&models.User{}).Where("LOWER(account)=LOWER(?) and disabled=0", account).Get(user)
}
func (r *PersistenceRepository) ListSetupAccounts(users *[]*models.User) error {
	return r.db().Find(users)
}
func (r *PersistenceRepository) InsertAdministrator(user *models.User) (int64, error) {
	return r.db().Insert(user)
}
func (r *PersistenceRepository) FindTokenAccount(account string, user *models.User) (bool, error) {
	return r.db().Table("user").Where("account =? and disabled=0", account).Get(user)
}
func (r *PersistenceRepository) SetAccountPassword(userID int, hash string) (int64, error) {
	return r.db().Table("user").Where("id=?", userID).Update(map[string]interface {
	}{"password": hash})
}
func (r *PersistenceRepository) ListAccounts(limit int, offset int, users *[]models.User) (int64, error) {
	return r.db().Table(&models.User{}).Limit(limit, offset).FindAndCount(users)
}
func (r *PersistenceRepository) FindAccountByID(id int, user *models.User) (bool, error) {
	return r.db().Where("id=?", id).Get(user)
}
func (r *PersistenceRepository) FindAccountByName(account string, user *models.User) (bool, error) {
	return r.db().Where("account=?", account).Get(user)
}
func (r *PersistenceRepository) UpdateAccount(id int, user *models.User) (int64, error) {
	return r.db().ID(id).Cols("name", "password", "disabled").Update(user)
}
func (r *PersistenceRepository) FindAPOPAccount(account string, user *models.User) (bool, error) {
	return r.db().Where("account =? and disabled = 0", account).Get(user)
}
