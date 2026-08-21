## Why

需要一个纯 MCP、无 REST API、无前端的记账 server,让 Claude 这样的 agent 能直接对话式记账,同时复用 ezbookkeeping 已经成熟的记账业务逻辑,而不必自己重新实现多币种、余额计算这类容易出错的东西。这个 change 打通"HTTP → bearer token 认证 → MCP JSON-RPC → 工具分发 → ezbookkeeping service → SQLite"这条完整链路,交付一个从空账本能真正走完"建账户 → 建分类 → 记一笔交易 → 查交易"全流程的最小可用骨架,验证整体架构可行,后续工具按同一模式增量补齐。

## What Changes

- 新建 Go module,把 `github.com/mayswind/ezbookkeeping` 当库依赖引入,只用其 `pkg/services`、`pkg/models`、`pkg/datastore`、`pkg/settings` 等业务逻辑与数据层,不引入其 `pkg/mcp`、`pkg/api`
- 启动时初始化全新 SQLite 数据库(复用 ezbookkeeping 的 datastore/migration),provision 唯一的单用户
- 实现 HTTP server 及单个静态 bearer token 认证中间件
- 自研 MCP JSON-RPC 协议层:`initialize` / `ping` / `tools/list` / `tools/call`,以及一个可扩展的工具注册表(不依赖官方 `pkg/mcp` 的私有实现,只参考其设计)
- 实现最小工具集,足以从空账本走完建账户、建分类、记交易、查交易全流程:
  - `list_accounts`、`manage_account`(仅创建)
  - `list_categories`、`manage_category`(仅创建)
  - `create_transaction`、`get_transaction`、`search_transactions`
- 明确不在这次实现,留给后续 change:账户/分类的更新与删除、标签与标签分组、自定义汇率、批量交易操作与 preview+apply 安全机制、分析聚合工具、transfer 类型交易

## Capabilities

### New Capabilities
- `mcp-transport`: MCP 协议与传输层——HTTP server、bearer token 认证、JSON-RPC 方法分发(`initialize`/`ping`/`tools/list`/`tools/call`)、可扩展工具注册表
- `account-management`: 通过 MCP 工具查看和创建账户
- `category-management`: 通过 MCP 工具查看和创建交易分类
- `transaction-recording`: 通过 MCP 工具记录单条交易、按 ID 查询、按条件搜索交易

### Modified Capabilities

(无——全新项目,此前没有已发布的 spec)

## Impact

- 新建 Go 模块(当前 `tally` 仓库),依赖 `github.com/mayswind/ezbookkeeping`(仅其库包,pin 到具体版本)
- 运行时在本地生成一个全新的 SQLite 文件作为账本,不对接、不影响任何已有系统
- 新增一个监听 HTTP 端口的常驻进程,唯一对外接口是 `/mcp`(JSON-RPC + bearer token 认证)
- 不涉及既有代码改动——项目此前只有 OpenSpec 脚手架,没有实现代码
