# WASM 内置插件

仓库内置插件现在都有 WASM 版本，共用同一套宿主 ABI（`pluginabi`）：

| 插件 | 目录 / ID | 作用 | 发布用 |
| --- | --- | --- | --- |
| 反垃圾 | `spam_block` | 调用 RETVec/TensorFlow Serving 模型，广告置 `status=5`，诈骗置 `3` | 是 |
| 微信推送 | `wechat_push` | 管理员收到新未读邮件时发微信模板消息，识别验证码 | 是 |
| 调试 | `debug` | 记录钩子请求大小；`ReceiveParseAfter` 强制状态 5 | 否，仅调试 |

旧进程插件机制与实现已移除，仓库只保留 WASM 实现。

## 构建

```bash
# 在仓库根目录
make wasm_plugins
```

等价于：

```bash
cd server/hooks/wasm_plugins
for p in spam_block wechat_push debug; do
  GOOS=wasip1 GOARCH=wasm CGO_ENABLED=0 \
    go build -buildmode=c-shared -o output/$p/plugin.wasm ./cmd/$p
  cp manifests/$p.json output/$p/manifest.json
done
```

`c-shared` 会生成 WASI reactor，导出 `_initialize`；主程序实例化后先调用它初始化 Go 运行时，再调用钩子。

## 部署

```text
plugins/wasm/
├── spam_block/
│   ├── manifest.json
│   └── plugin.wasm
├── wechat_push/
│   ├── manifest.json
│   └── plugin.wasm
└── debug/              # 仅调试环境
    ├── manifest.json
    └── plugin.wasm
```

1. 主配置设置 `"pluginWasmEnabled": 1`，重启 PMail。
2. 设置 → 插件设置 → 插件管理，逐个审批并启用。
3. 授权项：
   - `spam_block`：`ReceiveParseAfter` + `log`、`email.read`、`email.write`、`http`
   - `wechat_push`：`ReceiveSaveAfter`、`GetName`、`SettingsHtml` + `log`、`email.read`、`http`
   - `debug`：全部钩子 + `log`、`email.read`、`email.write`

## 配置

### spam_block

`manifest.json` 的 `config`：

| 字段 | 说明 |
| --- | --- |
| `api_url` | 模型 API 地址，需与 `http_allow` 主机一致 |
| `threshold` | 命中阈值，0–1，默认 0.20 |

模型服务与训练脚本见 [spam_block_model README](spam_block_model/README.md)。没有 TensorFlow 环境时可用本目录的 `mock_model.py`：

```bash
python3 server/hooks/wasm_plugins/mock_model.py
```

### wechat_push

`manifest.json` 的 `config`：

| 字段 | 说明 |
| --- | --- |
| `app_id` / `secret` | 微信公众号凭据 |
| `template_id` | 模板消息 ID |
| `user_id` | 接收推送的微信用户 |
| `web_url` | 消息点击跳转地址 |
| `admin_user_id` | 只推送该用户的新邮件，默认 1 |
| `api_base` | 默认 `https://api.weixin.qq.com` |

`http_allow` 必须包含 `api_base` 的主机。修改 manifest 后文件哈希变化，状态会回到待审批，需要重新授权。

### debug

无配置；**不要部署到生产**，否则所有收信状态会被改成 5。

## 安全边界

- 仅审批 `http` 后注册 `pmail_http_get` / `pmail_http_post`；目标主机必须精确命中 `http_allow`。
- 不跟随重定向；请求体 ≤1 MiB，响应 ≤`output_bytes`，HTTP 超时取 `async_timeout_ms`。
- 微信插件 token 缓存在 WASM 实例内，过期前 5 分钟刷新；不写磁盘、不打印 secret。
- Go 运行时内存占用较高，manifest 默认 128 MiB。
