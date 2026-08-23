## MODIFIED Requirements

### Requirement: 更新账本信息
用户 SHALL 能够通过 `manage_ledger` 工具,以 `operation="update"` 并指定账本 `id`,更新一个已存在账本的 `name` 与 `comment`。这两个字段必须一并提供(完整替换),不支持只修改其中一个字段而保留另一个字段原值。

#### Scenario: 提供有效信息更新账本
- **WHEN** 调用 `manage_ledger`(`operation="update"`),指定一个已存在的账本 `id`,并提供合法的 `name` 与 `comment`
- **THEN** 账本的 `name` 与 `comment` 都被更新为新值,随后 `list_ledgers` 反映新值

#### Scenario: 只提供 name,未提供 comment
- **WHEN** 调用 `manage_ledger`(`operation="update"`),指定一个已存在的账本 `id`,只提供了 `name`,未提供 `comment`
- **THEN** 请求被拒绝,返回说明缺少 `comment` 的错误,不修改任何账本

#### Scenario: 只提供 comment,未提供 name
- **WHEN** 调用 `manage_ledger`(`operation="update"`),指定一个已存在的账本 `id`,只提供了 `comment`,未提供 `name`
- **THEN** 请求被拒绝,返回说明缺少 `name` 的错误,不修改任何账本

#### Scenario: 目标账本不存在
- **WHEN** 调用 `manage_ledger`(`operation="update"`),指定的 `id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回"未找到"错误,不修改任何账本
