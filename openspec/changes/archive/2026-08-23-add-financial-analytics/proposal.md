## Why

Agent 是账本的唯一操作者,用户全程口头下达指令。"这个月餐饮花了多少"、"这季度净收支多少"是最自然的语音查询,但 tally 目前没有任何聚合查询工具——Agent 只能靠 `search_transactions` 把命中的全部原始交易拉进上下文,自己在对话里手动求和。这既浪费 token,又在跨分类、跨账户求和时容易算错,尤其是多币种场景下还得自己分组不能直接加总。需要一个只读的服务端聚合工具,直接把"这段时间收入/支出/净额,按分类、按账户拆分"算好返回。

## What Changes

- 新增 `get_financial_summary` 只读 MCP 工具:给定时间范围(`start_time`/`end_time`,复用 `search_transactions` 的 unix 秒语义,均可省略表示不限),返回该范围内的:
  - 总收入、总支出、净额(均按币种分组,不做汇率换算——tally 本来就没有本位币概念)
  - 按分类拆分的收入/支出小计
  - 按账户拆分的收入/支出小计
  - `balance_adjustment` 类型交易不计入收入/支出统计(它不是真实的收支行为),但会在响应中单独给出一个总额,避免用户口头问"这段时间对了几次账"时无从查起
- 聚合逻辑直接在 `internal/store`(SQL `SUM`/`GROUP BY`)里现算,不引入独立的统计表或缓存,和现有余额"查询时现算"的原则一致
- 不做:趋势/月度序列(`get_financial_trends`,留给后续 change,一次聚合与一次趋势的实现/spec 复杂度不同,不合并进这次)、对账单(`reconciliation`,需要更完整的账户对账语义)、汇率换算成单一本位币(tally 无本位币概念)

## Capabilities

### New Capabilities
- `financial-analytics`: 提供只读的财务聚合查询能力,当前范围是 `get_financial_summary`——按时间范围统计收入/支出/净额,并按分类、账户拆分

### Modified Capabilities
(无——不修改 `transaction-recording`/`account-management`/`category-management` 现有 requirement;仅新增读取路径,不改变交易/账户/分类的写入或既有查询行为)

## Impact

- 新增 `internal/tools/analytics.go`(工具注册与 handler),不修改 `internal/tools/transactions.go`/`accounts.go`/`categories.go`
- 新增 `internal/store` 侧的聚合查询(在 `queries.sql` 里新增语句,由 sqlc 生成代码;不改动现有查询)
- 新增 `openspec/specs/financial-analytics/spec.md`
- 不涉及数据库 schema 变更(现算聚合,不新增表/列)
