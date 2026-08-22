## Context

见 proposal.md 的 Why/What Changes。当前仓库除 OpenSpec 脚手架外没有任何实现代码,是一张白纸。约束条件:server 依赖一个我们不控制、仍在演进的外部 Go module(`github.com/mayswind/ezbookkeeping`),以及一个同样在快速迭代的外部协议 SDK(MCP 官方 Go SDK)。这两个依赖的版本锁定策略,是本设计要重点交代的部分。

## Goals / Non-Goals

**Goals:**
- 建立一套可复制的骨架模式:HTTP 传输 → bearer token 认证 → MCP 协议分发 → 工具处理函数 → ezbookkeeping service 调用 → SQLite。后续新增工具(标签、汇率、分析、preview+apply 等)只需照着这个模式加新的工具处理函数文件,不需要再碰传输/认证/协议分发这层基础设施代码。
- 让 7 个工具(list_accounts、manage_account、list_categories、manage_category、create_transaction、get_transaction、search_transactions)全部通过这套骨架真实调用 ezbookkeeping 的 service 层并落到 SQLite,而不是打桩或者绕过业务逻辑。

**Non-Goals:**
- 不做生产级运维能力(结构化日志、指标、优雅关闭之外的可观测性),这些等实际需要时再加,不在这次骨架里预置。
- 不实现除以上 7 个工具外的任何工具——包括 proposal.md 中列出的更新/删除/标签/汇率/分析/批量与 preview+apply。
- 不处理多用户、多进程部署、水平扩展;当前是单进程、单 SQLite 文件、单用户。

## Decisions

### 1. Go module 命名与目录结构
用 `module tally`(不带域名前缀的本地模块名——项目还没决定是否/在哪发布,用简短名避免现在瞎猜一个 GitHub 用户名,以后要发布再改一行 go.mod 即可,属于零成本的后置决定)。

```
cmd/tally-mcp/main.go     — 进程入口:加载配置、初始化 DB、bootstrap 用户、启动 HTTP server
internal/bootstrap/        — 配置加载、DB 初始化、单用户 bootstrap
internal/authn/            — bearer token 校验中间件
internal/mcpserver/        — MCP 协议层:基于官方 SDK 搭建 server、注册工具、HTTP handler
internal/tools/            — 每个 MCP 工具一个文件,内部调用 ezbookkeeping 的 pkg/services
```
用 `internal/` 是因为这次是我们自己的应用代码,没有被外部 import 的需求(和 ezbookkeeping 本身刻意不用 `internal/` 以保持可被我们 import 的原因相反)。

### 2. MCP 协议层基于官方 Go SDK,而不是手搓 JSON-RPC
用 `github.com/modelcontextprotocol/go-sdk` 的 `mcp` 包搭建 server、注册工具、跑 HTTP 传输,而不是手写 JSON-RPC 请求解析/分发(那是之前决定不去照抄 ezbookkeeping 私有 `pkg/mcp` 时,"自研 MCP 层"字面上可能的一种做法,但没必要——早先的决定是不依赖 ezbookkeeping 那个绑定了它自己业务代码的私有实现,不是要拒绝整个 MCP 生态的标准 SDK)。官方 SDK 由 MCP 维护方与 Google 共同维护,已发布到 v1.7.0(稳定,非实验版本),自带协议版本协商,向下兼容当前主流客户端(包括 Claude Code)仍在用的经典 `initialize` 握手流程。用它换来协议正确性上的省心,和"复用 ezbookkeeping service 层换来记账正确性上的省心"是同一个工程判断。

风险与应对见下面 Risks 一节。

### 3. HTTP 层用标准库 `net/http`,不引入额外路由框架
整个 server 只有一个 MCP 端点(SDK 提供 HTTP handler)加一个健康检查,标准库的 `http.NewServeMux` 完全够用,没必要为此引入 gin 或其他路由框架、增加依赖面。

### 4. Bearer token 来自环境变量,缺失即拒绝启动
`TALLY_MCP_TOKEN` 环境变量必须在启动时设置;缺失时 server 直接报错退出,不自动生成、不静默用空 token 运行。校验时用 `crypto/subtle.ConstantTimeCompare` 做常量时间比较,不用普通字符串 `==`,避免时序侧信道泄露 token 内容。

