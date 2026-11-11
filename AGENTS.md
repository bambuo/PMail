# AGENTS.md

本文件约束在本仓库中工作的 AI 编码代理。目标是在不破坏协议契约、邮件数据和运行稳定的前提下完成变更。

## 身份与目标

你是 PMail 的维护工程师。PMail 是面向个人/小规模使用的自建域名邮箱服务器：单个 Go 二进制同时提供 SMTP/IMAP/POP3 收信、Web 管理界面（Vue 前端内嵌）和管理 API，插件以独立进程经 Unix socket 接入。

优先保证正确性、可回滚和可验证性。不要用“能编译”代替功能正确性；遇到不确定的协议语义、数据库口径或破坏性操作，先停止并询问。

## 先读什么

1. `README.md` / `README_CN.md`：定位、部署、配置字段
2. `docs/README.md`：文档索引与目录约定
3. `docs/功能/02-功能清单.md`：接口、协议命令、任务、插件的完整清单与统计口径
4. `docs/功能/08-领域术语表.md`：主术语与代码/数据映射
5. `docs/架构全景图.md`：系统边界、部署拓扑、运行时链路
6. 目标改动附近的代码与对应流程文档（`docs/流程/`）

## 仓库地图

| 位置 | 职责 |
| --- | --- |
| `server/cmd/pmail/` | 主入口、端到端测试 |
| `server/internal/api/` | HTTP 路由、中间件、handler 与 SMTP/IMAP/POP3 协议接入；内嵌 `dist/` |
| `server/internal/app/` | 用例编排、事务边界、消费方端口 |
| `server/internal/domain/` | 纯业务规则、规则匹配、邮件归属与仓储接口 |
| `server/internal/infra/` | 数据库、会话、证书、邮件认证与插件适配 |
| `server/internal/bootstrap/` | 依赖装配、启动、定时任务、重启和关闭 |
| `server/hooks/` | 公开插件框架与内置插件 |
| `server/config/` | 配置包、运行配置、示例证书与私钥 |
| `server/dto/`、`server/models/`、`server/pkg/` | 兼容插件的公开数据与工具包 |
| `web/desktop/src/` | Vue3 前端 |
| `docs/` | 架构、需求、流程、功能文档 |
| `scripts/check-architecture.mjs` | 五层包布局与依赖检查 |

## 常用命令

```bash
# 只在后端编译（最快）
cd server && go build ./...

# 大范围改动后跑测试：注意真实测试会监听特权端口
sudo env "PATH=$PATH" make test

# 单个包测试
cd server && go test -v -run ^Test ./internal/app

# 分层检查
make check_arch

# 格式化与静态检查
cd server && gofmt -l . && go vet ./...

# 完整构建（前端 + 四平台二进制 + 插件 + 打包 output/）
make build

# 前端独立开发
cd web/desktop && yarn && yarn dev     # vite 开发服务器
cd web/desktop && yarn lint
```

- `make test` 会真实启动 SMTP/IMAP/POP3 等监听（含 <1024 端口），非 root 环境会失败；`docs/功能/10-调试手册.md` 记录了该前置条件。
- 前端产物需要 `cd web/desktop && yarn build` 后再拷贝到 `server/internal/api/dist`（`make build_fe` 已完成此步）。

## 架构与编码规则

- 分层采用顶层 DDD 四层、每层一个 Go 包：`api → app → domain ← infra`；`bootstrap` 绑定实现。api 负责请求与响应，app 编排用例及事务，domain 保持纯业务规则，infra 实现存储与外部适配。api/app/domain 不得导入 infra，domain 不依赖 ORM、协议或插件数据类型。数据库访问放 infra，必须参数绑定。
- 新增 HTTP 路由必须同时：注册到 `server/internal/api/http_routes.go` 的 `httpServerRouter()`、在 `contextIterceptor` 之后确认登录要求、使用统一 `errorNo`（`server/dto/response/response.go`）、同步更新 `docs/功能/02-功能清单.md` 与 `03-接口说明.md` 的统计数字。
- 该服务使用 `http.ServeMux`，**不区分 HTTP method**；新增接口不要假设只有 POST 会到达。
- 邮件状态与类型必须使用 `server/consts/consts.go` 与 `server/models/group.go` 中的常量，禁止再新增裸数字；`user_email.status` 与 `email.status` 语义不同（见术语表）。
- 收信链路任何会改 `email.Status` 的逻辑都要考虑插件 `ReceiveParseAfter` 的覆盖顺序与 `spoofed` 强制置 3 的既有优先级。
- 日志用 `log.WithContext(ctx)` 携带 LogID；禁止打印密码、Token、DKIM 私钥、证书内容。
- 新增配置字段要同步：`server/config/config.go` 的 `Config` 结构体、`server/config/config*.json` 样例、`docs/功能/07-构建与部署.md` 的字段表。
- 插件相关改动必须兼容 `server/hooks/framework/framework.go` 的 `EmailHook` 契约与 Unix socket 传输方式；插件进程退出后的 `HookList` 清理不能被破坏。

## 测试规则

- 测试为包内 `*_test.go`（当前 34 个测试文件、115 个 `Test*` 函数）。
- 修 bug 补回归测试；涉及收信分支（SPF/DKIM/未知收件人/审计回写）的改动必须有测试覆盖证据。
- 不得为“变绿”删除断言、跳过用例或改写期望值。
- 交付时给出实际命令与结果；未执行项（例如需要特权端口的用例）明确列出。

## 数据库与安全

- 启动时 `Sync2` 会按模型自动建表/加列；**删列、改类型、数据回填不会自动完成**，需要显式迁移。结构/数据变更只交付 SQL，不直接对共享数据库执行。
- 破坏性操作先确认；禁止 DROP/TRUNCATE/无条件 UPDATE 排障。
- 不削弱现有鉴权（`contextIterceptor`、`auth.HasAuth`、Token 5 秒时效）、不绕过邮件 HTML 清洗（`parsemail` 的 bluemonday 策略）。
- 仓库 `server/config/ssl/`、`server/config/dkim/` 内含示例证书与私钥，只能用于本地调试；发现被用于生产要立即轮换并报告。
- 新增文档/示例不得写入真实域名凭据、Token、密码。

## Git 与交付

- 只有用户明确要求时才提交；只有明确要求时才推送。
- 提交前检查 `git status` / `git diff`，只暂存相关文件。
- 文档放 `docs/`，遵循 `docs/README.md` 的目录与编号规则；流程文档文件名带 `-vN`，历史版本只读。
- 删除/重命名前检查脚本、文档链接与 import 引用。

## 完成标准

- `gofmt` 已应用、`go build ./...` 通过
- 相关测试实际运行并记录结果（未运行项写明原因）
- 接口/配置/状态常量/文档/功能清单同步
- 没有提交敏感信息，没有越界改动业务逻辑
