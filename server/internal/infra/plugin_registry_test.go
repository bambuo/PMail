package infra

import (
	"sync"
	"testing"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/hooks/contract"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
)

type pluginFakeHook struct {
	name string
}

func (f *pluginFakeHook) SendBefore(ctx *context.Context, email *parsemail.Email) {}
func (f *pluginFakeHook) SendAfter(ctx *context.Context, email *parsemail.Email, err map[string]error) {
}
func (f *pluginFakeHook) ReceiveParseBefore(ctx *context.Context, email *[]byte)         {}
func (f *pluginFakeHook) ReceiveParseAfter(ctx *context.Context, email *parsemail.Email) {}
func (f *pluginFakeHook) ReceiveSaveAfter(ctx *context.Context, email *parsemail.Email, ue []*models.UserEmail) {
}
func (f *pluginFakeHook) GetName(ctx *context.Context) string { return f.name }
func (f *pluginFakeHook) SettingsHtml(ctx *context.Context, url string, requestData string) string {
	return ""
}

func pluginResetHookList() {
	PluginHookList = map[string]contract.EmailHook{}
}

// TestRegisterAndGetHook 验证注册与按键读取使用同一个键；插件停用时必须用注册键移除。
func TestRegisterAndGetHook(t *testing.T) {
	pluginResetHookList()

	registeredKey := "spam_block"
	PluginRegisterHook(registeredKey, &pluginFakeHook{name: registeredKey})
	if _, ok := PluginGetHook(registeredKey); !ok {
		t.Fatalf("GetHook(%q) = not found, want found", registeredKey)
	}
	if _, ok := PluginGetHook("other"); ok {
		t.Fatalf("GetHook(%q) = found, want not found", "other")
	}
	PluginRemoveHook(registeredKey)
	if _, ok := PluginGetHook(registeredKey); ok {
		t.Fatalf("GetHook(%q) after RemoveHook = found, want not found", registeredKey)
	}
}

// TestAllHooksSnapshot 验证 AllHooks 返回的是当前快照。
func TestAllHooksSnapshot(t *testing.T) {
	pluginResetHookList()

	PluginRegisterHook("A", &pluginFakeHook{name: "A"})
	PluginRegisterHook("B", &pluginFakeHook{name: "B"})

	snap := PluginAllHooks()
	if len(snap) != 2 {
		t.Fatalf("AllHooks() len = %d, want 2", len(snap))
	}
	PluginRemoveHook("A")
	if len(snap) != 2 {
		t.Fatalf("snapshot should be immutable, got %d", len(snap))
	}
	if got := len(PluginAllHooks()); got != 1 {
		t.Fatalf("AllHooks() after remove = %d, want 1", got)
	}
}

// TestHookNames 验证名称列表与注册键一致。
func TestHookNames(t *testing.T) {
	pluginResetHookList()
	PluginRegisterHook("spam_block", &pluginFakeHook{name: "spam_block"})
	PluginRegisterHook("wechat_push", &pluginFakeHook{name: "wechat_push"})

	names := PluginHookNames()
	if len(names) != 2 {
		t.Fatalf("HookNames() len = %d, want 2", len(names))
	}
	seen := map[string]bool{}
	for _, n := range names {
		seen[n] = true
	}
	if !seen["spam_block"] || !seen["wechat_push"] {
		t.Fatalf("HookNames() = %v, want contain spam_block and wechat_push", names)
	}
}

// TestConcurrentAccess 在 -race 下验证并发读写不会触发数据竞争。
func TestConcurrentAccess(t *testing.T) {
	pluginResetHookList()
	const plugins = 50
	for i := 0; i < plugins; i++ {
		key := "hook" + string(rune('A'+i%26))
		PluginRegisterHook(key, &pluginFakeHook{name: key})
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				for _, h := range PluginAllHooks() {
					if h == nil {
						continue
					}
					_ = h.GetName(nil)
				}
			}
		}()
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				key := "hook" + string(rune('A'+(i+j)%26))
				PluginRemoveHook(key)
				PluginRegisterHook(key, &pluginFakeHook{name: key})
			}
		}(i)
	}
	wg.Wait()
}
