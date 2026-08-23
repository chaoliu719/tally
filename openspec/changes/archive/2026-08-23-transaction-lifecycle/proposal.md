## Why

`internal/tools/transactions.go` 目前只有 `create_transaction`/`get_transaction`/`search_transactions`,没有更新、删除能力——`openspec/specs/transaction-recording/spec.md` 的 Purpose 里也明确写了"更新、删除、批量操作、transfer 类型交易留给后续 change"。这个缺口和已归档的 `account-category-lifecycle` change 确立的规则——账户/分类只要被任意交易引用就永久拒绝删除、不提供强制覆盖(`internal/tools/accounts.go` 的 `deleteAccount`、`internal/tools/categories.go` 的 `deleteCategory`)——组合后产生一个实际的死锁:只要给一个账户记过一笔交易(哪怕是 `balance_adjustment`),这个账户就永久不可删除,因为没有任何办法先清空它名下的交易记录。这次补上 `update_transaction`/`delete_transaction`,既填上 spec 里早就预留的空位,也打通"先删交易、再删账户/分类"这条本应存在的路径。

## What Changes

- 新增 `update_transaction`、`delete_transaction` 两个独立 MCP 工具——延续 `create_transaction`/`get_transaction`/`search_transactions` 已有的"一个动词一个工具"风格,不引入 `manage_account`/`manage_category` 那种 `operation` 分发(那种风格是为 create/update/delete 共享同一组输入字段而设计的;`create_transaction`/`get_transaction`/`search_transactions` 的输入字段本就互不相同,拆开更符合现状)
- `update_transaction`:字段完整替换语义,接受与 `create_transaction` 完全相同的字段集合(`type`/`account_id`/`category_id`/`amount`/`time`/`comment`),复用完全相同的校验规则(income/expense 需要合法 `category_id` 且 `amount` 为正;`balance_adjustment` 不能带 `category_id` 且 `amount` 非零)。允许修改 `account_id`(相当于把交易改记到另一账户);不需要 `confirmation_token`,直接执行——单条记录的"改"按项目既定原则不强制二次确认
- `delete_transaction`:破坏性操作,走 preview → apply 两步确认,复用 `internal/confirm` 包与既有的 `write-confirmation` 机制。不同于账户/分类删除,交易删除没有"是否被引用"这类前置门槛——只要交易存在就允许删除;revision 覆盖交易的全部字段,用于探测 preview 之后交易是否被并发修改或删除
- `transactions` 表新增 `updated_at` 列,补齐 `accounts`/`categories` 已有的、可变数据表的通用约定
- 明确打通"先用 `delete_transaction` 清空某账户/分类下的全部交易记录、再删除该账户/分类"这条路径,补充覆盖该路径的集成/e2e 测试
- `transaction-recording` capability 的 Purpose 更新:更新、删除已支持;批量操作、transfer 类型交易仍留给后续 change

## Capabilities

### New Capabilities
(none)

### Modified Capabilities
- `transaction-recording`:新增 `update_transaction`、`delete_transaction` 两个 Requirement(含 preview/apply 场景),Purpose 相应更新

## Impact

- `internal/tools/transactions.go`:新增 `update_transaction`/`delete_transaction` 的注册与处理逻辑
- `internal/store/schema.sql`:`transactions` 表新增 `updated_at` 列
- `internal/store/queries.sql`/sqlc 生成代码:新增 `UpdateTransaction`、`DeleteTransaction` 查询
- `internal/confirm`:复用现有 `Issue`/`Verify`,新增一个 `delete_transaction` action 常量,包本身不改动
- `openspec/specs/transaction-recording/spec.md`:新增更新、删除相关 Requirement/Scenario,Purpose 更新
- `internal/tools/transactions_test.go`:新增 update/delete 各场景的单测
- e2e/集成测试:补充"清空某账户或分类下全部交易 → 删除该账户/分类"的完整路径覆盖(参考 `openspec/changes/archive/2026-08-23-e2e-lifecycle-coverage/` 的组织方式)
- `README.md`:Tools 表新增两行
