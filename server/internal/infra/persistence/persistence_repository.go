package persistence

import "xorm.io/xorm"

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
