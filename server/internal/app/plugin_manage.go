package app

import (
	"github.com/Jinnrry/pmail/dto/plugin"
	"github.com/Jinnrry/pmail/hooks/contract"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

// PluginPorts WASM 插件管理能力，由 bootstrap 绑定到 infra 实现。
type PluginPorts struct {
	List      func() []plugin.Info
	Approve   func(ctx *pcontext.Context, id string, hooks, capabilities []string) error
	Enable    func(ctx *pcontext.Context, id string) error
	Disable   func(ctx *pcontext.Context, id string) error
	Reject    func(ctx *pcontext.Context, id string) error
	Reload    func(ctx *pcontext.Context, id string) error
	Install   func([]byte) (plugin.Info, error)
	HookNames func() []string
	GetHook   func(string) (contract.EmailHook, bool)
}

// PluginService 插件用例：列出、审批、启停、安装与钩子查询。
type PluginService struct {
	ports PluginPorts
}

func NewPluginService(ports PluginPorts) *PluginService {
	return &PluginService{ports: ports}
}

// PluginSvc 插件服务实例，由 bootstrap.BindDependencies 装配
var PluginSvc *PluginService

func (s *PluginService) List() []plugin.Info {
	if s.ports.List == nil {
		return nil
	}
	return s.ports.List()
}

func (s *PluginService) Approve(ctx *pcontext.Context, id string, hooks, capabilities []string) error {
	if s.ports.Approve == nil {
		return plugin.ErrDisabled
	}
	return s.ports.Approve(ctx, id, hooks, capabilities)
}

func (s *PluginService) Enable(ctx *pcontext.Context, id string) error {
	if s.ports.Enable == nil {
		return plugin.ErrDisabled
	}
	return s.ports.Enable(ctx, id)
}

func (s *PluginService) Disable(ctx *pcontext.Context, id string) error {
	if s.ports.Disable == nil {
		return plugin.ErrDisabled
	}
	return s.ports.Disable(ctx, id)
}

func (s *PluginService) Reject(ctx *pcontext.Context, id string) error {
	if s.ports.Reject == nil {
		return plugin.ErrDisabled
	}
	return s.ports.Reject(ctx, id)
}

func (s *PluginService) Reload(ctx *pcontext.Context, id string) error {
	if s.ports.Reload == nil {
		return plugin.ErrDisabled
	}
	return s.ports.Reload(ctx, id)
}

func (s *PluginService) Install(data []byte) (plugin.Info, error) {
	if s.ports.Install == nil {
		return plugin.Info{}, plugin.ErrDisabled
	}
	return s.ports.Install(data)
}

func (s *PluginService) HookNames() []string {
	if s.ports.HookNames == nil {
		return nil
	}
	return s.ports.HookNames()
}

func (s *PluginService) GetHook(name string) (contract.EmailHook, bool) {
	if s.ports.GetHook == nil {
		return nil, false
	}
	return s.ports.GetHook(name)
}
