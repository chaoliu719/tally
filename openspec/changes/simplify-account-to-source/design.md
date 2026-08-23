## Context

当前存储层(`internal/store/schema.sql`)里,`accounts` 表有 `name`/`type`/`currency`/`comment`,`transactions` 表通过 `account_id` 外键关联账户,自己没有 `currency` 列——交易的币种完全靠 `JOIN accounts` 派生;`internal/tools/transactions.go` 里 `CreateTransactionInput`/`UpdateTransactionInput` 也确实没有 `currency` 参数。`internal/store/queries.sql` 里 `SummarizeTransactionsByCurrency`/`SummarizeTransactionsByCategory`/`SummarizeTransactionsByAccount` 三个聚合查询都靠 `JOIN accounts a ON a.id = t.account_id` 拿 `a.currency` 做分组。

`categories` 表(`id`/`name`/`parent_id`/`created_at`/`updated_at`)是本次"来源"要对齐的目标形态——本次改动之后 `sources` 表比 `categories` 还要再少一个 `parent_id`,只剩 `id`/`name`/`created_at`/`updated_at`。

项目本身没有正式用户数据和迁移框架(见项目 context:"数据:全新账本,全新 SQLite 数据库文件"),`schema.sql` 用 `CREATE TABLE IF NOT EXISTS` 一次性建表,不做版本迁移。

## Goals / Non-Goals

**Goals:**
- 把 `accounts` 表替换为极简的 `sources` 表(`id`/`name`),`transactions.account_id` 改名为 `source_id`。
- 把币种从"来源"移到交易自身:`transactions` 新增 `currency` 列,`create_transaction`/`update_transaction` 新增必填 `currency` 参数,现有的静态 ISO 4217 校验表直接复用,只是校验对象从账户输入换成交易输入。
- 彻底移除 `adjustment` 交易类型:`transactions.type` 枚举、`CHECK` 约束、三个聚合查询里所有 adjustment 相关的分支和字段一并删除。
- 聚合查询的分组维度从 `JOIN accounts` 拿 `a.currency`/`a.id` 改为直接用 `transactions` 自身的 `currency`/`source_id`,`SummarizeTransactionsByAccount` 改名为 `SummarizeTransactionsBySource`。

**Non-Goals:**
- 不做账户间转账(这正是被本次改动否决的前提)。
- 不做历史数据迁移——项目当前没有需要保留的正式数据,变更直接改 `schema.sql`/`queries.sql` 并重新生成 sqlc 输出。
- 不改变 `write-confirmation`(preview → apply)机制本身,只是把 `manage_account` 的删除保护逻辑原样搬到 `manage_source`。

## Decisions

**`sources` 表替代 `accounts` 表,而不是在原表上做 `ALTER TABLE` 增删列。**
项目没有迁移框架,`schema.sql` 本来就是一次性建表;既然要改名又要削减字段,直接改写 `CREATE TABLE` 语句、把 `internal/store` 下手写的 SQL 和 sqlc 生成代码一起重新生成最直接,不需要引入任何 `ALTER TABLE` 迁移语句。

**`currency` 落到 `transactions` 表,而不是继续留在某个"账户"式实体上。**
考虑过的替代方案:(a) 保留一个只带 currency 的轻量"来源"字段——被否决,因为这等于又把已经确认去掉的 currency 加回来,只是换了个名字;(b) 整个系统只支持单一隐式币种——被否决,因为会推翻已经确认的"`get_financial_summary` 保留按币种分组"这条需求。最终选择把 currency 作为交易自身的必填字段,这样"一笔交易发生在什么币种下"这件事只由这笔交易自己决定,不再依赖任何主数据实体。

**`transactions.type` 只剩 `income`/`expense` 后,`category_id` 直接改为 `NOT NULL`,移除原来"income/expense 需要分类、adjustment 不需要"的 `CHECK` 约束。**
既然只剩两种都需要分类的类型,这条 `CHECK` 分支本身就没有存在的必要了。

**聚合查询按 `t.currency`/`t.source_id` 分组,去掉 `JOIN accounts`(未来是 `JOIN sources` 或干脆不 join,因为分组不再需要来源表的任何字段)。**
`SummarizeTransactionsByCurrency` 不再需要 `adjustment` 这一列;`SummarizeTransactionsByAccount` 改名为 `SummarizeTransactionsBySource`,`WHERE t.type IN ('income', 'expense')` 这个过滤条件可以去掉,因为不会再有别的类型。

## Risks / Trade-offs

- [没有历史余额修正手段] → 之前 `adjustment` 类型是"修正账户余额的正式方式";现在没有余额概念,也没有替代机制。如果记录有误,只能直接改/删对应的 income/expense 交易本身。这是本次改动的既定取舍(见 proposal.md - Why),不是遗留风险。
- [`sources`/`transactions` 两张表要同时改,且 sqlc 生成代码、Go 结构体、MCP 工具 schema 都要联动改名] → 改动面比字段级的重命名(如之前的 `balance_adjustment` → `adjustment`)更大。缓解:任务拆分时按"schema → 生成代码 → 工具层 → 测试"的顺序推进,每一步都能独立编译验证。

## Migration Plan

项目没有正式用户数据,不提供旧数据迁移。实施步骤:
1. 改 `internal/store/schema.sql`(`accounts` → `sources`,`transactions` 新增 `currency`、`account_id` 改名 `source_id`、移除 adjustment 相关的 `CHECK`)与 `internal/store/queries.sql`,跑 sqlc 重新生成 `internal/store/queries.sql.go`/`querier.go`。
2. 改 `internal/tools` 下账户/交易/分析相关的工具实现与其输入输出结构体。
3. 改 `internal/tools` 下对应的单元测试与 `cmd/tally-mcp` 下的 e2e 测试。
4. 改 `README.md` 里提到 account/adjustment 的地方。
5. 本地已有的测试用 SQLite 文件如果还在用旧 schema,直接删除重建即可,不需要额外迁移脚本。

回滚策略:改动集中在这一个 change 内,没有对外发布过,直接回退这个 change 的提交即可。
