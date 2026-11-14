# Spec Delta

## MODIFIED Requirements

### Requirement: 合并通过逐笔重新分类加安全删除完成
Agent SHALL 通过 `batch_update_transactions` 把分类 A 下的交易改为分类 B,待分类 A 不再被任何交易引用后,再对分类 A 执行 `manage_category` 的 delete 操作(遵循 `write-confirmation` 的 preview → apply 两步确认)来完成合并。重新分类 SHALL 走批量更新的 preview → apply,不得再对每笔交易分别调用 `update_transaction`。

#### Scenario: 执行合并
- **WHEN** 用户确认合并分类 A 到分类 B
- **THEN** agent 用 `batch_update_transactions` 把分类 A 下的交易改为分类 B(先预览命中范围,确认后再应用),待分类 A 不再被任何交易引用后,对分类 A 调用 `manage_category`(`operation="delete"`)并完成其 preview → apply 两步确认

### Requirement: 交易量较大时提前告知成本
Agent SHALL 在待合并分类下的交易数量超过 `batch_update_transactions` 的单批上限、需要分多批才能完成重新分类时,在执行前告知用户需要分几批。

#### Scenario: 待合并分类交易数量较多
- **WHEN** 待合并分类下的交易数量超过 `batch_update_transactions` 的单批上限,需要分多批才能完成
- **THEN** agent 在执行前告知用户需要分成几批完成,由用户决定是否现在执行

### Requirement: 拆分同样遵循先呈现后执行
Agent SHALL 在拆分分类时,先创建新分类并展示将被移动的交易列表或数量,再通过 `batch_update_transactions` 把匹配的交易改为引用新分类。

#### Scenario: 用户同意拆分
- **WHEN** 用户同意从一个分类中拆出新的子分类
- **THEN** agent 先调用 `manage_category`(`operation="create"`)建立新分类,展示将被移动的交易列表或数量,再用 `batch_update_transactions` 把匹配的交易改为引用新分类


