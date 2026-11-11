package infra

import "github.com/Jinnrry/pmail/models"

func (PersistenceRepository) FindLocalForwardAccount(p0 string, p1 *models.User) (bool, error) {
	return Database.Table(&models.User{}).Where("LOWER(account)=LOWER(?) and disabled=0", p0).Get(p1)
}
func (PersistenceRepository) ListSetupAccounts(p0 *[]*models.User) error {
	return Database.Find(p0)
}
func (PersistenceRepository) InsertAdministrator(p0 *models.User) (int64, error) {
	return Database.Insert(p0)
}
func (PersistenceRepository) FindTokenAccount(p0 string, p1 *models.User) (bool, error) {
	return Database.Table("user").Where("account =? and disabled=0", p0).Get(p1)
}
func (PersistenceRepository) SetAccountPassword(p0 int, p1 string) (int64, error) {
	return Database.Table("user").Where("id=?", p0).Update(map[string]interface {
	}{"password": p1})
}
func (PersistenceRepository) ListAccounts(p0 int, p1 int, p2 *[]models.User) (int64, error) {
	return Database.Table(&models.User{}).Limit(p0, p1).FindAndCount(p2)
}
func (PersistenceRepository) FindAccountByID(p0 int, p1 *models.User) (bool, error) {
	return Database.Where("id=?", p0).Get(p1)
}
func (PersistenceRepository) FindAccountByName(p0 string, p1 *models.User) (bool, error) {
	return Database.Where("account=?", p0).Get(p1)
}
func (PersistenceRepository) UpdateAccount(p0 int, p1 *models.User) (int64, error) {
	return Database.ID(p0).Cols("name", "password", "disabled").Update(p1)
}
func (PersistenceRepository) FindAPOPAccount(p0 string, p1 *models.User) (bool, error) {
	return Database.Where("account =? and disabled = 0", p0).Get(p1)
}
