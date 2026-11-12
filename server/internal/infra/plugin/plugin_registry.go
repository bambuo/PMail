package plugin

import (
	"sync"

	"github.com/Jinnrry/pmail/hooks/contract"
)

// HookList 保存已启用的插件执行器；当前由 WASM 插件管理器维护。
var HookList = map[string]contract.EmailHook{}

// pluginHookListMu 保护 HookList。插件启停与收发邮件会并发读写。
var pluginHookListMu sync.RWMutex

// AllHooks 返回当前已注册插件的快照，供收发邮件时并发安全地遍历。
func AllHooks() []contract.EmailHook {
	pluginHookListMu.RLock()
	defer pluginHookListMu.RUnlock()
	list := make([]contract.EmailHook, 0, len(HookList))
	for _, h := range HookList {
		list = append(list, h)
	}
	return list
}

// HookNames 返回已注册插件的名称列表。
func HookNames() []string {
	pluginHookListMu.RLock()
	defer pluginHookListMu.RUnlock()
	names := make([]string, 0, len(HookList))
	for name := range HookList {
		names = append(names, name)
	}
	return names
}

// GetHook 返回指定名称的插件。
func GetHook(name string) (contract.EmailHook, bool) {
	pluginHookListMu.RLock()
	defer pluginHookListMu.RUnlock()
	h, ok := HookList[name]
	return h, ok
}

// RegisterHook 注册一个插件。
func RegisterHook(name string, h contract.EmailHook) {
	pluginHookListMu.Lock()
	defer pluginHookListMu.Unlock()
	HookList[name] = h
}

// RemoveHook 移除指定名称的插件。
func RemoveHook(name string) {
	pluginHookListMu.Lock()
	defer pluginHookListMu.Unlock()
	delete(HookList, name)
}
