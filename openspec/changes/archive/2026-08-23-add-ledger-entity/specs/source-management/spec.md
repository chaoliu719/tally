## MODIFIED Requirements

### Requirement: 查看全部来源
用户 SHALL 能够通过 `list_sources` 工具,指定 `ledger_id`,获取该账本下的全部来源及其名称。

#### Scenario: 账本为空
- **WHEN** 调用 `list_sources` 指定一个已存在的账本,但账本中还没有任何来源
- **THEN** 返回一个空列表,而不是错误

#### Scenario: 账本已有来源
- **WHEN** 调用 `list_sources` 指定一个已存在的账本,账本中已存在来源
- **THEN** 返回该账本下的全部来源,每个来源包含 `id` 与名称,不包含其他账本的来源

#### Scenario: 指定的账本不存在
- **WHEN** 调用 `list_sources`,指定的 `ledger_id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回"未找到"错误

### Requirement: 创建新来源
用户 SHALL 能够通过 `manage_source` 工具,以 `operation="create"`,在指定的 `ledger_id` 下创建一个新来源,指定名称。

#### Scenario: 提供有效信息创建来源
- **WHEN** 调用 `manage_source`(`operation="create"`)创建来源,指定的 `ledger_id` 已存在,提供的名称合法
- **THEN** 来源被创建并归属该账本,随后出现在该账本 `list_sources` 的结果中

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_source`(`operation="create"`)创建来源,但缺少名称或 `ledger_id`
- **THEN** 请求被拒绝,返回说明缺少哪个字段的错误,不创建任何来源

#### Scenario: 指定的账本不存在
- **WHEN** 调用 `manage_source`(`operation="create"`),指定的 `ledger_id` 不对应任何已存在的账本
- **THEN** 请求被拒绝,返回说明账本不存在的错误,不创建任何来源

### Requirement: 更新来源信息
用户 SHALL 能够通过 `manage_source` 工具,以 `operation="update"` 并指定来源 `id` 与其所属 `ledger_id`,更新一个已存在来源的 `name`。

#### Scenario: 提供有效信息更新来源
- **WHEN** 调用 `manage_source`(`operation="update"`),指定一个已存在的来源 `id` 与其所属的 `ledger_id`,并提供合法的 `name`
- **THEN** 来源的名称被更新为新值,随后 `list_sources` 反映新值

#### Scenario: 目标来源不存在
- **WHEN** 调用 `manage_source`(`operation="update"`),指定的 `id` 不对应任何已存在的来源
- **THEN** 请求被拒绝,返回"未找到"错误,不修改任何来源

#### Scenario: ledger_id 与来源实际所属账本不一致
- **WHEN** 调用 `manage_source`(`operation="update"`),指定的来源 `id` 确实存在,但提供的 `ledger_id` 不是它实际所属的账本
- **THEN** 请求被拒绝,返回"未找到"错误,不修改任何来源

#### Scenario: 缺少必填字段
- **WHEN** 调用 `manage_source`(`operation="update"`),但未提供 `name` 或 `ledger_id`
- **THEN** 请求被拒绝,返回说明缺少哪个字段的错误,不修改任何来源

### Requirement: 删除来源
用户 SHALL 能够通过 `manage_source` 工具,以 `operation="delete"` 并指定来源 `id` 与其所属 `ledger_id`,删除一个来源。这是一个破坏性操作,遵循 `write-confirmation` 能力定义的 preview → apply 两步流程。一个来源只要被同一账本内任意交易引用,就不能被删除,且不提供强制覆盖的方式。

#### Scenario: 删除一个没有交易记录的来源
- **WHEN** 先以 `operation="delete"` 不带 `confirmation_token` 调用 `manage_source`(得到预览与 `confirmation_token`),再以同样的 `id` 与 `ledger_id` 携带该 `confirmation_token` 调用 `operation="delete"`,且这个来源在两次调用之间始终没有被任何交易引用
- **THEN** 来源被删除,不再出现在该账本 `list_sources` 的结果中

#### Scenario: 删除一个仍被交易引用的来源
- **WHEN** 以 `operation="delete"` 调用 `manage_source`,目标来源被同一账本内至少一笔交易引用
- **THEN** 请求被拒绝,返回说明来源仍被引用、无法删除的错误,不删除该来源,也不签发 `confirmation_token`

#### Scenario: 目标来源不存在
- **WHEN** 调用 `manage_source`(`operation="delete"`),指定的 `id` 不对应任何已存在的来源,或对应的来源不属于提供的 `ledger_id`
- **THEN** 请求被拒绝,返回"未找到"错误
