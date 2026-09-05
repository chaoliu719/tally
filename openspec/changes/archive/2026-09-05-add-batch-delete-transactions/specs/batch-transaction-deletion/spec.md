## Purpose

让调用方用一次 preview→confirm 往返，删除同一账本内的多笔交易，而不是把 `delete_transaction` 的两步流程对每笔交易重复一遍。

## ADDED Requirements

### Requirement: 预览一批删除
不带 `confirmation_token` 调用 `batch_delete_transactions` 时，SHALL 针对给定的 `ledger_id` 和一组交易 id，在该账本内逐个查找：找到的返回其当前交易信息，未找到的返回"未找到"标记，且不删除任何内容。返回结果 SHALL 包含一个覆盖整批预览内容的 confirmation token。

#### Scenario: 预览一批既有已存在也有不存在的 id
- **WHEN** 调用 `batch_delete_transactions`，传入 `ledger_id` 和 id 列表 `[10, 11, 999]`、不带 `confirmation_token`，且该账本内交易 10、11 存在而 999 不存在
- **THEN** 返回结果中交易 10、11 各自带上当前字段，id 999 报告为未找到，并附带一个 `confirmation_token` 及其过期时间

### Requirement: 确认时逐笔尽力而为地删除已预览的批次
带着此前 preview 得到的 `confirmation_token` 调用 `batch_delete_transactions` 时，SHALL 对该 token 覆盖的每个 `(id, revision)`：如果该交易在同一账本内仍然存在且自预览以来未变化，则删除；否则报告该项的结果，且不影响批次中其余项的处理。批次中某一项已被删除、在此期间被其他调用删除、或自预览后被修改，SHALL NOT 阻止同一批次内其余项被删除。

#### Scenario: 确认前有一项已被删除
- **WHEN** 对一个包含 3 笔预览过的交易 id 的批次调用确认，其中一笔在预览之后、确认之前已经被删除（例如被另一次调用删除）
- **THEN** 返回结果删除了另外 2 笔交易，并将那笔已经不存在的 id 报告为未找到，而不是拒绝整个确认调用

#### Scenario: 有一项自预览后发生了变化
- **WHEN** 对一个已预览的批次调用确认，其中一笔交易在预览之后、确认之前被修改过（例如通过 `update_transaction`）
- **THEN** 返回结果删除了批次中未变化的项，并将被修改过的那笔交易报告为 revision 已变化，既不删除它，也不影响批次中其余项的处理

#### Scenario: 用过期或被篡改的 token 确认
- **WHEN** 调用 `batch_delete_transactions` 时所带的 `confirmation_token` 已过期，或其签名校验不通过
- **THEN** 该调用被拒绝并返回错误，批次中没有任何一笔交易被删除

### Requirement: 批量大小有上限
`batch_delete_transactions` SHALL 在进行任何查找或删除之前，如果请求（预览）中的 id 数量超过固定的批量上限，就直接拒绝整个调用。

#### Scenario: 批次超过最大数量
- **WHEN** 以超过固定批量上限的 id 数量调用 `batch_delete_transactions` 进行预览
- **THEN** 该调用被拒绝并返回说明该上限的错误，且不发生任何查找或删除
