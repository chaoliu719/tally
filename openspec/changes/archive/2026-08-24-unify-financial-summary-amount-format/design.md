## Context

见 proposal.md。`mcp/internal/tools/analytics.go` 里 `CurrencyTotals.Income`/`Expense`/`Net`、`CategorySummary.Income`/`Expense`、`SourceSummary.Income`/`Expense` 全部是 `int64`,单位是该 `Currency` 字段的最小单位;`getFinancialSummary` 从 store 层查出的聚合值(已经是最小单位整数,由 SQL `SUM(amount)` 算出)直接透传到这些字段,没有经过任何格式化。这次改动复用 `unify-transaction-amount-format` 已经落地的 `currency.FormatMajor`,不新增换算逻辑,也不改动本次范围之外的任何存储或聚合代码。

## Goals / Non-Goals

**Goals:**
- `get_financial_summary` 返回的每一个金额字段(`income`/`expense`/`net`,含按分类、按来源拆分的小计)都改为十进制字符串,精度由该行所属的 `currency` 决定。
- `Net` 为负时(净支出)字符串带负号,格式与 `amount` 字段的正数格式在小数位数上保持一致(如 CNY 恒 2 位:`"-50.00"`)。

**Non-Goals:**
- 不改动 SQL 聚合逻辑或 store 层——`SUM(amount)` 算出的仍是最小单位整数,只是这次改在工具层格式化时才转换成字符串。
- 不新增货币换算函数——直接复用 `FormatMajor`。
- 不改动 `get_financial_summary` 的入参(`ledger_id`/`start_time`/`end_time`)或分组逻辑。

## Decisions

### 复用 `FormatMajor`,不新增函数
`FormatMajor(code string, minorUnits int64) (string, error)` 在 `unify-transaction-amount-format` 里已经正确处理了负数输入(先取绝对值格式化,再在结果前加负号),`Net` 直接传原始的、可能为负的最小单位整数进去即可,不需要额外包装。

*备选方案*:为 `Net` 单独写一个"允许负数"的格式化函数,与 `amount` 用的"只允许正数"版本区分开。**未采用**——`FormatMajor` 本身从未假设过输入非负,专门拆两个函数是不必要的重复。

### `Income`/`Expense` 复用同一个换算,即使它们语义上总是非负
`Income`/`Expense` 来自 SQL 里对 `type='income'`/`type='expense'` 的分别求和,业务上不会是负数,但 `FormatMajor` 不需要为此加一条"拒绝负数"的校验——这是只读聚合接口的输出格式化,不是写入侧校验,信任 store 层聚合的正确性,不在格式化这一步重复防御。

### 校验范围不变
`get_financial_summary` 本身没有金额类入参,这次改动不涉及任何新增的输入校验逻辑,只涉及输出端的格式化。

## Risks / Trade-offs

- **[Risk]** `Net` 为负时如果沿用 `amount` 那套"永远是正数"的心智模型去写测试,容易漏掉负号断言。 → **Mitigation**:单测显式覆盖"支出大于收入"场景,断言 `Net` 字符串带负号。
- **[Risk]** `analytics_test.go` 里既有用例的 `int64` 字面量机械改写时手滑。 → **Mitigation**:改写时保留对同一场景下 `CurrencyTotals.Income`/`Expense` 差值与 `Net` 字符串的交叉校验(`Net` 应等于 `Income - Expense` 换算后的结果),不是分别孤立断言。
- **[Risk]** 破坏性变更,无兼容垫片。 → **Mitigation**:与上一次改动一致,当前无真实数据依赖旧格式,接受直接切换。

## Migration Plan

- 无数据迁移——存储与聚合不变,只改工具层输出编码。
- 无兼容垫片、无灰度开关,直接切换。
- 回滚方式:还原这次改动的提交即可。
