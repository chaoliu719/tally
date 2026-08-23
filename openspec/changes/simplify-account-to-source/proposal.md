## Why

"账户"目前是一个带币种、带余额的独立实体,配套还有专门修正余额的 `adjustment` 交易类型。但这个项目本来就不做复式记账、不做对账——余额本身也只是查询时对交易表 `SUM(amount)` 现算,从未落表。带着币种和余额去建模一个不需要余额语义的记账场景,增加了不必要的复杂度(创建账户要填初始余额、更新账户要挡 currency/balance 改动、有专门的 adjustment 交易类型和相关校验分支),却没有对应的价值——没有转账,没有对账单,余额从不被拿来做任何强校验。

把"账户"弱化成"来源"(source):一笔交易上"钱从哪来/去了哪"的标签,和"分类"同构、同样极简(只有 `id` + `name`),去掉 currency、balance、type、comment。这样模型更贴近实际用途,也顺带让"账户间转账"这个从未成立的需求彻底不需要存在——尚未开始实施的 `add-transaction-transfers` 提案随本次改动一并作废删除。

## What Changes

- **BREAKING**: 移除"账户"(account)概念,新增"来源"(source)概念取代它。MCP 工具 `list_accounts`/`manage_account` 替换为 `list_sources`/`manage_source`;`create_transaction`/`update_transaction`/`search_transactions` 中的 `account_id` 参数改名为 `source_id`。
- **BREAKING**: 来源的字段从 `{ name, type, currency, balance }` 简化为只有 `{ id, name }`——去掉 type、currency、balance、comment 四个字段。创建来源不再需要填初始余额;`manage_source` 的 update 操作只需要提供 `name`(不再有 currency/balance 不可改的特殊校验,因为这两个字段已经不存在)。
- **BREAKING**: 币种从来源移到交易自身。现状是交易的币种完全从所属账户的 `currency` 派生(`transactions` 表本身没有 currency 列,`create_transaction` 也不接受 currency 参数);来源去掉 currency 后这条派生链断裂,因此 `create_transaction`/`update_transaction` 新增一个必填的 `currency` 参数,币种码校验(是否受支持)也从"创建/更新账户"移到这两个交易工具上。
- **BREAKING**: 移除 `adjustment` 交易类型。交易类型只剩 `income`/`expense`。`create_transaction`/`update_transaction` 中所有与 `adjustment` 相关的校验分支和 scenario 一并移除;`get_financial_summary` 不再单独统计"余额调整总额"。
- `get_financial_summary` 原有的"按账户拆分收支小计"改名为"按来源拆分收支小计",维度语义不变(仍按币种分组、不做汇率换算),只是不再暗示"账户余额"。
- `manage_source` 的删除保护逻辑与 `manage_category` 完全同构:来源只要被任意交易引用就不能删除,不提供强制覆盖,走 `write-confirmation` 的 preview → apply 两步流程。
- 删除尚未提交、未开始实施的 `add-transaction-transfers` 改动目录——它建立在"账户间转账"这个前提上,该前提随本次改动不再成立。

## Capabilities

### New Capabilities
- `source-management`: 让唯一用户能够通过 `list_sources`/`manage_source` 查看、创建、更新、删除"来源"(交易的资金来源/去向标签),为记录交易提供必要的来源主数据。

### Modified Capabilities
- `account-management`: 全部账户相关能力(查看/创建/更新/删除账户)被移除,由 `source-management` 取代。
- `transaction-recording`: `create_transaction`/`update_transaction`/`search_transactions` 中的 `account_id` 改名为 `source_id`;`create_transaction`/`update_transaction` 新增必填的 `currency` 参数及"币种不受支持"校验(原本挂在账户创建/更新上的这条校验搬到这里);移除"记录一笔余额调整交易"这一整个 requirement 及相关 scenario;更新交易的字段校验规则相应调整(不再有 `adjustment` 分支);"清空账户下全部交易后可以删除该账户"的 scenario 改为对应"来源"。
- `financial-analytics`: "按账户拆分收支"改名为"按来源拆分收支";移除"单独统计余额调整总额"这一整个 requirement。

## Impact

- MCP 工具层:新增 `list_sources`/`manage_source`,移除 `list_accounts`/`manage_account`;`create_transaction`/`update_transaction`/`search_transactions`/`get_financial_summary` 的入参和返回字段相应调整。
- 存储层:`accounts` 表结构简化(去掉 `type`/`currency`/`comment` 列,或直接重命名为 `sources` 表,只剩 `id`/`name`);`transactions` 表的外键列 `account_id` 改名为 `source_id`,新增 `currency` 列;交易类型枚举去掉 `adjustment`。
- 现有 `openspec/specs/account-management/spec.md` 在归档后被清空/移除,新增 `openspec/specs/source-management/spec.md`。
- 未提交的 `openspec/changes/add-transaction-transfers/` 目录直接删除,不进入归档流程。
