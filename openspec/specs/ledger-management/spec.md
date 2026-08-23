# ledger-management Specification

## Purpose
账本是资金记录的顶层隔离容器,让唯一用户可以在同一套系统里维护多个互相隔离的账本,各自拥有独立的来源、分类与交易,统计口径互不影响。

## Requirements

### Requirement: 查看全部账本
用户 SHALL 能够通过 `list_ledgers` 工具获取系统中的全部账本及其名称、备注。

#### Scenario: 系统中还没有任何账本
- **WHEN** 调用 `list_ledgers` 时系统中还没有任何账本
- **THEN** 返回一个空列表,而不是错误

#### Scenario: 系统中已有账本
- **WHEN** 调用 `list_ledgers` 时系统中已存在账本
- **THEN** 返回全部账本,每个账本包含 `id`、名称与备注

### Requirement: 创建新账本
用户 SHALL 能够通过 `manage_ledger` 工具,以 `operation="create"`,创建一个新账本,指定名称与可选的备注。新账本创建时不带任何来源或分类——两者都需要之后单独创建。

#### Scenario: 提供有效信息创建账本
- **WHEN** 调用 `manage_ledger`(`operation="create"`)创建账本,提供的名称合法
- **THEN** 账本被创建,随后出现在 `list_ledgers` 的结果中,且其下没有任何来源或分类

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_ledger`(`operation="create"`)创建账本,但缺少名称
- **THEN** 请求被拒绝,返回说明缺少名称的错误,不创建任何账本

### Requirement: 更新账本信息
用户 SHALL 能够通过 `manage_ledger` 工具,以 `operation="update"` 并指定账本 `id`,更新一个已存在账本的 `name` 与/或 `comment`。

#### Scenario: 提供有效信息更新账本
- **WHEN** 调用 `manage_ledger`(`operation="update"`),指定一个已存在的账本 `id`,并提供合法的 `name`
- **THEN** 账本的信息被更新,随后 `list_ledgers` 反映新值

#### Scenario: 目标账本不存在
- **WHEN** 调用 `manage_ledger`(`operation="update"`),指定的 `id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回"未找到"错误,不修改任何账本

### Requirement: 删除账本
用户 SHALL 能够通过 `manage_ledger` 工具,以 `operation="delete"` 并指定账本 `id`,删除一个账本。这是一个破坏性操作,遵循 `write-confirmation` 能力定义的 preview → apply 两步流程。一个账本只要还存在任意来源、分类或交易,就不能被删除,且不提供强制覆盖或级联删除的方式。

#### Scenario: 删除一个完全为空的账本
- **WHEN** 先以 `operation="delete"` 不带 `confirmation_token` 调用 `manage_ledger`(得到预览与 `confirmation_token`),再以同样的 `id` 携带该 `confirmation_token` 调用 `operation="delete"`,且这个账本在两次调用之间始终没有任何来源、分类或交易
- **THEN** 账本被删除,不再出现在 `list_ledgers` 的结果中

#### Scenario: 删除一个仍有来源、分类或交易的账本
- **WHEN** 以 `operation="delete"` 调用 `manage_ledger`,目标账本下仍存在至少一个来源、一个分类,或一笔交易
- **THEN** 请求被拒绝,返回说明账本非空、无法删除的错误,不删除该账本,也不签发 `confirmation_token`

#### Scenario: 目标账本不存在
- **WHEN** 调用 `manage_ledger`(`operation="delete"`),指定的 `id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回"未找到"错误
