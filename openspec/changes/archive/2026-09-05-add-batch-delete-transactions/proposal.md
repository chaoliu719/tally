## Why

`delete_transaction` 每次只能删一笔，一次 preview→apply 只覆盖一笔交易。批量清理历史交易（比如整月的失效/重复记录）现在要花"每笔两次工具调用"的代价，而且如果分多轮、一笔一笔处理，交易还可能在 preview 和 apply 之间变得过期。批量版本把整批交易压缩成一次 preview 调用 + 一次 confirm 调用，同时仍然要求确认后才真正删除。

本次范围严格限定在"删除"上，不涉及批量导入/创建——那部分项目已经明确延后（见历史备注：批量导入方向已定但现在不做，且不要 propose，因为它面向的是多用户/外部账单解析场景，和"清理现有账本数据"是两个问题）。

## What Changes

- 新增 `batch_delete_transactions`：给定一个账本和一组交易 id，preview（不带 `confirmation_token`）返回每个 id 当前的交易信息（或"未找到"标记），外加一个覆盖整批的 confirmation token；confirm（带上该 token）一次性删除批次里所有仍然匹配的交易。
- 删除是逐笔尽力而为（best-effort）：某一笔过期、已被删除、或自 preview 后有变动，不会阻塞其余笔的删除。返回结果逐笔报告结果（`deleted`、`not_found`、`revision_changed`）。
- confirmation token 的 payload 扩展为携带一组 `(id, revision)`，而不是单个，复用 `mcp/internal/confirm` 里现有的无状态 HMAC 签名 token 机制。
- 设定固定的批量上限，超出直接拒绝整个调用，用来限制 token 体积和单次调用的数据库开销。
- 单笔的 `delete_transaction` 保持不变，继续可用于一次性删除。

## Capabilities

### New Capabilities
- `batch-transaction-deletion`：在一个账本内按 id 列表批量 preview→confirm 删除多笔交易，逐笔尽力而为地报告结果。

### Modified Capabilities
（无 —— `delete_transaction` 现有行为和契约不变；这是在它旁边新增一个工具）

## Impact

- `mcp/internal/tools/transactions.go`：新增 `batch_delete_transactions` 工具、输入输出类型和 handler；复用 `transactionDeletionRevision` 和现有单笔删除的数据库路径。
- `mcp/internal/confirm`：token payload 需要新增一种批量形态（一组 id+revision），与现有单资源 payload 并存；不能破坏 `delete_transaction` 现有的 token/行为。
- 无 schema/存储层改动 —— 每笔交易的删除语义与 `delete_transaction` 完全一致。