### 5. 单用户 bootstrap:启动时"不存在则创建",uid 常驻内存
进程启动时检查 DB 里是否已有用户,没有则调用 `pkg/services.UserService` 创建唯一一个用户(默认币种通过 `TALLY_DEFAULT_CURRENCY` 环境变量配置),密码字段用随机值填充但永远不会被读取或校验——认证完全走 bearer token,不走 ezbookkeeping 原本的密码/会话体系。创建后把这个用户的 uid 存在内存里,后续每次工具调用都直接用这个常量 uid 调 service,不做每请求的用户解析。这就是"不实现 UserService 传统认证机制"这条决定在代码层面的落地方式。

### 6. 复用 `pkg/settings.Config`,但只填必要字段
ezbookkeeping 的 service 函数签名普遍要求传入 `*settings.Config`(例如统计/汇率相关调用),所以我们复用这个类型而不是自己定义一个新的配置结构体,但只赋值我们真正用到的字段(数据库路径等),其余保持零值——不实现 ezbookkeeping 完整的 ini 配置文件格式,也不暴露那些我们不用的功能开关。

### 7. 复用 `pkg/datastore` 做 SQLite 初始化与迁移
数据库 schema 与迁移逻辑完全复用 ezbookkeeping 自己的实现,保证我们的 SQLite 文件结构和官方一致(即使我们不打算真的对接已有实例,保持 schema 兼容也是低成本、高回报的——万一以后想用官方工具做数据检查/修复,文件格式是通的)。

### 8. 依赖版本锁定策略
`go.mod` 里 `github.com/mayswind/ezbookkeeping` 和 `github.com/modelcontextprotocol/go-sdk` 都锁定到实现时的具体版本(优先用 tag,没有 tag 就用 commit 的 pseudo-version),不用 `go get -u` 自动跟随最新。两者都是我们不控制、仍在演进的外部依赖,升级是后续单独的、有意识的动作,不是这次骨架的一部分。

## Risks / Trade-offs

- **[风险]** ezbookkeeping 不是作为公开 library 维护的,其 `pkg/services`/`pkg/models` 的方法签名没有稳定性承诺,未来它自己重构可能直接 break 我们的编译。
  → **应对**:锁定具体版本(见决定 8),升级前先看 diff,不盲目跟随。
- **[风险]** MCP 官方 Go SDK 处于快速迭代期(几个月内发布了多个版本,协议版本也在演进)。
  → **应对**:同样锁定具体版本;实现时以当时最新的 stable release 为准记录进 tasks.md,不假设本设计文档里提到的具体版本号在实现时依然是最新。
- **[风险]** SQLite 单文件在并发请求下的写锁竞争——虽然单用户场景下真实并发写的概率低,但 MCP 客户端仍可能同时发出多个工具调用。
  → **应对**:ezbookkeeping 的 `pkg/datastore` 大概率已经处理了这个(连接池/锁策略),实现时验证其默认行为是否够用,不在此设计里预先假设需要额外加锁。
- **[风险]** bootstrap 阶段"随机生成、永不使用的密码"这种做法,如果日后不小心接上了 ezbookkeeping 原生的密码登录路径(比如误用了它的某个 API),会造成一个事实上无法登录、状态诡异的账户。
  → **应对**:因为我们完全不 import `pkg/api`,原生登录路径在这个二进制里根本不存在,物理上不可达,不只是"约定不用"。

## Migration Plan

全新部署,没有既有数据或既有版本需要迁移/回滚。首次运行步骤:设置 `TALLY_MCP_TOKEN`(必需)与 `TALLY_DEFAULT_CURRENCY`(可选,默认 CNY)→ 启动二进制 → 首次启动自动创建 SQLite 文件、跑迁移、创建唯一用户 → 用 `list_accounts`/`manage_account` 建第一个账户,`list_categories`/`manage_category` 建第一个分类,即可开始用 `create_transaction` 记账。
