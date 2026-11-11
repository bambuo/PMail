package contract

import (
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
)

// EmailHook 是插件与邮件处理链路之间的唯一契约。
// 当前由 WASM 执行器实现；进程插件机制已移除。
type EmailHook interface {
	// SendBefore 邮件发送前的数据 同步执行
	SendBefore(ctx *context.Context, email *parsemail.Email)
	// SendAfter 邮件发送后的数据，err是每个收信服务器的错误信息 异步执行
	SendAfter(ctx *context.Context, email *parsemail.Email, err map[string]error)
	// ReceiveParseBefore 接收到邮件，解析之前的原始数据 同步执行
	ReceiveParseBefore(ctx *context.Context, email *[]byte)
	// ReceiveParseAfter 接收到邮件，解析之后的结构化数据 (收信规则前，写数据库前执行)  同步执行
	ReceiveParseAfter(ctx *context.Context, email *parsemail.Email)
	// ReceiveSaveAfter 邮件落库以后执行（收信规则后执行） 异步执行
	ReceiveSaveAfter(ctx *context.Context, email *parsemail.Email, ue []*models.UserEmail)
	// GetName 获取插件名称
	GetName(ctx *context.Context) string
	// SettingsHtml 插件页面
	SettingsHtml(ctx *context.Context, url string, requestData string) string
}
