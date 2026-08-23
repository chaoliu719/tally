## Why

`manage_account`/`manage_category` 目前只能创建,不能改名、改类型、挪分类,也不能删除;账户余额一旦记错初始值就没有正式的改法。这次补齐账户与分类的更新、删除能力,并为删除这类破坏性操作建立一个统一的、无状态的二次确认机制(供这次和未来的破坏性操作复用)。顺带修正分类模型里两处从 ezbookkeeping 继承、但在当前代码里从未真正生效过的设计:`category.type` 字段和"分类只能两层"的限制。

## What Changes

- `manage_account`/`manage_category` 新增 `operation` 参数(`create`/`update`/`delete`),不新增独立工具
- 新增一个跨账户/分类复用的 preview → apply 确认机制:同一个工具调用,靠 `confirmation_token` 参数有无区分是第一步(preview)还是第二步(apply);token 无状态、HMAC 签名,由新环境变量 `TALLY_CONFIRMATION_SECRET` 签发和校验(与 `TALLY_MCP_TOKEN` 一样,启动时必需,未设置则 fail-fast)——**BREAKING**(新增必需的启动配置项)
- 账户可改字段:`name`/`type`/`comment`,`operation=update` 要求完整传入这三个字段(不支持只改一个字段的局部更新);`currency`/`balance` 不可通过 update 修改,传了就拒绝
- 账户删除:被任意交易(含 `balance_adjustment`)引用则拒绝删除,不提供强制覆盖;走 preview → apply
- `create_transaction` 新增 `type=balance_adjustment`,作为修正账户余额的正式入口:不能带 `category_id`(传了就拒绝),`amount` 必须非零(可正可负,代表带符号的增量)
- **BREAKING**:`category.type`(income/expense/transfer)字段整体移除——现有代码里这个字段从未被任何业务逻辑实际依赖(`create_transaction` 不校验交易类型与分类类型是否一致),纯粹是未生效的历史包袱。`manage_category` 不再接受/返回 `type`,`list_categories` 的输出相应减少这一字段
- **BREAKING**:分类"只能两层"的限制整体取消——`parent_id` 可以指向任意已存在的分类,嵌套深度不限;`create_transaction` 相应地可以引用任意分类(不再要求必须是二级分类)
- 分类可改字段:`name`/`parent_id`,同样是完整替换语义;挪动(改 `parent_id`)只校验目标分类存在、不是挪给自己、不会形成环,不再有"同类型""必须挪到一级分类下"这类已经连带消失的限制
- 分类删除:有任意子分类,或被任意交易引用,都拒绝删除,不提供强制覆盖;走 preview → apply

## Capabilities

### New Capabilities
- `write-confirmation`: 无状态、HMAC 签名的 preview → apply 两步确认机制,供破坏性写操作(这次是账户/分类删除)使用,后续批量操作等破坏性能力可复用同一套 token 签发/校验规则

### Modified Capabilities
- `account-management`: 新增账户更新、删除能力(经 `write-confirmation` 确认)
- `category-management`: 新增分类更新、删除能力(经 `write-confirmation` 确认);移除 `type` 字段;移除"只能两层"的层级限制,改为任意深度嵌套
- `transaction-recording`: `create_transaction` 新增 `balance_adjustment` 类型;分类引用规则从"必须是二级分类"放宽为"任意已存在的分类"

## Impact

- `internal/tools/accounts.go`、`internal/tools/categories.go`:新增 update/delete 处理逻辑与校验
- `internal/tools/transactions.go`:`create_transaction` 接受 `balance_adjustment`,分类校验规则简化
- 新增一个 token 签发/校验的内部包(preview/apply 共用)
- `internal/store/schema.sql`:`categories` 表移除 `type` 列;`queries.sql`/sqlc 生成代码新增 update/delete/引用计数/祖先链查询
- `internal/bootstrap/config.go`:新增必需环境变量 `TALLY_CONFIRMATION_SECRET`
- 现有测试(`accounts_test.go`/`categories_test.go`/`transactions_test.go`)需要跟着输入输出结构的变化更新
- `openspec/specs/{account-management,category-management,transaction-recording}/spec.md` 三份既有 spec 需要更新
