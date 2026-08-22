## Why

`github.com/mayswind/ezbookkeeping` 的 `pkg/services` 不是一个为外部复用设计的稳定库边界,而是它自己 `pkg/api` handler 的实现细节:方法签名围绕它自己的场景设计(比如 `ModifyAccounts` 为"一个主账户带多个不同币种子账户"设计,`DeleteAccount`/`DeleteCategory` 里级联检查它自己的定时交易模板功能),字段级别的更新权限只在它未被复用的 API 层做校验,ID 生成是为多实例部署设计的 Snowflake 方案,交易时间戳专门为"跨时区旅行的多用户 web 应用"设计。这些复杂度都是 ezbookkeeping 自己场景带来的,而 tally 明确排除了这些场景(单用户、无子账户、无模板、无 UI)。在给账户/分类加 update/delete 能力时,每加一个操作都要重新考古它未文档化的隐藏规则,而不是设计 tally 自己需要的东西——这个模式会在后续每个 change 里重复出现。

这次换掉整个存储与业务逻辑层,自己设计一套只服务 tally 实际场景的 schema,范围严格限定为**功能对等迁移**:让 [mcp-server-core](../archive/2026-08-22-mcp-server-core) 已经交付的七个工具(`list_accounts`/`manage_account`/`list_categories`/`manage_category`/`create_transaction`/`get_transaction`/`search_transactions`)在新底座上原样可用,不加新能力。账户/分类的 update/delete 留给下一个 change,在新架构上做会干净得多——不再需要处理 ezbookkeeping 的历史包袱。

## What Changes

- 从 `go.mod` 彻底移除 `github.com/mayswind/ezbookkeeping` 依赖
- 自己设计 SQLite schema(accounts / categories / transactions 三张表),用手写 SQL + `sqlc` 生成类型安全的 Go 代码,不引入 ORM
- 账户余额不再是存储字段,改为查询时对交易表 `SUM(amount)` 现算,结构上消除"余额与流水对不上"的可能
- 彻底去掉"用户"概念——不再有 `uid` 列、不再有启动时 bootstrap 一个假用户的逻辑;整个数据库就是隐式的唯一账本
- 主键改用 SQLite 原生自增整数,不再依赖类 Snowflake 的分布式 ID 生成;线上协议仍把 ID 编码成十进制字符串(这个约定不变)
- 不引入任何时区配置——`time` 字段在现有 wire 格式里一直是纯 Unix 秒数,从未暴露过时区语义,ezbookkeeping 的 `TimezoneUtcOffset` 纯粹是它自己 service 签名要求的内部管道,原样去掉即可,不需要替代方案
- 移除 `TALLY_DEFAULT_CURRENCY` 配置项——它现在唯一的用途是喂给即将删除的 ezbookkeeping 用户 bootstrap,对工具行为从无影响
- 引入通用的 `balance_adjustment` 交易类型(取代 ezbookkeeping 的 `modify_balance`),金额是带符号的增量(而不是 ezbookkeeping 那种"绝对目标值 + 只能在账户为空时用一次"),`manage_account` 内部用它把非零初始余额落成一笔真实交易;这次不作为 `create_transaction` 可选的 `type` 暴露给 agent 主动调用,留给下一个 change
- **BREAKING(小,主动选择)**:`search_transactions` 不再像现在这样默认把这类交易过滤掉——现状是 `get_transaction` 按 ID 能查到、但 `search_transactions` 默认搜不到([transactions.go:229-232](../../../internal/tools/transactions.go));余额既然已经现算(见上一条),没有再"隐藏一笔特殊记账"去维持缓存字段一致性叙事的必要,新实现里两个工具一视同仁。这是本次唯一一处对已测试行为的主动偏离,其余全部维持功能对等
- 自己维护一份对齐 ISO 4217 标准的静态币种表(代码 + 小数位数),取代 ezbookkeeping 的货币校验器;金额换算改为按币种真实小数位数处理(例如日元 0 位、科威特第纳尔 3 位),不再是 ezbookkeeping 那种不管什么币种都固定按两位小数换算的简化方式。**BREAKING**:对非两位小数的 ISO 4217 币种,同样的整数金额在新旧实现下代表的实际数值不同;由于目前没有任何真实账本数据,这个影响是理论上的
- 转账类型交易依然不在这次范围内(继承自 mcp-server-core 最初的排除范围,不是这次新缩小的),而且这次**不**预留 `related_account_id` 之类的列——转账需要的汇率处理还完全没有设计过,与其现在猜一个可能猜错的列结构,不如等真正设计转账时做一次有依据的 schema migration
- SQLite 驱动改用 `modernc.org/sqlite`(纯 Go,无需 cgo);开启 WAL 模式 + `busy_timeout`;任何一次涉及多条写入的操作(如建账户带初始余额)强制包在同一个 SQL 事务里
- MCP 传输层(`internal/mcpserver`、`internal/authn`,bearer token 认证、JSON-RPC 分发)不受影响,原样保留

## Capabilities

### New Capabilities
(无)

### Modified Capabilities
(无——`account-management`、`category-management`、`transaction-recording` 三份 spec 的每条 Requirement 和 Scenario 都写在外部行为层面,不涉及本次改动的任何实现细节,原文照样成立。这是一次纯粹的后端替换,不是行为变更,`.openspec.yaml` 已设置 `skip_specs: true`。)

## Impact

- `go.mod`/`go.sum`:移除 `github.com/mayswind/ezbookkeeping` 及其全部间接依赖(含 `mattn/go-sqlite3`);新增 `sqlc` 生成代码所需的运行时依赖与 `modernc.org/sqlite`
- 重写:`internal/bootstrap/`(datastore 初始化、用户 bootstrap 逻辑整个替换/删除)、`internal/tools/`(accounts.go / categories.go / transactions.go 改为调用新的查询层)及对应的全部测试
- 新增:`schema.sql`、`sqlc.yaml`、sqlc 生成的查询代码、ISO 4217 币种参考表
- 不受影响:`internal/mcpserver/`、`internal/authn/`、`cmd/tally-mcp/main.go` 的整体结构(具体的 bootstrap 调用会跟着调整)
- 本地此前不存在真实的 `tally.db`(`*.db` 本就在 `.gitignore` 里,从未入库),无需考虑旧数据迁移
- `openspec/config.yaml` 里描述"把 ezbookkeeping 当 Go 库依赖引入"的项目上下文段落已经不成立,需要同步更新(随本 change 一并处理,不算作 spec 变更)
