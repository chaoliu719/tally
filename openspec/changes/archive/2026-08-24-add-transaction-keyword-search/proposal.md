## Why

comment 纪律要求记录/导入交易时把原始凭据文本(账单原文、商户原名)写进 `comment`,此后任何修正只动结构化字段(分类/来源/金额),永不改写 `comment`。这样每笔交易天然带着"原文 vs 解释"两层信息,但目前 `search_transactions` 没有任何按 `comment` 内容筛选的手段——Agent 在记录一笔新交易前,想看"同一个商户/同一串账单文本历史上出现过几次、当时账单上长什么样、用户最终把它归到了哪个分类和来源",除了把整个账本分页拉全量再靠自己扫描之外没有别的办法。这既浪费上下文,又没有把"先例检索"这个基础能力真正交给服务端。本变更给 `search_transactions` 加上 `keyword` 参数,让 Agent 能直接按子串命中历史先例,不需要引入任何外部记忆基础设施。

## What Changes

- `search_transactions` 新增可选入参 `keyword`:对该账本内交易的 `comment` 字段做大小写不敏感的子串匹配,SQL 用 `LIKE` 实现(明确不用 FTS5——个人账本量级,全表扫足够正确)
- `keyword` 与现有过滤条件(`ledger_id`、`start_time`/`end_time`、`source_id`、`category_id`)以 AND 组合,不改变其余过滤条件的语义
- 用户输入中的 LIKE 通配符(`%`、`_`)与转义字符本身在拼接查询前做转义,保证 `keyword` 始终被当作字面子串匹配,不会被误解成通配符表达式
- 空白(空串或仅由空白字符组成)的 `keyword` 视为未提供该参数,与 `source_id`/`category_id` 等现有可选字符串参数"空字符串即未提供"的既有处理方式保持一致,不额外报错
- 分页/cursor 语义不变,但 `keyword` 会被纳入 cursor 的 `filter_fingerprint` 计算,和其余过滤字段一样参与"cursor 是否仍匹配当前筛选条件"的校验,防止用不同 `keyword` 复用旧 cursor 产生语义杂糅的结果

## Capabilities

### New Capabilities
(无)

### Modified Capabilities
- `transaction-recording`:「按条件搜索交易」这条 requirement 的 `search_transactions` 新增 `keyword` 入参,按 `comment` 子串筛选,与其余过滤条件 AND 组合,并纳入 cursor 有效性校验的筛选字段集合

## Impact

- 仅修改 `internal/tools/transactions.go` 里的 `SearchTransactionsInput`/`searchTransactions`,以及 `internal/tools/transactions_cursor.go` 里的 `searchTransactionsFilterFields`(需新增 `Keyword` 字段并参与 `filter_fingerprint` 哈希),不涉及 `create_transaction`/`get_transaction`/`update_transaction`/`delete_transaction`
- `internal/store/queries.sql` 的 `SearchTransactions` 语句新增基于 `comment` 的可选 `LIKE` 过滤条件,不改动其现有的 `source_id`/`category_id`/`start_time`/`end_time`/keyset 分页逻辑
- 不涉及数据库 schema 变更,不新增索引(个人账本量级下全表扫描足够,详见 design.md)
- 与已归档的 `add-transaction-search-pagination` change 在同一批过滤字段与 cursor fingerprint 机制上叠加,不冲突;两者都落在 `SearchTransactions` 这一条查询上,但作用的是不同的过滤维度
