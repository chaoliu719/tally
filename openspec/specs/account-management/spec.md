# account-management Specification

## Purpose
让唯一用户能够通过 MCP 工具查看、创建、更新、删除账户,为记录交易提供必要的账户主数据。隐藏、排序等能力留给后续 change。

## Requirements

### Requirement: 查看全部账户
用户 SHALL 能够通过 `list_accounts` 工具获取当前账本下的全部账户及其关键信息(名称、类型、币种、当前余额)。

#### Scenario: 账本为空
- **WHEN** 调用 `list_accounts` 时账本中还没有任何账户
- **THEN** 返回一个空列表,而不是错误

#### Scenario: 账本已有账户
- **WHEN** 调用 `list_accounts` 时账本中已存在账户
- **THEN** 返回全部账户,每个账户包含名称、类型、币种、当前余额

### Requirement: 创建新账户
用户 SHALL 能够通过 `manage_account` 工具,以 `operation="create"`,创建一个新账户,指定名称、类型、币种与初始余额。

#### Scenario: 提供有效信息创建账户
- **WHEN** 调用 `manage_account`(`operation="create"`)创建账户,提供的名称、类型、币种、初始余额均合法
- **THEN** 账户被创建,随后出现在 `list_accounts` 的结果中,余额等于指定的初始余额

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_account`(`operation="create"`)创建账户,但缺少名称、类型或币种等必填字段
- **THEN** 请求被拒绝,返回说明缺少哪个字段的错误,不创建任何账户

#### Scenario: 币种不受支持
- **WHEN** 调用 `manage_account`(`operation="create"`)创建账户,指定的币种代码不是系统支持的货币
- **THEN** 请求被拒绝,不创建账户

### Requirement: 更新账户信息
用户 SHALL 能够通过 `manage_account` 工具,以 `operation="update"` 并指定账户 `id`,更新一个已存在账户的 `name`、`type`、`comment`。这三个字段必须一并提供(完整替换),不支持只修改其中一个字段而保留其余字段原值。`currency` 与 `balance` 不能通过这个操作修改。

#### Scenario: 提供有效信息更新账户
- **WHEN** 调用 `manage_account`(`operation="update"`),指定一个已存在的账户 `id`,并提供合法的 `name`、`type`、`comment`
- **THEN** 账户的这三个字段被更新为新值,随后 `list_accounts` 反映新值,账户余额不受影响

#### Scenario: 目标账户不存在
- **WHEN** 调用 `manage_account`(`operation="update"`),指定的 `id` 不对应任何已存在的账户
- **THEN** 请求被拒绝,返回"未找到"错误,不修改任何账户

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_account`(`operation="update"`),但 `name`、`type`、`comment` 三者之一未提供
- **THEN** 请求被拒绝,返回说明缺少哪个字段的错误,不修改任何账户

#### Scenario: 尝试修改币种或余额
- **WHEN** 调用 `manage_account`(`operation="update"`)时提供了 `currency`,或提供了非零的 `balance`
- **THEN** 请求被拒绝,返回说明这两个字段不可通过更新修改的错误,不修改任何账户

### Requirement: 删除账户
用户 SHALL 能够通过 `manage_account` 工具,以 `operation="delete"` 并指定账户 `id`,删除一个账户。这是一个破坏性操作,遵循 `write-confirmation` 能力定义的 preview → apply 两步流程。一个账户只要被任意交易(包括 `adjustment` 类型的交易)引用,就不能被删除,且不提供强制覆盖的方式。

#### Scenario: 删除一个没有交易记录的账户
- **WHEN** 先以 `operation="delete"` 不带 `confirmation_token` 调用 `manage_account`(得到预览与 `confirmation_token`),再以同样的 `id` 携带该 `confirmation_token` 调用 `operation="delete"`,且这个账户在两次调用之间始终没有被任何交易引用
- **THEN** 账户被删除,不再出现在 `list_accounts` 的结果中

#### Scenario: 删除一个仍被交易引用的账户
- **WHEN** 以 `operation="delete"` 调用 `manage_account`,目标账户被至少一笔交易引用
- **THEN** 请求被拒绝,返回说明账户仍被引用、无法删除的错误,不删除该账户,也不签发 `confirmation_token`

#### Scenario: 目标账户不存在
- **WHEN** 调用 `manage_account`(`operation="delete"`),指定的 `id` 不对应任何已存在的账户
- **THEN** 请求被拒绝,返回"未找到"错误
