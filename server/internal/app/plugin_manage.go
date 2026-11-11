package app

import (
	"github.com/Jinnrry/pmail/dto/plugin"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

func GetPluginManageList() []plugin.Info {
	if PluginList == nil {
		return nil
	}
	return PluginList()
}

func ApprovePlugin(ctx *pcontext.Context, id string, hooks, capabilities []string) error {
	if PluginApprove == nil {
		return plugin.ErrDisabled
	}
	return PluginApprove(ctx, id, hooks, capabilities)
}

func EnablePlugin(ctx *pcontext.Context, id string) error {
	if PluginEnable == nil {
		return plugin.ErrDisabled
	}
	return PluginEnable(ctx, id)
}

func DisablePlugin(ctx *pcontext.Context, id string) error {
	if PluginDisable == nil {
		return plugin.ErrDisabled
	}
	return PluginDisable(ctx, id)
}

func RejectPlugin(ctx *pcontext.Context, id string) error {
	if PluginReject == nil {
		return plugin.ErrDisabled
	}
	return PluginReject(ctx, id)
}

func ReloadPlugin(ctx *pcontext.Context, id string) error {
	if PluginReload == nil {
		return plugin.ErrDisabled
	}
	return PluginReload(ctx, id)
}
