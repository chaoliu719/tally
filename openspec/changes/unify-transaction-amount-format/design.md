## Context

见 proposal.md - Why / What Changes。当前 `mcp/internal/tools/transactions.go` 里 `CreateTransactionInput.Amount`、`UpdateTransactionInput.Amount`、`TransactionInfo.Amount` 都是 `int64`,单位是该币种最小单位;`mcp/internal/currency.Decimals` 提供"币种 → 小数位数"的静态表,但目前没有任何生产代码调用它。存储层(`transactions.amount INTEGER`,最小单位,带符号)与 `queries.sql` 里的 `SUM(amount)` 聚合完全不在本次改动范围内——这次只动 MCP 工具边界的输入/输出格式。

## Goals / Non-Goals

**Goals:**
- `amount` 的线上表示(输入与输出)统一为十进制字符串,单位是该 `currency` 的主单位,小数位数由 `currency.Decimals` 决定(不固定两位)。
- 格式非法、精度超出该币种标准位数、解析后非正的 `amount`,在写入时(`create_transaction`/`update_transaction`)被明确拒绝,不做静默四舍五入或截断。
- 字符串 ↔ 最小单位整数的换算全程用整数运算完成,不引入浮点数,保证换算精确无损。

**Non-Goals:**
- 不改动 SQLite schema 或 `transactions.amount` 列的存储格式(继续是最小单位整数)。
- 不改动 `queries.sql` 里的 `SUM(amount)` 聚合逻辑。
- 不改动 `financial-analytics` 能力(`income`/`expense`/`net` 等字段暂不跟进,见 proposal.md Impact)。
- 不引入跨币种换算能力——`currency` 字段语义不变,这次只影响同一笔交易内 `amount` 与其自身 `currency` 的表示方式。

## Decisions

### 固定位数格式,不裁剪尾随零
输出的金额字符串**固定**保留该币种标准精度对应的小数位数——CNY 恒为 2 位(`"50.00"`,不是 `"50"`),JPY 恒为 0 位(`"5000"`),BHD 恒为 3 位(`"5.000"`)。

*备选方案*:输出时裁剪多余的尾随零(如 `"50"` 而不是 `"50.00"`)。**未采用**——变长精度会让调用方无法仅凭字符串本身判断"这是精确到分的原始值"还是"被裁剪过",给round-trip 测试和下游解析都增加不必要的歧义;固定位数是确定性最强、最容易验证正确性的选择。

### 换算函数放在 `currency` 包,只用整数运算
新增两个函数(命名待实现时定,示意为 `FormatMajor(code string, minorUnits int64) (string, error)` 和 `ParseMajor(code string, s string) (minorUnits int64, err error)`),与现有 `Decimals`/`Supported` 放在一起。实现只做字符串切分、按 `Decimals(code)` 位数补零/校验、`strconv.ParseInt`/整数拼接——**不经过 `float64` 或 `math/big.Float`**,避免二进制浮点数无法精确表示 `0.1`、`25.9` 这类十进制小数的经典问题。

*备选方案*:用 `float64` 做 `amount / 100.0` 再 `strconv.FormatFloat`。**未采用**——即使 Go 的最短往返浮点格式化通常能得到干净的结果,这类隐患不该靠"通常没事"来保证,金额换算应该在设计上就排除浮点误差的可能性,而不是依赖运气。

### 校验顺序:先定币种,再核对金额精度
现有 `validateTransactionInput` 里 `amount` 的校验(`amount <= 0`)在 `currency` 校验之前。这次改动后,必须先知道 `currency` 才能确定 `amount` 字符串允许的小数位数,因此校验顺序调整为:**先校验 `currency` 受支持,再解析并校验 `amount`**。

这是一个可观察的行为变化:如果调用方同时传入了不受支持的币种和格式错误的金额,以前返回"金额非法"、以后会返回"币种不受支持"。两种情况请求都会被拒绝,最终结果一致,只是错误信息的具体内容变了,已在 Risks 里记录。

### 符号处理不变
`amount` 字符串本身**永远是正数**(现有契约"Positive for both income and expense"不变),`income`/`expense` 的方向仍由 `type` 字段决定,写库前按 `type` 取正负、读出时取绝对值——这部分 `transactions.go` 里已有的逻辑(`signedAmount`/`abs64`)不需要改动,只是它们现在操作的是"字符串解析出的整数"而不是"直接从 wire 读到的整数"。

## Risks / Trade-offs

- **[Risk]** 实现时图省事,用固定两位小数格式化(如 `fmt.Sprintf("%.2f", ...)`),对 JPY/BHD 等非两位小数币种产生错误结果,重新引入 `2026-08-22-replace-ezbookkeeping-backend` 当年特意修掉的那个问题。 → **Mitigation**:所有格式化/解析必须经过 `currency` 包里统一的换算函数,不允许在 `transactions.go` 里另起格式化逻辑;单元测试必须覆盖至少一个 0 位小数币种(JPY)和一个 3 位小数币种(BHD),不能只测 CNY。
- **[Risk]** `transactions_test.go` 里大量既有用例用 `int64` 字面量表示 `amount`,机械改写成对应的十进制字符串时容易手滑(比如少写一个零)。 → **Mitigation**:改写时同时保留对存储层最小单位整数值的断言(通过 `store` 层直接查库核对),用两层断言互相验证,不能只断言 wire 格式的字符串本身。
- **[Risk]** 这是一次没有兼容垫片的破坏性变更,任何已经按旧契约(`int64` 最小单位)接入的调用方会直接调用失败。 → **Mitigation**:按 proposal.md 记录,当前没有真实账本数据、也没有已知的外部调用方依赖旧格式,接受直接切换,不做双格式兼容期。

## Migration Plan

- 无数据迁移——存储格式不变,只改 MCP 工具层的输入/输出编解码。
- 无兼容垫片、无灰度开关——项目一贯不引入迁移框架或多版本协议共存(参见 `openspec/project.md` 里"没有独立的迁移框架"的既定方向),这次维持同样的极简策略,直接切换。
- 回滚方式:还原这次改动的提交即可,不涉及任何数据状态需要撤销。
