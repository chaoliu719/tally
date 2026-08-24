## Why

`create_transaction`/`update_transaction` 的 `amount` 入参,与 `get_transaction`/`search_transactions`/`update_transaction`/`delete_transaction` 返回的 `TransactionInfo.amount`,目前都是"该币种最小单位"的整数(人民币是分)。这个契约要求每一次调用方(通常是驱动 MCP 的 LLM agent)自己心算"元↔分"换算,而服务端对这次换算没有任何校验——`validateTransactionInput` 只检查 `amount > 0`,不检查量级是否合理。2026-08-24 的一次真实使用中,这个设计直接导致金额被错误展示为原值的 100 倍(¥50 显示成 ¥5,000)。排查确认这不是数据错误:数据库里存的值本身是对的,问题是"最小单位整数"这个契约把换算这一步变成了可选、可省略、且不会报错的动作。同样的失误如果发生在写入侧(把"50 元"直接当 `Amount: 50` 传给 `create_transaction`),会静默存成 ¥0.50,此后读取、搜索、汇总会"自洽地"复现这个错误值,没有任何机制能发现。

## What Changes

- `amount` 的线上表示从"最小单位整数"改为"十进制字符串,单位是该币种的主单位"。小数位数**因币种而异**,继续复用现有的、按 ISO 4217 感知精度的静态表(`currency.Decimals`),不是不管什么币种都固定按两位小数格式化——同一个原始值 `5000`(最小单位),CNY 下是 `"50.00"`(2 位小数),JPY 下是 `"5000"`(0 位小数),BHD 下是 `"5.000"`(3 位小数)。这延续的是 `2026-08-22-replace-ezbookkeeping-backend` 当年"改用币种感知小数位数"的既有决定,不是引入新原则;实现时若图省事写死两位小数格式化,会把当年改掉的那个 bug 重新引入,需要在 design.md 里作为明确约束点出来。这次的字符串化仿照本项目已有的 `id` 编码先例——`id` 早已是十进制字符串而非 `int64`,为的是避免 JSON number 精度丢失;这次同样用字符串,为的是让调用方不管读还是写,永远只处理"人类认知里的那个数",消灭"元/分"心算换算这一整类错误。`currency` 字段本身不受影响,继续独立存在于每笔交易上,多币种账本(同一账本内 CNY/USD/JPY 交易并存)的能力不变。
- 读写两端统一,不再有单位不对称:
  - **BREAKING**: `create_transaction`/`update_transaction` 的 `amount` 入参类型从 `int64`(最小单位)改为十进制字符串(主单位)。
  - **BREAKING**: `TransactionInfo.amount`(`create_transaction`/`get_transaction`/`search_transactions`/`update_transaction`/`delete_transaction` 响应共用同一结构)从 `int64` 改为十进制字符串。
- 换算逻辑收拢到服务端一处:复用已有的、按币种感知小数位数的静态表(`currency.Decimals`——目前已定义但从未被调用),做精确的字符串↔整数换算,不引入浮点运算。
- 新增对非法金额字符串的校验(如非数字、负数、小数位数超出该币种允许精度),拒绝时返回明确错误,不静默截断或四舍五入。
- 数据库内部存储(SQLite `amount INTEGER`,最小单位)与 SQL 层聚合(`SUM(amount)` 计算余额/汇总)不变——这纯粹是 MCP 工具边界的线上格式变更,不涉及存储结构或聚合逻辑。
- 不在本次范围内:`financial-analytics` 能力下的 `income`/`expense`/`net` 字段目前是同样的最小单位整数,存在相同的换算风险,但本次不改动。

## Capabilities

### New Capabilities
(无)

### Modified Capabilities
- `transaction-recording`: `create_transaction`、`get_transaction`、`search_transactions`、`update_transaction`、`delete_transaction` 中所有涉及 `amount` 字段的场景,输入/输出格式从"最小单位整数"改为"主单位十进制字符串";新增对非法金额字符串格式(非数字、负数、小数位数超过该币种允许精度)的拒绝场景。

## Impact

- 代码:`mcp/internal/tools/transactions.go`(`TransactionInfo`/`CreateTransactionInput`/`UpdateTransactionInput` 及 `toTransactionInfo`)、`mcp/internal/currency`(补一个字符串↔整数换算函数,复用现有 `Decimals` 表)。
- 已有测试:`transactions_test.go` 中依赖 `int64` amount 的用例需要同步改写。
- 已归档的历史设计决策(`2026-08-22-replace-ezbookkeeping-backend` 的"金额怎么存"章节)记录的是内部存储层的选择,本次不改动存储层,只改协议层,与该决策不冲突。
- 不影响:`financial-analytics` 工具的 `income`/`expense`/`net` 字段暂不改动,后续如需保持一致可另开 change。
- 当前无真实账本历史数据依赖旧格式,视为无迁移成本的破坏性变更。
