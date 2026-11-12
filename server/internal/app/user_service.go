package app

import (
	"fmt"

	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/password"
)

// UserService 用户认证与口令用例，绑定用户持久化端口（依赖注入，替代包级全局直连）
type UserService struct {
	users UserPersistence
}

func NewUserService(users UserPersistence) *UserService {
	return &UserService{users: users}
}

// UserSvc 用户认证用例服务实例，由 bootstrap.BindDependencies 装配
var UserSvc *UserService

// Authenticate 按账号校验口令，成功返回用户信息。
// 历史双重 MD5 哈希在验证通过后透明升级为 bcrypt，存量用户无感。
func (s *UserService) Authenticate(account, plain string) (*models.User, error) {
	var user models.User
	if _, err := s.users.FindTokenAccount(account, &user); err != nil {
		return nil, err
	}
	if user.ID == 0 || !password.Verify(user.Password, plain) {
		return nil, fmt.Errorf("account or password error")
	}
	if password.NeedsUpgrade(user.Password) {
		if err := s.updatePasswordHash(user.ID, password.Hash(plain)); err != nil {
			return nil, err
		}
	}
	return &user, nil
}

// ModifyPassword 修改用户口令
func (s *UserService) ModifyPassword(userID int, plain string) error {
	return s.updatePasswordHash(userID, password.Hash(plain))
}

// CreateAdministrator 创建账号（管理员操作）
func (s *UserService) CreateAdministrator(name, account, plain string) (*models.User, error) {
	user := models.User{Name: name, Password: password.Hash(plain), Account: account}
	if _, err := s.users.InsertAdministrator(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers 分页列出账号
func (s *UserService) ListUsers(limit, offset int) ([]models.User, int64, error) {
	var users []models.User
	total, err := s.users.ListAccounts(limit, offset, &users)
	return users, total, err
}

// FindUserByID 按 ID 查找账号
func (s *UserService) FindUserByID(id int) (*models.User, error) {
	var user models.User
	if _, err := s.users.FindAccountByID(id, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// FindUserByName 按账号名查找账号
func (s *UserService) FindUserByName(account string) (*models.User, error) {
	var user models.User
	if _, err := s.users.FindAccountByName(account, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUser 更新账号信息；newPlain 非空时同步重置口令
func (s *UserService) UpdateUser(user *models.User, newPlain string) (int64, error) {
	if newPlain != "" {
		user.Password = password.Hash(newPlain)
	}
	return s.users.UpdateAccount(user.ID, user)
}

func (s *UserService) updatePasswordHash(userID int, hash string) error {
	_, err := s.users.SetAccountPassword(userID, hash)
	return err
}
