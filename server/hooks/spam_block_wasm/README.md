# SpamBlock WASM 插件

用 WASM 实现的 `spam_block`：收信解析后把「主题 + 正文」送到 RETVec / TensorFlow Serving 模型分类，命中后写 `email.Status`：

| 类别 | 条件 | 状态 |
| --- | --- | --- |
| 正常 | class 0 | 不改 |
| 广告 | class 1 且得分 > threshold | `5` |
| 诈骗 | class 2 且得分 > threshold | `3` |

与进程版 `server/hooks/spam_block` 的行为一致；模型服务与训练脚本复用后者的 [README](../spam_block/README.md)。

## 构建

```bash
cd server/hooks/spam_block_wasm
GOOS=wasip1 GOARCH=wasm CGO_ENABLED=0 go build -buildmode=c-shared -o plugin.wasm .
```

`c-shared` 会生成 WASI reactor，导出 `_initialize`；主程序在实例化后先调用它初始化 Go 运行时，再调用插件钩子。

## 部署

1. 把 `plugin.wasm` 与 `manifest.json` 放到运行目录：

   ```text
   plugins/wasm/spam_block/
   ├── manifest.json
   └── plugin.wasm
   ```

2. 在主配置中启用 WASM 插件能力：`"pluginWasmEnabled": 1`，重启 PMail。
3. 设置 → 扩展功能 → WASM 插件：审批并启用 `spam_block`，授予：
   - 钩子：`ReceiveParseAfter`
   - 能力：`log`、`email.read`、`email.write`、`http`

## manifest 配置

| 字段 | 说明 |
| --- | --- |
| `http_allow` | 允许访问的主机（精确匹配 `host` 或 `host:port`，不带 scheme/path）。默认 `127.0.0.1:8501` |
| `config.api_url` | 模型 API 地址，必须与 `http_allow` 的主机一致 |
| `config.threshold` | 命中阈值，0–1，默认 `0.20` |
| `limits.timeout_ms` | 同步钩子超时；本地模型慢时保留 5000 |
| `limits.memory_mb` | Go 运行时需要较多内存，默认 128 MiB |

如果 TensorFlow Serving 跑在 Docker 容器里，把 `http_allow` 改为容器名/端口（例如 `tensorflow-serving:8501`），并把 `api_url` 指向同一主机。

## 本地模型 mock

没有 TensorFlow 环境时，可用内置脚本验证链路：

```bash
python3 server/hooks/spam_block_wasm/mock_model.py
```

它监听 `127.0.0.1:8501`，按关键词返回三类预测。然后在 UI 里审批启用插件，发一封测试邮件即可看到状态变化。

## 安全边界

- 只能访问 `http_allow` 中精确列出的主机；不跟随重定向。
- 单次请求体 ≤ 1 MiB，响应 ≤ `output_bytes`（默认 256 KiB），HTTP 超时 10s。
- 仅 `http` 能力被审批后才会注册 `pmail_http_post` 导入；未授权时插件无法实例化。
- 插件以 128 MiB 内存上限运行，超时/陷阱只影响本次钩子，邮件继续处理。
