# budget-management Specification

## Purpose
让唯一用户通过 MCP 工具查看、创建、更新、删除"预算"——一笔交易上"这笔钱算哪一类钱"的标签,与来源、分类同级,为记录交易提供必要的预算主数据。

## Requirements

### Requirement: 查看全部预算
用户 SHALL 能够通过 `list_budgets` 工具,指定 `ledger_id`,获取该账本下的全部预算及其名称。

#### Scenario: 账本已有预算
- **WHEN** 调用 `list_budgets` 指定一个已存在的账本,账本中已存在预算
- **THEN** 返回该账本下的全部预算,每个预算包含 `id` 与名称,不包含其他账本的预算

#### Scenario: 指定的账本不存在
- **WHEN** 调用 `list_budgets`,指定的 `ledger_id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回"未找到"错误

### Requirement: 创建新预算
用户 SHALL 能够通过 `manage_budget` 工具,以 `operation="create"`,在指定的 `ledger_id` 下创建一个新预算,指定名称。

#### Scenario: 提供有效信息创建预算
- **WHEN** 调用 `manage_budget`(`operation="create"`)创建预算,指定的 `ledger_id` 已存在,提供的名称合法
- **THEN** 预算被创建并归属该账本,随后出现在该账本 `list_budgets` 的结果中

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_budget`(`operation="create"`)创建预算,但缺少名称或 `ledger_id`
- **THEN** 请求被拒绝,返回说明缺少哪个字段的错误,不创建任何预算

#### Scenario: 指定的账本不存在
- **WHEN** 调用 `manage_budget`(`operation="create"`),指定的 `ledger_id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回说明账本不存在的错误,不创建任何预算

### Requirement: 更新预算信息
用户 SHALL 能够通过 `manage_budget` 工具,以 `operation="update"` 并指定预算 `id` 与其所属 `ledger_id`,更新一个已存在预算的 `name`。

#### Scenario: 提供有效信息更新预算
- **WHEN** 调用 `manage_budget`(`operation="update"`),指定一个已存在的预算 `id` 与其所属的 `ledger_id`,并提供合法的 `name`
- **THEN** 预算的名称被更新为新值,随后 `list_budgets` 反映新值

#### Scenario: 目标预算不存在
- **WHEN** 调用 `manage_budget`(`operation="update"`),指定的 `id` 不对应任何已存在的预算
- **THEN** 请求被拒绝,返回"未找到"错误,不修改任何预算

#### Scenario: ledger_id 与预算实际所属账本不一致
- **WHEN** 调用 `manage_budget`(`operation="update"`),指定的预算 `id` 确实存在,但提供的 `ledger_id` 不是它实际所属的账本
- **THEN** 请求被拒绝,返回"未找到"错误,不修改任何预算

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_budget`(`operation="update"`),但未提供 `name` 或 `ledger_id`
- **THEN** 请求被拒绝,返回说明缺少哪个字段的错误,不修改任何预算

### Requirement: 删除预算
用户 SHALL 能够通过 `manage_budget` 工具,以 `operation="delete"` 并指定预算 `id` 与其所属 `ledger_id`,删除一个预算。这是一个破坏性操作,遵循 `write-confirmation` 能力定义的 preview → apply 两步流程。一个预算只要被同一账本内任意交易引用,就不能被删除,且不提供强制覆盖的方式。

#### Scenario: 删除一个没有交易记录的预算
- **WHEN** 先以 `operation="delete"` 不带 `confirmation_token` 调用 `manage_budget`(得到预览与 `confirmation_token`),再以同样的 `id` 与 `ledger_id` 携带该 `confirmation_token` 调用 `operation="delete"`,且这个预算在两次调用之间始终没有被任何交易引用
- **THEN** 预算被删除,不再出现在该账本 `list_budgets` 的结果中

#### Scenario: 删除一个仍被交易引用的预算
- **WHEN** 以 `operation="delete"` 调用 `manage_budget`,目标预算被同一账本内至少一笔交易引用
- **THEN** 请求被拒绝,返回说明预算仍被引用、无法删除的错误,不删除该预算,也不签发 `confirmation_token`

#### Scenario: 目标预算不存在
- **WHEN** 调用 `manage_budget`(`operation="delete"`),指定的 `id` 不对应任何已存在的预算,或对应的预算不属于提供的 `ledger_id`
- **THEN** 请求被拒绝,返回"未找到"错误

### Requirement: 已有账本在升级时获得种子预算
系统升级引入预算维度时,每个已存在的账本 SHALL 获得一组种子预算,名称为 `固定`、`专项`、`突发`、`日常`。这组种子 SHALL 只建立一次:用户之后删除或改名其中任何一个,系统 SHALL NOT 在后续启动时重新建立。

#### Scenario: 升级一个已有交易的账本
- **WHEN** 一个在预算维度引入之前创建的账本,在系统升级后第一次被访问
- **THEN** 该账本下存在名为 `固定`、`专项`、`突发`、`日常` 的四个预算

#### Scenario: 种子预算被删除后不再重建
- **WHEN** 用户删除了某个账本下名为 `日常` 的种子预算,之后系统重新启动
- **THEN** 该账本下不再出现名为 `日常` 的预算,除非用户重新创建
