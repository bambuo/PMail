//go:build wasip1

// Debug WASM 插件：记录各钩子的请求大小；ReceiveParseAfter 强制把邮件状态置 5。
// 仅用于调试，不要部署到生产。
package main

import (
	"encoding/json"
	"strconv"

	"github.com/Jinnrry/pmail/hooks/wasm_plugins/internal/pluginabi"
)

const (
	hookSendBefore         = 1
	hookSendAfter          = 2
	hookReceiveParseBefore = 3
	hookReceiveParseAfter  = 4
	hookReceiveSaveAfter   = 5
	hookGetName            = 6
	hookSettingsHTML       = 7
)

//go:wasmexport pmail_alloc
func pmailAlloc(size uint32) uint32 { return pluginabi.Alloc(size) }

//go:wasmexport pmail_free
func pmailFree(ptr, size uint32) { pluginabi.Free(ptr, size) }

//go:wasmexport pmail_hook
func pmailHook(hookID, reqPtr, reqLen uint32) uint32 {
	switch hookID {
	case hookGetName:
		pluginabi.Emit([]byte(`{"name":"debug"}`))
	case hookSettingsHTML:
		body, _ := json.Marshal(map[string]string{"html": `<div style="line-height:1.6">
<p>Debug 插件会把 <code>ReceiveParseAfter</code> 的邮件状态强制置 <code>5</code>。</p>
<p>仅用于调试，请勿部署到生产环境。</p>
</div>`})
		pluginabi.Emit(body)
	case hookReceiveParseAfter:
		pluginabi.Log(1, "debug ReceiveParseAfter request_bytes="+strconv.Itoa(int(reqLen)))
		pluginabi.Emit([]byte(`{"email":{"Status":5}}`))
	case hookSendBefore, hookSendAfter, hookReceiveParseBefore, hookReceiveSaveAfter:
		pluginabi.Log(1, "debug hook="+strconv.Itoa(int(hookID))+" request_bytes="+strconv.Itoa(int(reqLen)))
	}
	return 0
}

func main() {}
