## Purpose

让唯一用户能够通过 MCP 工具查看现有账户、创建新账户,为记录交易提供必要的账户主数据。这次改动只覆盖查看与创建,更新、删除、隐藏、排序等能力留给后续 change。

## ADDED Requirements

### Requirement: 查看全部账户
用户 SHALL 能够通过 `list_accounts` 工具获取当前账本下的全部账户及其关键信息(名称、类型、币种、当前余额)。

#### Scenario: 账本为空
- **WHEN** 调用 `list_accounts` 时账本中还没有任何账户
- **THEN** 返回一个空列表,而不是错误

#### Scenario: 账本已有账户
- **WHEN** 调用 `list_accounts` 时账本中已存在账户
- **THEN** 返回全部账户,每个账户包含名称、类型、币种、当前余额

### Requirement: 创建新账户
用户 SHALL 能够通过 `manage_account` 工具创建一个新账户,指定名称、类型、币种与初始余额。

#### Scenario: 提供有效信息创建账户
- **WHEN** 调用 `manage_account` 创建账户,提供的名称、类型、币种、初始余额均合法
- **THEN** 账户被创建,随后出现在 `list_accounts` 的结果中,余额等于指定的初始余额

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_account` 创建账户,但缺少名称、类型或币种等必填字段
- **THEN** 请求被拒绝,返回说明缺少哪个字段的错误,不创建任何账户

#### Scenario: 币种不受支持
- **WHEN** 调用 `manage_account` 创建账户,指定的币种代码不是系统支持的货币
- **THEN** 请求被拒绝,不创建账户
