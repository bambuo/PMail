package wasmabi

const (
	ABI         = "pmail-hook@1"
	HostModule  = "env"
	ExportAlloc = "pmail_alloc"
	ExportFree  = "pmail_free"
	ExportHook  = "pmail_hook"
	ImportEmit  = "pmail_emit"
	ImportLog   = "pmail_log"
	ImportHTTP  = "pmail_http_post"
)

const (
	HookSendBefore uint32 = iota + 1
	HookSendAfter
	HookReceiveParseBefore
	HookReceiveParseAfter
	HookReceiveSaveAfter
	HookGetName
	HookSettingsHTML
)

var hookNames = map[uint32]string{
	HookSendBefore:         "SendBefore",
	HookSendAfter:          "SendAfter",
	HookReceiveParseBefore: "ReceiveParseBefore",
	HookReceiveParseAfter:  "ReceiveParseAfter",
	HookReceiveSaveAfter:   "ReceiveSaveAfter",
	HookGetName:            "GetName",
	HookSettingsHTML:       "SettingsHtml",
}

// HookName 返回钩子编号对应的名称，未知编号返回空字符串。
func HookName(id uint32) string {
	return hookNames[id]
}
