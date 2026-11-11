package app

import (
	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/pkg/errors"
)

// Finish 标记初始化完成
func SetupFinish() error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return errors.Wrap(err)
	}
	cfg.IsInit = true

	err = config.WriteConfig(cfg)
	if err != nil {
		return errors.Wrap(err)
	}
	LifecycleInitChan <- // 初始化完成
	true
	return nil
}
