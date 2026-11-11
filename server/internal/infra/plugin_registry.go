package infra

import (
	"sync"

	"github.com/Jinnrry/pmail/hooks/contract"
)

// PluginHookList 保存已启用的插件执行器；当前由 WASM 插件管理器维护。
var PluginHookList = map[string]contract.EmailHook{}

// pluginHookListMu 保护 HookList。插件启停与收发邮件会并发读写。
var pluginHookListMu sync.RWMutex

// PluginAllHooks 返回当前已注册插件的快照，供收发邮件时并发安全地遍历。
func PluginAllHooks() []contract.EmailHook {
	pluginHookListMu.RLock()
	defer pluginHookListMu.RUnlock()
	list := make([]contract.EmailHook, 0, len(PluginHookList))
	for _, h := range PluginHookList {
		list = append(list, h)
	}
	return list
}

// PluginHookNames 返回已注册插件的名称列表。
func PluginHookNames() []string {
	pluginHookListMu.RLock()
	defer pluginHookListMu.RUnlock()
	names := make([]string, 0, len(PluginHookList))
	for name := range PluginHookList {
		names = append(names, name)
	}
	return names
}

// PluginGetHook 返回指定名称的插件。
func PluginGetHook(name string) (contract.EmailHook, bool) {
	pluginHookListMu.RLock()
	defer pluginHookListMu.RUnlock()
	h, ok := PluginHookList[name]
	return h, ok
}

// PluginRegisterHook 注册一个插件。
func PluginRegisterHook(name string, h contract.EmailHook) {
	pluginHookListMu.Lock()
	defer pluginHookListMu.Unlock()
	PluginHookList[name] = h
}

// PluginRemoveHook 移除指定名称的插件。
func PluginRemoveHook(name string) {
	pluginHookListMu.Lock()
	defer pluginHookListMu.Unlock()
	delete(PluginHookList, name)
}
