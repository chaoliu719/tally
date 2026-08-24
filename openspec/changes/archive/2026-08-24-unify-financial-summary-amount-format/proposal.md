## Why

`unify-transaction-amount-format`(已归档)把 `create_transaction`/`get_transaction`/`search_transactions`/`update_transaction` 的 `amount` 从"最小单位整数"改成了币种感知的十进制字符串,修掉了"调用方必须自己心算元↔分换算、服务端不校验"的问题。但排查那次事故时确认,`get_financial_summary`(`financial-analytics` 能力)的 `income`/`expense`/`net` 三个聚合字段完全没动——它们现在仍然是"该币种最小单位"的 `int64`,jsonschema 描述也还是同一句"in this currency's smallest unit",跟改之前的 `amount` 是一模一样的契约:调用方(驱动 MCP 的 LLM agent)必须自己换算,服务端不做任何量级校验。这是同一类风险在另一个工具上的复现:调用方读到 `income: 500000` 时,如果没有意识到单位是分,直接当成 ¥500000 汇报给用户,就是同一种"¥50 显示成 ¥5,000"式的错误,只是发生在汇总场景而不是单笔记录场景。

## What Changes

- `get_financial_summary` 返回结构中所有金额字段的线上表示,从"最小单位整数"改为"十进制字符串,单位是该币种的主单位",做法与 `unify-transaction-amount-format` 完全一致:复用同一张 `currency.Decimals` 精度表、同一个 `currency.FormatMajor` 换算函数,不重新发明格式化逻辑。
  - **BREAKING**: `CurrencyTotals.Income`/`Expense`/`Net` 从 `int64` 改为十进制字符串。
  - **BREAKING**: `CategorySummary.Income`/`Expense` 从 `int64` 改为十进制字符串。
  - **BREAKING**: `SourceSummary.Income`/`Expense` 从 `int64` 改为十进制字符串。
- `Net` 字段(`income - expense`)在这里可能为负(净支出),因此复用的换算函数必须正确处理负数最小单位整数——不同于 `amount`(永远是正数,符号由 `type` 字段携带),`Net` 的符号就是该数值本身的符号,输出字符串需要带负号(如 `"-50.00"`)。`FormatMajor` 已支持负数输入(见其实现),这里直接复用,不需要新增函数。
- 数据库聚合逻辑(`SUM(amount)`)与 SQL 查询本身不变——这纯粹是 MCP 工具边界的线上格式变更,不涉及聚合计算方式。
- 不引入金额校验:`get_financial_summary` 是只读聚合接口,没有入参金额,不存在"格式非法/精度超限"这类写入侧的校验需求,这次改动只涉及输出格式化。

## Capabilities

### Modified Capabilities
- `financial-analytics`:`get_financial_summary` 返回的 `income`/`expense`/`net`(含按分类、按来源拆分的小计),从"最小单位整数"改为"主单位十进制字符串",精度由该字段所属分组的 `currency` 决定。

## Impact

- 代码:`mcp/internal/tools/analytics.go`(`CurrencyTotals`/`CategorySummary`/`SourceSummary` 及 `getFinancialSummary` 中构造这些结构体的位置)。不新增 `currency` 包函数——直接复用 `unify-transaction-amount-format` 已经实现的 `FormatMajor`。
- 已有测试:`analytics_test.go` 中依赖 `int64` income/expense/net 字面量的用例需要同步改写。
- 当前无真实账本历史数据依赖旧格式,视为无迁移成本的破坏性变更,做法与上一次改动一致:无兼容垫片,直接切换。
