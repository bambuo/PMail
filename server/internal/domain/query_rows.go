package domain

type EmailStatRow struct {
	Total int64
	Size  int64
}
type GroupIDRow struct {
	Id int `db:"id"`
}
