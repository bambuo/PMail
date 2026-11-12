package domain

import ()

type EmailDeletionRepository interface {
	MarkDeleted(userID int, emailID int64) error
	FindEmailByUID(userID, uid int) (emailID int, found bool, err error)
	RemoveEmailOwner(userID int, emailID int64) error
	RemoveEmailUID(userID, uid int) error
	CountEmailOwners(emailID int64) (int, error)
	RemoveEmail(emailID int64) error
}

type EmailStatRow struct {
	Total int64
	Size  int64
}
type GroupIDRow struct {
	Id int `db:"id"`
}
