package infra

import "github.com/Jinnrry/pmail/models"

func (PersistenceRepository) FindLocalForwardAccount(account string, user *models.User) (bool, error) {
	return Database.Table(&models.User{}).Where("LOWER(account)=LOWER(?) and disabled=0", account).Get(user)
}
func (PersistenceRepository) ListSetupAccounts(users *[]*models.User) error {
	return Database.Find(users)
}
func (PersistenceRepository) InsertAdministrator(user *models.User) (int64, error) {
	return Database.Insert(user)
}
func (PersistenceRepository) FindTokenAccount(account string, user *models.User) (bool, error) {
	return Database.Table("user").Where("account =? and disabled=0", account).Get(user)
}
func (PersistenceRepository) SetAccountPassword(userID int, hash string) (int64, error) {
	return Database.Table("user").Where("id=?", userID).Update(map[string]interface {
	}{"password": hash})
}
func (PersistenceRepository) ListAccounts(limit int, offset int, users *[]models.User) (int64, error) {
	return Database.Table(&models.User{}).Limit(limit, offset).FindAndCount(users)
}
func (PersistenceRepository) FindAccountByID(id int, user *models.User) (bool, error) {
	return Database.Where("id=?", id).Get(user)
}
func (PersistenceRepository) FindAccountByName(account string, user *models.User) (bool, error) {
	return Database.Where("account=?", account).Get(user)
}
func (PersistenceRepository) UpdateAccount(id int, user *models.User) (int64, error) {
	return Database.ID(id).Cols("name", "password", "disabled").Update(user)
}
func (PersistenceRepository) FindAPOPAccount(account string, user *models.User) (bool, error) {
	return Database.Where("account =? and disabled = 0", account).Get(user)
}
