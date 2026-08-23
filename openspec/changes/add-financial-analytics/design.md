## Context

`transactions.amount` 是有符号整数:income 存正数、expense 存负数、balance_adjustment 存调用方提供的带符号值(见 [internal/tools/transactions.go](internal/tools/transactions.go) 的 `validateTransactionInput`)。账户余额从不作为存储字段,而是对 `transactions` 表按 `account_id` 现算 `SUM(amount)`(见 [internal/store/queries.sql](internal/store/queries.sql) 的 `GetAccountBalance`)。这次新增的聚合统计延续同样的原则:不新增表、不缓存,查询时对 `transactions` 现算 `SUM`/`GROUP BY`。

`currency` 是账户上的字段,不在 `transactions` 表里;要按币种分组统计,需要 JOIN `accounts`。

参见 proposal.md - Why。

## Goals / Non-Goals

**Goals:**
- 一次查询拿到:总收入/支出/净额(按币种)、按分类拆分、按账户拆分、balance_adjustment 单独总额
- 聚合完全在 SQL 层完成(`SUM` + `GROUP BY`),不把明细交易读到 Go 层再累加,避免大账本下的内存/性能问题

**Non-Goals:**
- 不做跨币种换算或"本位币"概念(proposal.md 已声明)
- 不做趋势/月度序列、不做对账单(留给后续 change)
- 不做结果缓存——账本规模决定了现算 `SUM` 足够快,和现有 `GetAccountBalance` 的取舍一致

## Decisions

**新增两条 sqlc 查询,而不是一条大查询**
`SearchTransactions` 之外单独加:
- `SummarizeTransactionsByCurrency`:按 `accounts.currency` 分组,分别 `SUM(amount) FILTER (WHERE type = 'income')`、`SUM(-amount) FILTER (WHERE type = 'expense')`(expense 存的是负数,取负得到正的支出金额)、`SUM(amount) FILTER (WHERE type = 'balance_adjustment')`,并支持可选的 `start_time`/`end_time` 过滤(用 `sql.NullInt64`,和 `SearchTransactions` 现有的可选范围过滤是同一模式,见 [internal/store/queries.sql:94](internal/store/queries.sql:94))
- `SummarizeTransactionsByCategory` / `SummarizeTransactionsByAccount`:各自按 `category_id`/`account_id` + `accounts.currency` 分组,只统计 `type IN ('income', 'expense')`(balance_adjustment 没有 category,也不计入这两个拆分维度)

拆成三条查询而不是一条塞满 `CASE WHEN` 的巨查询,是因为它们的 `GROUP BY` 维度不同(币种 vs 分类+币种 vs 账户+币种),硬塞一条查询会需要在 Go 侧对同一行拆分出三种不同粒度的结果,可读性更差。三条都走同一个 `start_time`/`end_time` 过滤,SQLite 对小账本(个人记账场景)执行三次全表扫描级别的聚合查询没有实际性能问题。

**SQLite 不支持标准 `FILTER` 子句**——用 `SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END)` 代替 `FILTER (WHERE ...)` 的写法(纯语法选择,行为等价)。

**响应结构:三个独立的列表,而不是嵌套树**
`by_category`/`by_account` 直接返回 `[]CategorySummary`/`[]AccountSummary`(各自带上 `currency`),不做"先按分类分组、组内再按币种嵌套"的树形结构。Agent 消费聚合结果是为了转述给用户或做进一步计算,扁平列表比嵌套结构更容易在 JSON schema 里描述、也更容易被 Agent 直接遍历。同一分类下出现多种币种时,`by_category` 会有该分类的多行(每行一个币种),而不是一行里塞一个币种字典。

**复用 `formatID`/币种校验等既有工具函数**,新代码只放在新文件 `internal/tools/analytics.go`,不修改 `transactions.go`/`accounts.go`/`categories.go`,以及新查询只追加到 `queries.sql` 末尾,不改动已有查询语句——这是本次 change 明确要和"search_transactions 分页"那个 change 并行开发、避免触碰同一批文件的硬约束(见 proposal.md - Impact)。

## Risks / Trade-offs

- [风险] 三条独立查询而不是一条,统计范围很大的账本时会有 3 倍的全表扫描 → 缓解:个人记账场景的数据量级(几千到几万笔交易)下 SQLite 聚合查询本身就是毫秒级,且已有 `idx_transactions_time` 索引覆盖 `time` 过滤;真正体量大到需要优化时再重新评估,不为假设中的规模预先做优化
- [风险] `by_category`/`by_account` 返回列表没有分页,账本分类/账户特别多、且统计范围横跨全部历史时,列表可能很长 → 缓解:分类/账户数量本身通常是几十个量级(用户手动维护的结构性数据,不是交易明细),不会像交易记录一样无限增长,暂不引入分页;如果未来证明有必要,是独立的后续 change

## Open Questions

(无——响应字段的粒度、聚合口径、文件边界已在上面的 Decisions 中确定)
