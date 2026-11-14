# Spec Delta

## MODIFIED Requirements

### Requirement: 创建新账本
用户 SHALL 能够通过 `manage_ledger` 工具,以 `operation="create"`,创建一个新账本,指定名称与可选的备注。新账本创建时不带任何来源或分类——两者都需要之后单独创建;但 SHALL 同时建立一组种子预算,名称为 `固定`、`专项`、`突发`、`日常`,使新账本一经创建即可记录交易。

#### Scenario: 提供有效信息创建账本
- **WHEN** 调用 `manage_ledger`(`operation="create"`)创建账本,提供的名称合法
- **THEN** 账本被创建,随后出现在 `list_ledgers` 的结果中,其下没有任何来源或分类,但已有名为 `固定`、`专项`、`突发`、`日常` 的四个预算

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_ledger`(`operation="create"`)创建账本,但缺少名称
- **THEN** 请求被拒绝,返回说明缺少名称的错误,不创建任何账本,也不建立任何预算
