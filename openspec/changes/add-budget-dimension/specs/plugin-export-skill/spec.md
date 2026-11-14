# Spec Delta

## ADDED Requirements

### Requirement: 导出不携带预算
导出的 CSV SHALL 保持目标导入页认的固定列,不新增预算列,也不把预算名称写入分类列或其他任何列。预算信息在导出时丢失是预期行为,Agent SHALL 在交付时向用户说明这一点。导出 SHALL NOT 为了让预算信息得以保留而改写任何交易的分类或备注。

#### Scenario: 导出一段包含多个预算的交易
- **WHEN** 用户要求导出一段时间的交易,这些交易分属不同预算
- **THEN** 生成的 CSV 仍是目标导入页认的固定列,不含预算信息,agent 在交付时说明预算没有被导出

#### Scenario: 想用分类列保留预算
- **WHEN** agent 考虑把预算名称拼进分类列,以免导出后丢失预算信息
- **THEN** agent 不这样做,分类列仍只写一级分类名,预算信息留在账本内不进入导出文件

## MODIFIED Requirements

### Requirement: 导出是只读操作
导出 SHALL 只使用读类工具(`search_transactions`、`list_categories`、`list_sources`、`list_budgets`、`list_ledgers`、`open_transaction_timeline`)读取既有交易,SHALL NOT 新增、修改或删除 tally 中的任何交易、分类、来源或预算。

#### Scenario: 导出过程中发现分类需要调整
- **WHEN** agent 认为某笔交易的分类名不适配目标服务,导出时会更容易导入
- **THEN** agent 不修改该交易,按 skill 规定的映射原样导出,并在回复里说明差异
